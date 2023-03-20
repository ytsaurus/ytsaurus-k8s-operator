package components

import (
	"context"
	ytv1 "github.com/YTsaurus/yt-k8s-operator/api/v1"
	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/consts"
	"github.com/YTsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/YTsaurus/yt-k8s-operator/pkg/resources"
	"github.com/YTsaurus/yt-k8s-operator/pkg/ytconfig"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path"
)

// Server represents a typical YT cluster server component, like master or scheduler.
type Server struct {
	labeller *labeller.Labeller
	apiProxy *apiproxy.ApiProxy

	binaryPath         string
	enableAntiAffinity bool

	instanceSpec *ytv1.InstanceSpec

	statefulSet      *resources.StatefulSet
	headlessService  *resources.HeadlessService
	builtStatefulSet *appsv1.StatefulSet

	configHelper *ConfigHelper
}

func NewServer(
	labeller *labeller.Labeller,
	apiProxy *apiproxy.ApiProxy,
	instanceSpec *ytv1.InstanceSpec,
	binaryPath, configFileName, statefulSetName, serviceName string,
	enableAntiAffinity bool,
	generator ytconfig.GeneratorFunc) *Server {
	return &Server{
		labeller:           labeller,
		apiProxy:           apiProxy,
		instanceSpec:       instanceSpec,
		binaryPath:         binaryPath,
		enableAntiAffinity: enableAntiAffinity,
		statefulSet: resources.NewStatefulSet(
			statefulSetName,
			labeller,
			apiProxy),
		headlessService: resources.NewHeadlessService(
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

func (s *Server) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		s.statefulSet,
		s.configHelper,
		s.headlessService,
	})
}

func (s *Server) IsInSync() bool {
	if s.configHelper.NeedSync() || !resources.Exists(s.statefulSet) || !resources.Exists(s.headlessService) {
		return false
	}

	return s.statefulSet.NeedSync(
		s.instanceSpec.InstanceCount,
		s.apiProxy.Ytsaurus().Spec.CoreImage)
}

func (s *Server) ArePodsReady(ctx context.Context) bool {
	return s.statefulSet.CheckPodsReady(ctx)
}

func (s *Server) Sync(ctx context.Context) (err error) {
	_ = s.configHelper.Build()
	_ = s.headlessService.Build()
	_ = s.BuildStatefulSet()

	return resources.Sync(ctx, []resources.Syncable{
		s.statefulSet,
		s.configHelper,
		s.headlessService,
	})
}

func (s *Server) BuildStatefulSet() *appsv1.StatefulSet {
	if s.builtStatefulSet != nil {
		return s.builtStatefulSet
	}

	locationCreationCommand := getLocationInitCommand(s.instanceSpec.Locations)
	volumeMounts := createVolumeMounts(s.instanceSpec.VolumeMounts)

	statefulSet := s.statefulSet.Build()
	statefulSet.Spec.Replicas = &s.instanceSpec.InstanceCount
	statefulSet.Spec.ServiceName = s.headlessService.Name()
	statefulSet.Spec.VolumeClaimTemplates = createVolumeClaims(s.instanceSpec.VolumeClaimTemplates)

	setHostameAsFQDN := true
	statefulSet.Spec.Template.Spec = corev1.PodSpec{
		ImagePullSecrets:  s.apiProxy.Ytsaurus().Spec.ImagePullSecrets,
		SetHostnameAsFQDN: &setHostameAsFQDN,
		Containers: []corev1.Container{
			corev1.Container{
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

	if s.enableAntiAffinity {
		affinity := corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: s.labeller.GetSelectorLabelMap(),
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		}
		statefulSet.Spec.Template.Spec.Affinity = &affinity
	}
	s.builtStatefulSet = statefulSet
	return statefulSet
}
