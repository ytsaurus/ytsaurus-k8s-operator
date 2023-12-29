package components

import (
	"context"
	"log"
	"path"

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

// server manages common resources of YTsaurus cluster server components.
type server interface {
	resources.Fetchable
	resources.Syncable
	podsManager
	needUpdate() bool
	needSync() bool
	buildStatefulSet() *appsv1.StatefulSet
	rebuildStatefulSet() *appsv1.StatefulSet
}

type serverImpl struct {
	image      string
	labeller   *labeller.Labeller
	configSpec *ytv1.ConfigurationSpec
	proxy      apiproxy.APIProxy
	cluster    ytsaurusResourceStateManager
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
	return newServerConfigured(
		l,
		&ytsaurus.GetResource().Spec.ConfigurationSpec,
		ytsaurus.APIProxy(),
		ytsaurus,
		instanceSpec,
		binaryPath, configFileName, statefulSetName, serviceName,
		generator,
	)
}

func newServerConfigured(
	l *labeller.Labeller,
	configSpec *ytv1.ConfigurationSpec,
	proxy apiproxy.APIProxy,
	cluster ytsaurusResourceStateManager,
	instanceSpec *ytv1.InstanceSpec,
	binaryPath, configFileName, statefulSetName, serviceName string,
	generator ytconfig.YsonGeneratorFunc,
) server {
	image := configSpec.CoreImage
	if instanceSpec.Image != nil {
		image = *instanceSpec.Image
	}
	var caBundle *resources.CABundle
	if caBundleSpec := configSpec.CABundle; caBundleSpec != nil {
		caBundle = resources.NewCABundle(caBundleSpec.Name, consts.CABundleVolumeName, consts.CABundleMountPoint)
	}

	var tlsSecret *resources.TLSSecret
	transportSpec := instanceSpec.NativeTransport
	if transportSpec == nil {
		//FIXME(khlebnikov): do not mount common bus secret into all servers
		transportSpec = configSpec.NativeTransport
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
		configSpec:   configSpec,
		proxy:        proxy,
		cluster:      cluster,
		instanceSpec: instanceSpec,
		binaryPath:   binaryPath,
		statefulSet: resources.NewStatefulSet(
			statefulSetName,
			l,
			proxy,
			configSpec.ExtraPodAnnotations,
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
			configSpec.ConfigOverrides,
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

func (s *serverImpl) needSync() bool {
	needReload, err := s.configHelper.NeedReload()
	if err != nil {
		needReload = false
	}
	return s.configHelper.NeedInit() ||
		(s.cluster.GetClusterState() == ytv1.ClusterStateUpdating && needReload) ||
		!s.exists() ||
		s.statefulSet.NeedSync(s.instanceSpec.InstanceCount)
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

	setHostnameAsFQDN := true
	statefulSet.Spec.Template.Spec = corev1.PodSpec{
		ImagePullSecrets:  s.configSpec.ImagePullSecrets,
		SetHostnameAsFQDN: &setHostnameAsFQDN,
		Containers: []corev1.Container{
			{
				Image:        s.image,
				Name:         consts.YTServerContainerName,
				Command:      []string{s.binaryPath, "--config", path.Join(consts.ConfigMountPoint, fileNames[0])},
				VolumeMounts: volumeMounts,
				Resources:    s.instanceSpec.Resources,
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
	if s.configSpec.HostNetwork {
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
