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
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
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
	setUpdateStrategy(strategy appsv1.StatefulSetUpdateStrategyType)
	addCARootBundle(c *corev1.Container)
	addTlsSecretMount(c *corev1.Container)
	addMonitoringPort(port corev1.ServicePort)
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
	caRootBundle      *resources.CABundle
	caBundle          *resources.CABundle
	busServerSecret   *resources.TLSSecret
	busClientSecret   *resources.TLSSecret
	configs           *ConfigMapBuilder

	builtStatefulSet *appsv1.StatefulSet

	updateStrategy *appsv1.StatefulSetUpdateStrategyType

	componentContainerPorts []corev1.ContainerPort

	readinessProbePort     intstr.IntOrString
	readinessProbeHTTPPath string
	readinessByContainers  []string
}

func newServer(
	l *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	instanceSpec *ytv1.InstanceSpec,
	binaryPath, configFileName string,
	generator ConfigGeneratorFunc,
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
	generator ConfigGeneratorFunc,
	defaultMonitoringPort int32,
	optFuncs ...Option,
) server {
	image := commonSpec.CoreImage
	if instanceSpec.Image != nil {
		image = *instanceSpec.Image
	}

	var busServerSecret *resources.TLSSecret
	var busClientSecret *resources.TLSSecret
	transportSpec := instanceSpec.NativeTransport
	if transportSpec == nil {
		// FIXME(khlebnikov): do not mount common bus secret into all servers
		transportSpec = commonSpec.NativeTransport
	}
	if transportSpec != nil {
		if transportSpec.TLSSecret != nil {
			busServerSecret = resources.NewTLSSecret(
				transportSpec.TLSSecret.Name,
				consts.BusServerSecretVolumeName,
				consts.BusServerSecretMountPoint)
		}
		if transportSpec.TLSClientSecret != nil {
			busClientSecret = resources.NewTLSSecret(
				transportSpec.TLSClientSecret.Name,
				consts.BusClientSecretVolumeName,
				consts.BusClientSecretMountPoint)
		}
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

	configs := NewConfigMapBuilder(
		l,
		proxy,
		l.GetMainConfigMapName(),
		commonSpec.ConfigOverrides,
	)

	configs.AddGenerator(configFileName, ConfigFormatYson, generator)

	return &serverImpl{
		labeller:      l,
		image:         image,
		configs:       configs,
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
		caRootBundle:    resources.NewCARootBundle(commonSpec.CARootBundle),
		caBundle:        resources.NewCABundle(commonSpec.CABundle),
		busServerSecret: busServerSecret,
		busClientSecret: busClientSecret,

		componentContainerPorts: opts.containerPorts,

		readinessProbePort:     opts.readinessProbeEndpointPort,
		readinessProbeHTTPPath: opts.readinessProbeEndpointPath,
		readinessByContainers:  opts.readinessByContainers,
	}
}

func (s *serverImpl) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		s.statefulSet,
		s.configs,
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
	needReload, err := s.configs.NeedReload()
	if err != nil {
		needReload = false
	}
	return needReload
}

func (s *serverImpl) needBuild() bool {
	return s.configs.NeedInit() ||
		!s.exists() ||
		s.statefulSet.NeedSync(s.instanceSpec.InstanceCount)
}

func (s *serverImpl) needSync() bool {
	return s.configNeedsReload() || s.needBuild()
}

