package components

import (
	"context"
	"log"
	"path"

	"k8s.io/apimachinery/pkg/util/intstr"
	ptr "k8s.io/utils/pointer"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

const (
	monitoringPortName     = "monitoring"
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
}

type serverImpl struct {
	image      string
	labeller   *labeller.Labeller
	proxy      apiproxy.APIProxy
	commonSpec ytv1.CommonSpec

	binaryPath string

	instanceSpec *ytv1.InstanceSpec

	statefulSet       *resources.StatefulSet
	headlessService   *resources.HeadlessService
	monitoringService *resources.MonitoringService
	caBundle          *resources.CABundle
	tlsSecret         *resources.TLSSecret
	configHelper      *ConfigHelper

	builtStatefulSet *appsv1.StatefulSet
}

func newServer(
	l *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	instanceSpec *ytv1.InstanceSpec,
	binaryPath, configFileName, statefulSetName, serviceName string,
	generator ytconfig.YsonGeneratorFunc,
) server {
	proxy := ytsaurus.APIProxy()
	commonSpec := ytsaurus.GetCommonSpec()
	return newServerConfigured(
		l,
		proxy,
		commonSpec,
		instanceSpec,
		binaryPath, configFileName, statefulSetName, serviceName,
		generator,
	)
}

func newServerConfigured(
	l *labeller.Labeller,
	proxy apiproxy.APIProxy,
	commonSpec ytv1.CommonSpec,
	instanceSpec *ytv1.InstanceSpec,
	binaryPath, configFileName, statefulSetName, serviceName string,
	generator ytconfig.YsonGeneratorFunc,
) server {
	image := commonSpec.CoreImage
	if instanceSpec.Image != nil {
		image = *instanceSpec.Image
	}

	if instanceSpec.MonitoringPort != nil {
		l.MonitoringPort = *instanceSpec.MonitoringPort
	}

	var caBundle *resources.CABundle
	if caBundleSpec := commonSpec.CABundle; caBundleSpec != nil {
		caBundle = resources.NewCABundle(caBundleSpec.Name, consts.CABundleVolumeName, consts.CABundleMountPoint)
	}

	var tlsSecret *resources.TLSSecret
	transportSpec := instanceSpec.NativeTransport
	if transportSpec == nil {
		//FIXME(khlebnikov): do not mount common bus secret into all servers
		transportSpec = commonSpec.NativeTransport
	}
	if transportSpec != nil && transportSpec.TLSSecret != nil {
		tlsSecret = resources.NewTLSSecret(
			transportSpec.TLSSecret.Name,
			consts.BusSecretVolumeName,
			consts.BusSecretMountPoint)
	}

	return &serverImpl{
		labeller:     l,
		image:        image,
		proxy:        proxy,
		commonSpec:   commonSpec,
		instanceSpec: instanceSpec,
		binaryPath:   binaryPath,
		statefulSet: resources.NewStatefulSet(
			statefulSetName,
			l,
			proxy,
			commonSpec,
		),
		headlessService: resources.NewHeadlessService(
			serviceName,
			l,
			proxy,
		),
		monitoringService: resources.NewMonitoringService(
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
	return s.statefulSet.OldObject().(*appsv1.StatefulSet).Spec.Template.Spec.Containers[0].Image == s.image
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
	return s.statefulSet.ArePodsReady(ctx, s.instanceSpec.MinReadyInstanceCount)
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

	setHostnameAsFQDN := true
	statefulSet.Spec.Template.Spec = corev1.PodSpec{
		RuntimeClassName:  s.instanceSpec.RuntimeClassName,
		ImagePullSecrets:  s.commonSpec.ImagePullSecrets,
		SetHostnameAsFQDN: &setHostnameAsFQDN,
		Containers: []corev1.Container{
			{
				Image:        s.image,
				Name:         consts.YTServerContainerName,
				Command:      []string{s.binaryPath, "--config", path.Join(consts.ConfigMountPoint, fileNames[0])},
				VolumeMounts: volumeMounts,
				Ports: []corev1.ContainerPort{
					{
						Name:          monitoringPortName,
						ContainerPort: s.labeller.MonitoringPort,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				Resources: s.instanceSpec.Resources,
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.FromString(monitoringPortName),
							Path: readinessProbeHTTPPath,
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
	}
	if s.commonSpec.HostNetwork {
		statefulSet.Spec.Template.Spec.HostNetwork = true
		statefulSet.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}

	if s.caBundle != nil {
		s.caBundle.AddVolume(&statefulSet.Spec.Template.Spec)
		for i := range statefulSet.Spec.Template.Spec.Containers {
			s.caBundle.AddVolumeMount(&statefulSet.Spec.Template.Spec.Containers[i])
		}
		for i := range statefulSet.Spec.Template.Spec.InitContainers {
			s.caBundle.AddVolumeMount(&statefulSet.Spec.Template.Spec.InitContainers[i])
		}
	}

	if s.tlsSecret != nil {
		s.tlsSecret.AddVolume(&statefulSet.Spec.Template.Spec)
		s.tlsSecret.AddVolumeMount(&statefulSet.Spec.Template.Spec.Containers[0])
	}

	s.builtStatefulSet = statefulSet
	return statefulSet
}

func (s *serverImpl) removePods(ctx context.Context) error {
	ss := s.rebuildStatefulSet()
	ss.Spec.Replicas = ptr.Int32(0)
	return s.Sync(ctx)
}
