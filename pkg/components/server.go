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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
}

// Server represents a typical YT cluster server component, like master or scheduler.
type server struct {
	labeller *labeller.Labeller
	apiProxy *apiproxy.APIProxy

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
	apiProxy *apiproxy.APIProxy,
	instanceSpec *ytv1.InstanceSpec,
	binaryPath, configFileName, statefulSetName, serviceName string,
	generator ytconfig.GeneratorFunc) Server {
	return &server{
		labeller:     labeller,
		apiProxy:     apiProxy,
		instanceSpec: instanceSpec,
		binaryPath:   binaryPath,
		statefulSet: resources.NewStatefulSet(
			statefulSetName,
			labeller,
			apiProxy),
		headlessService: resources.NewHeadlessService(
			serviceName,
			labeller,
			apiProxy),
		monitoringService: resources.NewMonitoringService(
			serviceName,
			labeller,
			apiProxy),
		configHelper: NewConfigHelper(
			labeller,
			apiProxy,
			labeller.GetMainConfigMapName(),
			configFileName,
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

func (s *server) NeedUpdate() bool {
	oldImage := s.statefulSet.OldObject().(*appsv1.StatefulSet).Spec.Template.Spec.Containers[0].Image
	newImage := s.apiProxy.Ytsaurus().Spec.CoreImage
	return oldImage != newImage
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
		ImagePullSecrets:  s.apiProxy.Ytsaurus().Spec.ImagePullSecrets,
		SetHostnameAsFQDN: &setHostnameAsFQDN,
		Containers: []corev1.Container{
			{
				Image:        s.apiProxy.Ytsaurus().Spec.CoreImage,
				Name:         consts.YTServerContainerName,
				Command:      []string{s.binaryPath, "--config", path.Join(consts.ConfigMountPoint, s.configHelper.GetFileName())},
				VolumeMounts: volumeMounts,
				Resources:    s.instanceSpec.Resources,
			},
		},
		InitContainers: []corev1.Container{
			{
				Image:        s.labeller.Ytsaurus.Spec.CoreImage,
				Name:         consts.PrepareLocationsContainerName,
				Command:      []string{"bash", "-c", locationCreationCommand},
				VolumeMounts: volumeMounts,
			},
		},
		Volumes: createVolumes(s.instanceSpec.Volumes, s.labeller.GetMainConfigMapName()),
	}

	affinity := s.instanceSpec.Affinity
	if s.instanceSpec.EnableAntiAffinity != nil && *s.instanceSpec.EnableAntiAffinity {
		if affinity == nil {
			affinity = &corev1.Affinity{}
		}

		affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: s.labeller.GetSelectorLabelMap(),
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		}
	}
	statefulSet.Spec.Template.Spec.Affinity = affinity

	s.builtStatefulSet = statefulSet
	return statefulSet
}