func (s *serverImpl) Sync(ctx context.Context) error {
	cm, err := s.configs.Build()
	if err != nil {
		return err
	}
	_ = s.headlessService.Build()
	_ = s.monitoringService.Build()
	_ = s.buildStatefulSet()

	if err := s.setConfigChecksum(cm); err != nil {
		return err
	}

	return resources.Sync(ctx,
		s.statefulSet,
		s.configs,
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

	needReload, err := s.configs.NeedReload()
	if err != nil {
		return false
	}
	return needReload
}

func (s *serverImpl) arePodsReady(ctx context.Context) bool {
	return s.statefulSet.ArePodsReady(ctx, int(s.instanceSpec.InstanceCount), s.instanceSpec.MinReadyInstanceCount, s.readinessByContainers)
}

func (s *serverImpl) arePodsUpdatedToNewRevision(ctx context.Context) bool {
	logger := ctrllog.FromContext(ctx)

	if !resources.Exists(s.statefulSet) {
		return false
	}

	// Fetch the latest StatefulSet status from Kubernetes to ensure we have up-to-date revision info
	if err := s.statefulSet.Fetch(ctx); err != nil {
		logger.Error(err, "Failed to fetch StatefulSet status", "component", s.labeller.GetFullComponentName())
		return false
	}

	sts := s.statefulSet.OldObject()

	if sts.Status.UpdateRevision == "" {
		logger.Info("StatefulSet has no update revision yet", "component", s.labeller.GetFullComponentName())
		return false
	}

	logger.Info("StatefulSet revision status",
		"component", s.labeller.GetFullComponentName(),
		"currentRevision", sts.Status.CurrentRevision,
		"updateRevision", sts.Status.UpdateRevision,
		"currentReplicas", sts.Status.CurrentReplicas,
		"updatedReplicas", sts.Status.UpdatedReplicas,
		"readyReplicas", sts.Status.ReadyReplicas,
		"replicas", sts.Status.Replicas)

	// In OnDelete mode, currentRevision doesn't automatically update to match updateRevision.
	// see https://github.com/kubernetes/kubernetes/issues/106055
	// to be 100% sure, will compare pods StatefulSetRevisionLabel with sts.Status.UpdateRevision
	isOnDelete := sts.Spec.UpdateStrategy.Type == appsv1.OnDeleteStatefulSetStrategyType
	// for race condition case when new revision is created but status not updated yet
	if sts.Generation != sts.Status.ObservedGeneration {
		logger.Info("StatefulSet status not observed yet",
			"component", s.labeller.GetFullComponentName(),
			"generation", sts.Generation,
			"observedGeneration", sts.Status.ObservedGeneration)
		return false
	}
	if isOnDelete {
		if sts.Status.UpdateRevision == sts.Status.CurrentRevision {
			logger.Info("StatefulSet update revision has not advanced yet",
				"component", s.labeller.GetFullComponentName(),
				"currentRevision", sts.Status.CurrentRevision,
				"updateRevision", sts.Status.UpdateRevision)
			return false
		}

		pods, err := s.statefulSet.ListPods(ctx)
		if err != nil {
			logger.Error(err, "Failed to list pods for StatefulSet", "component", s.labeller.GetFullComponentName())
			return false
		}

		if len(pods) != int(*sts.Spec.Replicas) {
			logger.Info("OnDelete update in progress: pod count mismatch",
				"component", s.labeller.GetFullComponentName(),
				"expectedReplicas", *sts.Spec.Replicas,
				"actualPods", len(pods))
			return false
		}

		for _, pod := range pods {
			if pod.DeletionTimestamp != nil {
				logger.Info("OnDelete update in progress: pod is terminating",
					"component", s.labeller.GetFullComponentName(),
					"pod", pod.Name)
				return false
			}

			podRevision := pod.Labels[appsv1.StatefulSetRevisionLabel]
			if podRevision != sts.Status.UpdateRevision {
				logger.Info("OnDelete update in progress: pod not updated",
					"component", s.labeller.GetFullComponentName(),
					"pod", pod.Name,
					"podRevision", podRevision,
					"updateRevision", sts.Status.UpdateRevision)
				return false
			}

			if pod.Status.Phase != corev1.PodRunning {
				logger.Info("OnDelete update in progress: pod not ready",
					"component", s.labeller.GetFullComponentName(),
					"pod", pod.Name)
				return false
			}
		}

		logger.Info("OnDelete update complete: all pods updated and ready",
			"component", s.labeller.GetFullComponentName(),
			"updateRevision", sts.Status.UpdateRevision,
			"totalReplicas", *sts.Spec.Replicas)
		return true
	} else if sts.Status.CurrentRevision == sts.Status.UpdateRevision {
		// For RollingUpdate mode, currentRevision will eventually match updateRevision
		logger.Info("Update complete: currentRevision matches updateRevision",
			"component", s.labeller.GetFullComponentName(),
			"revision", sts.Status.CurrentRevision)
		return true
	}

	return false
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

	if s.updateStrategy != nil {
		statefulSet.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
			Type: *s.updateStrategy,
		}
	}

	fileNames := s.configs.GetFileNames()
	if len(fileNames) != 1 {
		log.Panicf("expected exactly one config filename, found %v", len(fileNames))
	}

	configPostprocessingCommand := getConfigPostprocessingCommand(fileNames[0])
	defaultEnvVars := getDefaultEnv()

	var readinessProbeParams ytv1.HealthcheckProbeParams
	if s.instanceSpec.ReadinessProbeParams != nil {
		readinessProbeParams = *s.instanceSpec.ReadinessProbeParams
	}

	command := make([]string, 0, 3+len(s.instanceSpec.EntrypointWrapper))
	command = append(command, s.instanceSpec.EntrypointWrapper...)
	command = append(command, s.binaryPath, "--config", path.Join(consts.ConfigMountPoint, fileNames[0]))

	podSpec := &statefulSet.Spec.Template.Spec
	*podSpec = corev1.PodSpec{
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
				Env:          defaultEnvVars,
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
				Env:          defaultEnvVars,
				VolumeMounts: volumeMounts,
			},
			{
				Image:        s.image,
				Name:         consts.PostprocessConfigContainerName,
				Command:      []string{"bash", "-xc", configPostprocessingCommand},
				Env:          defaultEnvVars,
				VolumeMounts: volumeMounts,
			},
		},
		Volumes:      volumes,
		Affinity:     s.instanceSpec.Affinity,
		NodeSelector: s.instanceSpec.NodeSelector,
		Tolerations:  s.instanceSpec.Tolerations,
		DNSConfig:    s.instanceSpec.DNSConfig,
	}

	if ptr.Deref(s.instanceSpec.HostNetwork, s.commonSpec.HostNetwork) {
		podSpec.HostNetwork = true
		// https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy
		podSpec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}
	if s.instanceSpec.DNSPolicy != "" {
		podSpec.DNSPolicy = s.instanceSpec.DNSPolicy
	}

	s.caRootBundle.AddVolume(podSpec)
	s.caBundle.AddVolume(podSpec)
	s.busServerSecret.AddVolume(podSpec)
	s.busClientSecret.AddVolume(podSpec)

	// Add CA root bundle into all containers.
	for i := range podSpec.Containers {
		s.addCARootBundle(&podSpec.Containers[i])
	}
	for i := range podSpec.InitContainers {
		s.addCARootBundle(&podSpec.InitContainers[i])
	}

	serverContainer := &podSpec.Containers[0]

	// Native transport certificates are required only in server container.
	s.addTlsSecretMount(serverContainer)

	s.builtStatefulSet = statefulSet
	return statefulSet
}

