package components

import (
	"context"
	ptr "k8s.io/utils/pointer"
	"log"
	"path"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	image    string
	labeller *labeller.Labeller
	ytsaurus *apiproxy.Ytsaurus

	binaryPath string

	instanceSpec *ytv1.InstanceSpec

	statefulSet       *resources.StatefulSet
	headlessService   *resources.HeadlessService
	monitoringService *resources.MonitoringService
	configHelper      *ConfigHelper

	builtStatefulSet *appsv1.StatefulSet
}

func newServer(
	l *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	instanceSpec *ytv1.InstanceSpec,
	binaryPath, configFileName, statefulSetName, serviceName string,
	generator ytconfig.YsonGeneratorFunc) server {
	image := ytsaurus.GetResource().Spec.CoreImage
	if instanceSpec.Image != nil {
		image = *instanceSpec.Image
	}
	return &serverImpl{
		labeller:     l,
		image:        image,
		ytsaurus:     ytsaurus,
		instanceSpec: instanceSpec,
		binaryPath:   binaryPath,
		statefulSet: resources.NewStatefulSet(
			statefulSetName,
			l,
			ytsaurus),
		headlessService: resources.NewHeadlessService(
			serviceName,
			l,
			ytsaurus.APIProxy()),
		monitoringService: resources.NewMonitoringService(
			l,
			ytsaurus.APIProxy()),
		configHelper: NewConfigHelper(
			l,
			ytsaurus.APIProxy(),
			l.GetMainConfigMapName(),
			ytsaurus.GetResource().Spec.ConfigOverrides,
			map[string]ytconfig.GeneratorDescriptor{
				configFileName: {
					F:   generator,
					Fmt: ytconfig.ConfigFormatYson,
				},
			}),
	}
}

func (s *serverImpl) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		s.statefulSet,
		s.configHelper,
		s.headlessService,
		s.monitoringService,
	})
}

func (s *serverImpl) exists() bool {
	return resources.Exists(s.statefulSet) &&
		resources.Exists(s.headlessService) &&
		resources.Exists(s.monitoringService)
}

func (s *serverImpl) needSync() bool {
	return s.configHelper.NeedSync() ||
		!s.exists() ||
		s.statefulSet.NeedSync(s.instanceSpec.InstanceCount)
}

func (s *serverImpl) Sync(ctx context.Context) error {
	_ = s.configHelper.Build()
	_ = s.headlessService.Build()
	_ = s.monitoringService.Build()
	_ = s.buildStatefulSet()

	return resources.Sync(ctx, []resources.Syncable{
		s.statefulSet,
		s.configHelper,
		s.headlessService,
		s.monitoringService,
	})
}

func (s *serverImpl) arePodsRemoved() bool {
	if s.configHelper.NeedSync() || !resources.Exists(s.statefulSet) || !resources.Exists(s.headlessService) {
		return false
	}

	return !s.statefulSet.NeedSync(0)
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

	volumes := createVolumes(s.instanceSpec.Volumes, s.labeller.GetMainConfigMapName())
	volumeMounts := createVolumeMounts(s.instanceSpec.VolumeMounts)

	statefulSet := s.statefulSet.Build()
	statefulSet.Spec.Replicas = &s.instanceSpec.InstanceCount
	statefulSet.Spec.ServiceName = s.headlessService.Name()
	statefulSet.Spec.VolumeClaimTemplates = createVolumeClaims(s.instanceSpec.VolumeClaimTemplates)

	fileNames := s.configHelper.GetFileNames()
	if len(fileNames) != 1 {
		log.Panicf("expected exactly one config filename, found %v", len(fileNames))
	}

	setHostnameAsFQDN := true
	statefulSet.Spec.Template.Spec = corev1.PodSpec{
		ImagePullSecrets:  s.ytsaurus.GetResource().Spec.ImagePullSecrets,
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
				Command:      []string{"bash", "-c", locationCreationCommand},
				VolumeMounts: volumeMounts,
			},
		},
		Volumes:      volumes,
		Affinity:     s.instanceSpec.Affinity,
		NodeSelector: s.instanceSpec.NodeSelector,
		Tolerations:  s.instanceSpec.Tolerations,
	}
	if s.ytsaurus.GetResource().Spec.HostNetwork {
		statefulSet.Spec.Template.Spec.HostNetwork = true
		statefulSet.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}

	s.builtStatefulSet = statefulSet
	return statefulSet
}

func (s *serverImpl) removePods(ctx context.Context) error {
	ss := s.rebuildStatefulSet()
	ss.Spec.Replicas = ptr.Int32(0)
	return s.Sync(ctx)
}
