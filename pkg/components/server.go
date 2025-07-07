package components

import (
	"context"
	"fmt"
	"log"
	"path"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

const (
	readinessProbeHTTPPath = "/orchid/service"
)

// server manages common resources of YTsaurus cluster server components.
type server interface {
	resources.Fetchable
	resources.Syncable
	podsManager
	needUpdate() bool
	configNeedsReload() bool
	needBuild() bool
	needSync() bool
	buildStatefulSet() *appsv1.StatefulSet
	rebuildStatefulSet() *appsv1.StatefulSet
	addCABundleMount(c *corev1.Container)
	addTlsSecretMount(c *corev1.Container)
}

type serverImpl struct {
	image         string
	sidecarImages map[string]string
	labeller      *labeller.Labeller
	proxy         apiproxy.APIProxy
	commonSpec    ytv1.CommonSpec

	binaryPath string

	instanceSpec *ytv1.InstanceSpec

	statefulSet       *resources.StatefulSet
	headlessService   *resources.HeadlessService
	monitoringService *resources.MonitoringService
	caBundle          *resources.CABundle
	tlsSecret         *resources.TLSSecret
	configHelper      *ConfigHelper

	cfgen *ytconfig.NodeGenerator

	builtStatefulSet *appsv1.StatefulSet

	componentContainerPorts []corev1.ContainerPort

	readinessProbePort     intstr.IntOrString
	readinessProbeHTTPPath string
}

func newServer(
	l *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	instanceSpec *ytv1.InstanceSpec,
	binaryPath, configFileName string,
	generator ytconfig.YsonGeneratorFunc,
	cfgen *ytconfig.NodeGenerator,
	defaultMonitoringPort int32,
	options ...Option,
) server {
	proxy := ytsaurus.APIProxy()
	commonSpec := ytsaurus.GetCommonSpec()
	return newServerConfigured(
		l,
		proxy,
		commonSpec,
		instanceSpec,
		binaryPath, configFileName,
		generator,
		cfgen,
		defaultMonitoringPort,
		options...,
	)
}

func newServerConfigured(
	l *labeller.Labeller,
	proxy apiproxy.APIProxy,
	commonSpec ytv1.CommonSpec,
	instanceSpec *ytv1.InstanceSpec,
	binaryPath, configFileName string,
	generator ytconfig.YsonGeneratorFunc,
	cfgen *ytconfig.NodeGenerator,
	defaultMonitoringPort int32,
	optFuncs ...Option,
) server {
	image := commonSpec.CoreImage
	if instanceSpec.Image != nil {
		image = *instanceSpec.Image
	}

	var caBundle *resources.CABundle
	if caBundleSpec := commonSpec.CABundle; caBundleSpec != nil {
		caBundle = resources.NewCABundle(caBundleSpec.Name, consts.CABundleVolumeName, consts.CABundleMountPoint)
	}

	var tlsSecret *resources.TLSSecret
	transportSpec := instanceSpec.NativeTransport
	if transportSpec == nil {
		// FIXME(khlebnikov): do not mount common bus secret into all servers
		transportSpec = commonSpec.NativeTransport
	}
	if transportSpec != nil && transportSpec.TLSSecret != nil {
		tlsSecret = resources.NewTLSSecret(
			transportSpec.TLSSecret.Name,
			consts.BusSecretVolumeName,
			consts.BusSecretMountPoint)
	}

	monitoringPort := ptr.Deref(instanceSpec.MonitoringPort, defaultMonitoringPort)
	opts := &options{
		containerPorts: []corev1.ContainerPort{
			{
				Name:          consts.YTMonitoringContainerPortName,
				ContainerPort: monitoringPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		readinessProbeEndpointPort: intstr.FromString(consts.YTMonitoringContainerPortName),
		readinessProbeEndpointPath: readinessProbeHTTPPath,
		sidecarImages:              map[string]string{},
	}

	for _, fn := range optFuncs {
		fn(opts)
	}

	if timbertruckImage := extractTimbertruckImageFromSpec(commonSpec, instanceSpec); timbertruckImage != "" {
		opts.sidecarImages[consts.TimbertruckContainerName] = timbertruckImage
	}

	return &serverImpl{
		labeller:      l,
		image:         image,
		sidecarImages: opts.sidecarImages,
		proxy:         proxy,
		commonSpec:    commonSpec,
		instanceSpec:  instanceSpec,
		binaryPath:    binaryPath,
		statefulSet: resources.NewStatefulSet(
			l.GetServerStatefulSetName(),
			l,
			proxy,
			commonSpec,
		),
		headlessService: resources.NewHeadlessService(
			l.GetHeadlessServiceName(),
			l,
			proxy,
		),
		monitoringService: resources.NewMonitoringService(
			monitoringPort,
			l,
			proxy,
		),
		caBundle:  caBundle,
		tlsSecret: tlsSecret,
		configHelper: NewConfigHelper(
			l,
			proxy,
			l.GetMainConfigMapName(),
			commonSpec.ConfigOverrides,
			map[string]ytconfig.GeneratorDescriptor{
				configFileName: {
					F:   generator,
					Fmt: ytconfig.ConfigFormatYson,
				},
			}),

		cfgen: cfgen,

		componentContainerPorts: opts.containerPorts,

		readinessProbePort:     opts.readinessProbeEndpointPort,
		readinessProbeHTTPPath: opts.readinessProbeEndpointPath,
	}
}

func (s *serverImpl) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		s.statefulSet,
		s.configHelper,
		s.headlessService,
		s.monitoringService,
	)
}

func (s *serverImpl) exists() bool {
	return resources.Exists(s.statefulSet) &&
		resources.Exists(s.headlessService) &&
		resources.Exists(s.monitoringService)
}

func (s *serverImpl) configNeedsReload() bool {
	needReload, err := s.configHelper.NeedReload()
	if err != nil {
		needReload = false
	}
	return needReload
}

func (s *serverImpl) needBuild() bool {
	return s.configHelper.NeedInit() ||
		!s.exists() ||
		s.statefulSet.NeedSync(s.instanceSpec.InstanceCount)
}

func (s *serverImpl) needSync() bool {
	return s.configNeedsReload() || s.needBuild()
}

func (s *serverImpl) Sync(ctx context.Context) error {
	_ = s.configHelper.Build()
	_ = s.headlessService.Build()
	_ = s.monitoringService.Build()
	_ = s.buildStatefulSet()

	return resources.Sync(ctx,
		s.statefulSet,
		s.configHelper,
		s.headlessService,
		s.monitoringService,
	)
}

func (s *serverImpl) arePodsRemoved(ctx context.Context) bool {
	if !resources.Exists(s.statefulSet) {
		return true
	}

	return s.statefulSet.ArePodsRemoved(ctx)
}

func (s *serverImpl) podsImageCorrespondsToSpec() bool {
	found := 0
	for _, container := range s.statefulSet.OldObject().Spec.Template.Spec.Containers {
		switch container.Name {
		case consts.YTServerContainerName:
			if container.Image != s.image {
				return false
			}
		case consts.HydraPersistenceUploaderContainerName:
			found++
			requiredImage, ok := s.sidecarImages[consts.HydraPersistenceUploaderContainerName]
			if !ok || requiredImage != container.Image {
				return false
			}
		case consts.TimbertruckContainerName:
			found++
			requiredImage, ok := s.sidecarImages[consts.TimbertruckContainerName]
			if !ok || requiredImage != container.Image {
				return false
			}
		}
	}
	return found == len(s.sidecarImages)
}

func (s *serverImpl) needUpdate() bool {
	if !s.exists() {
		return false
	}

	if !s.podsImageCorrespondsToSpec() {
		return true
	}

	needReload, err := s.configHelper.NeedReload()
	if err != nil {
		return false
	}
	return needReload
}

func (s *serverImpl) arePodsReady(ctx context.Context) bool {
	return s.statefulSet.ArePodsReady(ctx, int(s.instanceSpec.InstanceCount), s.instanceSpec.MinReadyInstanceCount)
}

func (s *serverImpl) buildStatefulSet() *appsv1.StatefulSet {
	if s.builtStatefulSet != nil {
		return s.builtStatefulSet
	}

	return s.rebuildStatefulSet()
}

func (s *serverImpl) rebuildStatefulSet() *appsv1.StatefulSet {
	locationCreationCommand := getLocationInitCommand(s.instanceSpec.Locations)

	volumes := createServerVolumes(s.instanceSpec.Volumes, s.labeller.GetMainConfigMapName())
	volumeMounts := createVolumeMounts(s.instanceSpec.VolumeMounts)

	statefulSet := s.statefulSet.Build()

	for key, value := range s.instanceSpec.PodLabels {
		metav1.SetMetaDataLabel(&statefulSet.Spec.Template.ObjectMeta, key, value)
	}

	for key, value := range s.instanceSpec.PodAnnotations {
		metav1.SetMetaDataAnnotation(&statefulSet.Spec.Template.ObjectMeta, key, value)
	}

	statefulSet.Spec.Replicas = &s.instanceSpec.InstanceCount
	statefulSet.Spec.ServiceName = s.headlessService.Name()
	statefulSet.Spec.VolumeClaimTemplates = createVolumeClaims(s.instanceSpec.VolumeClaimTemplates)

	fileNames := s.configHelper.GetFileNames()
	if len(fileNames) != 1 {
		log.Panicf("expected exactly one config filename, found %v", len(fileNames))
	}

	configPostprocessingCommand := getConfigPostprocessingCommand(fileNames[0])
	configPostprocessEnv := getConfigPostprocessEnv()

	var readinessProbeParams ytv1.HealthcheckProbeParams
	if s.instanceSpec.ReadinessProbeParams != nil {
		readinessProbeParams = *s.instanceSpec.ReadinessProbeParams
	}

	command := make([]string, 0, 3+len(s.instanceSpec.EntrypointWrapper))
	command = append(command, s.instanceSpec.EntrypointWrapper...)
	command = append(command, s.binaryPath, "--config", path.Join(consts.ConfigMountPoint, fileNames[0]))

	statefulSet.Spec.Template.Spec = corev1.PodSpec{
		RuntimeClassName:   s.instanceSpec.RuntimeClassName,
		ImagePullSecrets:   s.commonSpec.ImagePullSecrets,
		SetHostnameAsFQDN:  s.instanceSpec.SetHostnameAsFQDN,
		EnableServiceLinks: ptr.To(false),

		TerminationGracePeriodSeconds: s.instanceSpec.TerminationGracePeriodSeconds,

		Containers: []corev1.Container{
			{
				Image:        s.image,
				Name:         consts.YTServerContainerName,
				Command:      command,
				VolumeMounts: volumeMounts,
				Ports:        s.componentContainerPorts,
				Resources:    *s.instanceSpec.Resources.DeepCopy(),
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: s.readinessProbePort,
							Path: s.readinessProbeHTTPPath,
						},
					},
					InitialDelaySeconds: readinessProbeParams.InitialDelaySeconds,
					TimeoutSeconds:      readinessProbeParams.TimeoutSeconds,
					PeriodSeconds:       readinessProbeParams.PeriodSeconds,
					SuccessThreshold:    readinessProbeParams.SuccessThreshold,
					FailureThreshold:    readinessProbeParams.FailureThreshold,
				},
			},
		},
		InitContainers: []corev1.Container{
			{
				Image:        s.image,
				Name:         consts.PrepareLocationsContainerName,
				Command:      []string{"bash", "-xc", locationCreationCommand},
				VolumeMounts: volumeMounts,
			},
			{
				Image:        s.image,
				Name:         consts.PostprocessConfigContainerName,
				Command:      []string{"bash", "-xc", configPostprocessingCommand},
				Env:          configPostprocessEnv,
				VolumeMounts: volumeMounts,
			},
		},
		Volumes:      volumes,
		Affinity:     s.instanceSpec.Affinity,
		NodeSelector: s.instanceSpec.NodeSelector,
		Tolerations:  s.instanceSpec.Tolerations,
		DNSConfig:    s.instanceSpec.DNSConfig,
	}

	var stsDNSPolicy corev1.DNSPolicy
	if ptr.Deref(s.instanceSpec.HostNetwork, s.commonSpec.HostNetwork) {
		statefulSet.Spec.Template.Spec.HostNetwork = true
		// https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy
		stsDNSPolicy = corev1.DNSClusterFirstWithHostNet
	}
	if s.instanceSpec.DNSPolicy != "" {
		stsDNSPolicy = s.instanceSpec.DNSPolicy
	}
	statefulSet.Spec.Template.Spec.DNSPolicy = stsDNSPolicy

	s.patchWithTimbertruck(statefulSet, s.instanceSpec.StructuredLoggers)

	if s.caBundle != nil {
		s.caBundle.AddVolume(&statefulSet.Spec.Template.Spec)
		for i := range statefulSet.Spec.Template.Spec.Containers {
			s.addCABundleMount(&statefulSet.Spec.Template.Spec.Containers[i])
		}
		for i := range statefulSet.Spec.Template.Spec.InitContainers {
			s.addCABundleMount(&statefulSet.Spec.Template.Spec.InitContainers[i])
		}
	}

	if s.tlsSecret != nil {
		s.tlsSecret.AddVolume(&statefulSet.Spec.Template.Spec)
		s.addTlsSecretMount(&statefulSet.Spec.Template.Spec.Containers[0])
	}

	s.builtStatefulSet = statefulSet
	return statefulSet
}