func (s *serverImpl) setConfigChecksum(cm *corev1.ConfigMap) error {
	configChecksum := cm.Annotations[consts.ConfigChecksumAnnotationName]
	if configChecksum == "" {
		return fmt.Errorf("config checksum is not found for %v", cm.Name)
	}
	// Propagate config checksum from configmap to pod template to trigger restart when needed.
	metav1.SetMetaDataAnnotation(&s.builtStatefulSet.Spec.Template.ObjectMeta, consts.ConfigChecksumAnnotationName, configChecksum)
	return nil
}

func (s *serverImpl) removePods(ctx context.Context) error {
	ss := s.rebuildStatefulSet()
	ss.Spec.Replicas = ptr.To(int32(0))
	return s.Sync(ctx)
}

func (s *serverImpl) addCARootBundle(c *corev1.Container) {
	s.caRootBundle.AddVolumeMount(c)
	s.caRootBundle.AddContainerEnv(c)
}

func (s *serverImpl) addTlsSecretMount(c *corev1.Container) {
	s.caBundle.AddVolumeMount(c)
	s.busServerSecret.AddVolumeMount(c)
	s.busClientSecret.AddVolumeMount(c)
}

// setUpdateStrategy sets the desired StatefulSet update strategy
func (s *serverImpl) setUpdateStrategy(strategy appsv1.StatefulSetUpdateStrategyType) {
	s.updateStrategy = &strategy
	// Clear the built StatefulSet so it will be rebuilt with the new strategy
	s.builtStatefulSet = nil
}

func (s *serverImpl) addMonitoringPort(port corev1.ServicePort) {
	s.monitoringService.AddPort(port)
}
