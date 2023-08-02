package components

import (
	"context"
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

type Server interface {
	Fetch(ctx context.Context) error
	IsInSync() bool
	ArePodsRemoved() bool
	ArePodsReady(ctx context.Context) bool
	Sync(ctx context.Context) error
	BuildStatefulSet() *appsv1.StatefulSet
	RebuildStatefulSet() *appsv1.StatefulSet
	NeedUpdate() bool
	isImageEqualTo(expectedImage string) bool
}

// Server represents a typical YT cluster server component, like master or scheduler.
type server struct {
	labeller *labeller.Labeller
	ytsaurus *apiproxy.Ytsaurus

	binaryPath string

	instanceSpec *ytv1.InstanceSpec

	statefulSet       *resources.StatefulSet
	headlessService   *resources.HeadlessService
	monitoringService *resources.MonitoringService
	builtStatefulSet  *appsv1.StatefulSet

	configHelper *ConfigHelper
}

func NewServer(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	instanceSpec *ytv1.InstanceSpec,
	binaryPath, configFileName, statefulSetName, serviceName string,
	generator ytconfig.GeneratorFunc) Server {
	return &server{
		labeller:     labeller,
		ytsaurus:     ytsaurus,
		instanceSpec: instanceSpec,
		binaryPath:   binaryPath,
		statefulSet: resources.NewStatefulSet(
			statefulSetName,
			labeller,
			ytsaurus),
		headlessService: resources.NewHeadlessService(
			serviceName,
			labeller,
			ytsaurus.APIProxy()),
		monitoringService: resources.NewMonitoringService(
			labeller,
			ytsaurus.APIProxy()),
		configHelper: NewConfigHelper(
			labeller,
			ytsaurus.APIProxy(),
			labeller.GetMainConfigMapName(),
			configFileName,
			ytsaurus.GetResource().Spec.ConfigOverrides,
			generator),
	}
}

func (s *server) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		s.statefulSet,
		s.configHelper,
		s.headlessService,
		s.monitoringService,
	})
}

func (s *server) IsInSync() bool {
	if s.configHelper.NeedSync() || !resources.Exists(s.statefulSet) || !resources.Exists(s.headlessService) || !resources.Exists(s.monitoringService) {
		return false
	}

	return s.statefulSet.NeedSync(s.instanceSpec.InstanceCount)
}

func (s *server) ArePodsRemoved() bool {
	if s.configHelper.NeedSync() || !resources.Exists(s.statefulSet) || !resources.Exists(s.headlessService) {
		return false
	}

	return s.statefulSet.NeedSync(0)
}

func (s *server) isImageEqualTo(expectedImage string) bool {
	return s.statefulSet.OldObject().(*appsv1.StatefulSet).Spec.Template.Spec.Containers[0].Image == expectedImage
}

func (s *server) NeedUpdate() bool {
	return !s.isImageEqualTo(s.ytsaurus.GetResource().Spec.CoreImage)
}

func (s *server) ArePodsReady(ctx context.Context) bool {
	return s.statefulSet.CheckPodsReady(ctx)
}

func (s *server) Sync(ctx context.Context) (err error) {
	_ = s.configHelper.Build()
	_ = s.headlessService.Build()
	_ = s.monitoringService.Build()
	_ = s.BuildStatefulSet()

	return resources.Sync(ctx, []resources.Syncable{
		s.statefulSet,
		s.configHelper,
		s.headlessService,
		s.monitoringService,
	})
}

func (s *server) BuildStatefulSet() *appsv1.StatefulSet {
	if s.builtStatefulSet != nil {
		return s.builtStatefulSet
	}

	return s.RebuildStatefulSet()
}

func (s *server) RebuildStatefulSet() *appsv1.StatefulSet {
	locationCreationCommand := getLocationInitCommand(s.instanceSpec.Locations)
	volumeMounts := createVolumeMounts(s.instanceSpec.VolumeMounts)

	statefulSet := s.statefulSet.Build()
	statefulSet.Spec.Replicas = &s.instanceSpec.InstanceCount
	statefulSet.Spec.ServiceName = s.headlessService.Name()
	statefulSet.Spec.VolumeClaimTemplates = createVolumeClaims(s.instanceSpec.VolumeClaimTemplates)

	setHostnameAsFQDN := true
	statefulSet.Spec.Template.Spec = corev1.PodSpec{
		ImagePullSecrets:  s.ytsaurus.GetResource().Spec.ImagePullSecrets,
		SetHostnameAsFQDN: &setHostnameAsFQDN,
		Containers: []corev1.Container{
			{
				Image:        s.ytsaurus.GetResource().Spec.CoreImage,
				Name:         consts.YTServerContainerName,
				Command:      []string{s.binaryPath, "--config", path.Join(consts.ConfigMountPoint, s.configHelper.GetFileName())},
				VolumeMounts: volumeMounts,
				Resources:    s.instanceSpec.Resources,
			},
		},
		InitContainers: []corev1.Container{
			{
				Image:        s.ytsaurus.GetResource().Spec.CoreImage,
				Name:         consts.PrepareLocationsContainerName,
				Command:      []string{"bash", "-c", locationCreationCommand},
				VolumeMounts: volumeMounts,
			},
		},
		Volumes:  createVolumes(s.instanceSpec.Volumes, s.labeller.GetMainConfigMapName()),
		Affinity: s.instanceSpec.Affinity,
	}
	s.builtStatefulSet = statefulSet
	return statefulSet
}