func extractTimbertruckImageFromSpec(commonSpec ytv1.CommonSpec, instanceSpec *ytv1.InstanceSpec) string {
	// If none of the loggers are going to deliver logs to cypress, then don't return the image.
	// This is important because otherwise the StatefulSet will wait for a timbertruck to be created,
	// which will not happen without at least one DeliveryToCypress.
	image := extractBuiltinSidecarImageFromSpec(commonSpec, *instanceSpec)
	for _, structuredLogger := range instanceSpec.StructuredLoggers {
		if structuredLogger.CypressDelivery != nil && structuredLogger.CypressDelivery.Enabled {
			return image
		}
	}
	return ""
}

func (s *serverImpl) patchWithTimbertruck(statefulSet *appsv1.StatefulSet, structuredLoggers []ytv1.StructuredLoggerSpec) {
	image := extractTimbertruckImageFromSpec(s.commonSpec, s.instanceSpec)
	if image == "" {
		return
	}

	workDir := ""
	for _, locations := range s.instanceSpec.Locations {
		if locations.LocationType == ytv1.LocationTypeLogs {
			workDir = locations.Path
			// workDir = path.Join(locations.Path, consts.TimbertruckWorkDirName)
			break
		}
	}
	if workDir == "" {
		return
	}
	timbertruckConfig := ytconfig.NewTimbertruckConfig(
		structuredLoggers,
		path.Join(workDir, consts.TimbertruckWorkDirName),
		consts.GetServiceKebabCase(s.labeller.ComponentType),
		workDir,
		s.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole),
	)

	if timbertruckConfig == nil {
		return
	}

	timbertruckInitScript, err := getTimbertruckInitScript(timbertruckConfig)
	if err != nil {
		return
	}

	statefulSet.Spec.Template.Spec.Containers = append(statefulSet.Spec.Template.Spec.Containers, corev1.Container{
		Name:    consts.TimbertruckContainerName,
		Image:   image,
		Command: []string{"/bin/bash", "-c", timbertruckInitScript},
		Env: []corev1.EnvVar{
			{
				Name: consts.TokenSecretKey,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: fmt.Sprintf("%s-secret", consts.TimbertruckUserName),
						},
						Key: consts.TokenSecretKey,
					},
				},
			},
			{
				Name:  "YT_PROXY",
				Value: s.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: path.Base(workDir), MountPath: workDir, ReadOnly: false},
		},
		ImagePullPolicy: corev1.PullAlways,
	})
}

func (s *serverImpl) removePods(ctx context.Context) error {
	ss := s.rebuildStatefulSet()
	ss.Spec.Replicas = ptr.To(int32(0))
	return s.Sync(ctx)
}

func (s *serverImpl) addCABundleMount(c *corev1.Container) {
	if s.caBundle != nil {
		s.caBundle.AddVolumeMount(c)
	}
}

func (s *serverImpl) addTlsSecretMount(c *corev1.Container) {
	if s.tlsSecret != nil {
		s.tlsSecret.AddVolumeMount(c)
	}
}
