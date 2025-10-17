package resources

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	labeller2 "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type StatefulSet struct {
	BaseManagedResource[*appsv1.StatefulSet]

	commonSpec ytv1.CommonSpec
	built      bool
}

func NewStatefulSet(
	name string,
	labeller *labeller2.Labeller,
	proxy apiproxy.APIProxy,
	commonSpec ytv1.CommonSpec,
) *StatefulSet {
	return &StatefulSet{
		BaseManagedResource: BaseManagedResource[*appsv1.StatefulSet]{
			proxy:     proxy,
			labeller:  labeller,
			name:      name,
			oldObject: &appsv1.StatefulSet{},
			newObject: &appsv1.StatefulSet{},
		},
		commonSpec: commonSpec,
	}
}

func (s *StatefulSet) Build() *appsv1.StatefulSet {
	if !s.built {
		s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)
		s.newObject.Spec = appsv1.StatefulSetSpec{
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: s.labeller.GetSelectorLabelMap(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      s.labeller.GetMetaLabelMap(false),
					Annotations: s.commonSpec.ExtraPodAnnotations,
				},
			},
		}
	}

	s.built = true
	return s.newObject
}

func (s *StatefulSet) getPods(ctx context.Context) *corev1.PodList {
	logger := log.FromContext(ctx)
	podList := &corev1.PodList{}
	err := s.proxy.ListObjects(ctx, podList, s.labeller.GetListOptions()...)
	if err != nil {
		logger.Error(err, "unable to list pods for component", "component", s.labeller.GetFullComponentName())
		return nil
	}

	return podList
}

func (s *StatefulSet) ArePodsRemoved(ctx context.Context) bool {
	logger := log.FromContext(ctx)

	podList := s.getPods(ctx)
	if podList == nil {
		return false
	}
	podsCount := len(podList.Items)
	if podsCount != 0 {
		logger.Info("there are pods", "podsCount", podsCount, "component", s.labeller.GetFullComponentName())
		return false
	}
	return true
}

func checkReadinessByContainers(pod corev1.Pod, byContainerNames []string) bool {
	found := 0
	for _, containerNameToCheck := range byContainerNames {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.Name != containerNameToCheck {
				continue
			}
			if !containerStatus.Ready {
				return false
			}
			found++
		}
	}
	return found == len(byContainerNames)
}

func (s *StatefulSet) ArePodsReady(ctx context.Context, instanceCount int, minReadyInstanceCount *int, byContainerNames []string) bool {
	logger := log.FromContext(ctx)
	podList := s.getPods(ctx)
	if podList == nil {
		return false
	}

	effectiveMinReadyInstanceCount := instanceCount
	if minReadyInstanceCount != nil && *minReadyInstanceCount < instanceCount {
		effectiveMinReadyInstanceCount = *minReadyInstanceCount
	}

	readyInstanceCount := 0
	for _, pod := range podList.Items {
		var ready bool
		if len(byContainerNames) > 0 {
			ready = checkReadinessByContainers(pod, byContainerNames)
		} else {
			ready = pod.Status.Phase == corev1.PodRunning
		}
		if !ready {
			logger.Info("pod is not yet running", "podName", pod.Name, "phase", pod.Status.Phase)
		} else {
			readyInstanceCount += 1
		}
	}

	if readyInstanceCount < effectiveMinReadyInstanceCount {
		logger.Info(
			"not enough pods are running",
			"component", s.labeller.GetFullComponentName(),
			"readyInstanceCount", readyInstanceCount,
			"minReadyInstanceCount", effectiveMinReadyInstanceCount,
			"totalInstanceCount", len(podList.Items))
		return false
	}
	logger.Info(
		"pods are ready",
		"component", s.labeller.GetFullComponentName(),
		"readyInstanceCount", readyInstanceCount,
		"minReadyInstanceCount", effectiveMinReadyInstanceCount,
		"totalInstanceCount", len(podList.Items))
	return true
}

func (s *StatefulSet) NeedSync(replicas int32) bool {
	return s.oldObject.Spec.Replicas == nil ||
		*s.oldObject.Spec.Replicas != replicas
}

// SpecChanged compares the old StatefulSet spec with a new spec to detect changes
func (s *StatefulSet) SpecChanged(newSpec appsv1.StatefulSetSpec) bool {
	return !statefulSetSpecEqual(s.oldObject.Spec, newSpec)
}

func statefulSetSpecEqual(oldSpec, newSpec appsv1.StatefulSetSpec) bool {
	if !podTemplateSpecEqual(oldSpec.Template, newSpec.Template) {
		return false
	}

	// Compare volume claims
	// if len(oldSpec.VolumeClaimTemplates) != len(newSpec.VolumeClaimTemplates) {
	// 	return false
	// }
	// // Compare update strategy
	// if !updateStrategyEqual(oldSpec.UpdateStrategy, newSpec.UpdateStrategy) {
	// 	return false
	// }

	return true
}

// podTemplateSpecEqual compares only the fields we explicitly set, avoiding false positives
// from Kubernetes-added defaults and normalizations.
func podTemplateSpecEqual(oldTemplate, newTemplate corev1.PodTemplateSpec) bool {
	oldPodSpec := oldTemplate.Spec
	newPodSpec := newTemplate.Spec

	// Compare containers
	if len(oldPodSpec.Containers) != len(newPodSpec.Containers) {
		return false
	}
	for i := range oldPodSpec.Containers {
		oldContainer := oldPodSpec.Containers[i]
		newContainer := newPodSpec.Containers[i]

		// Compare resource requirements (only if new spec has resources defined)
		if !resourceRequirementsEqual(oldContainer.Resources, newContainer.Resources) {
			return false
		}
	}

	// Compare init containers
	// if len(oldPodSpec.InitContainers) != len(newPodSpec.InitContainers) {
	// 	return false
	// }
	// for i := range oldPodSpec.InitContainers {
	// 	oldContainer := oldPodSpec.InitContainers[i]
	// 	newContainer := newPodSpec.InitContainers[i]

	// 	if !resourceRequirementsEqual(oldContainer.Resources, newContainer.Resources) {
	// 		return false
	// 	}
	// }

	return true
}

// resourceRequirementsEqual compares resource requirements, handling nil vs empty cases
func resourceRequirementsEqual(oldResourceRequirements, newResourceRequirements corev1.ResourceRequirements) bool {
	// Compare requests
	if !resourceListEqual(oldResourceRequirements.Requests, newResourceRequirements.Requests) {
		return false
	}

	// Compare limits
	if !resourceListEqual(oldResourceRequirements.Limits, newResourceRequirements.Limits) {
		return false
	}

	return true
}

// resourceListEqual compares resource lists, treating nil and empty as equivalent
func resourceListEqual(oldResourceList, newResourceList corev1.ResourceList) bool {
	// If both are nil or empty, they're equal
	if len(oldResourceList) == 0 && len(newResourceList) == 0 {
		return true
	}

	// If one is empty and the other isn't, they're not equal
	if len(oldResourceList) != len(newResourceList) {
		return false
	}

	// Compare each resource
	for key, newVal := range newResourceList {
		oldVal, exists := oldResourceList[key]
		if !exists || !oldVal.Equal(newVal) {
			return false
		}
	}

	return true
}

// updateStrategyEqual compares StatefulSet update strategies
func updateStrategyEqual(oldStrategy, newStrategy appsv1.StatefulSetUpdateStrategy) bool {
	// Compare strategy type
	if oldStrategy.Type != newStrategy.Type {
		return false
	}

	// If both are RollingUpdate, compare the rolling update config
	if oldStrategy.Type == appsv1.RollingUpdateStatefulSetStrategyType {
		// Handle nil cases
		if oldStrategy.RollingUpdate == nil && newStrategy.RollingUpdate == nil {
			return true
		}
		if oldStrategy.RollingUpdate == nil || newStrategy.RollingUpdate == nil {
			return true
		}

		// Compare partition if both are set
		if oldStrategy.RollingUpdate.Partition != nil && newStrategy.RollingUpdate.Partition != nil {
			return *oldStrategy.RollingUpdate.Partition == *newStrategy.RollingUpdate.Partition
		}

		// If one has partition and the other doesn't, treat nil as 0
		oldPartition := int32(0)
		if oldStrategy.RollingUpdate.Partition != nil {
			oldPartition = *oldStrategy.RollingUpdate.Partition
		}
		newPartition := int32(0)
		if newStrategy.RollingUpdate.Partition != nil {
			newPartition = *newStrategy.RollingUpdate.Partition
		}
		return oldPartition == newPartition
	}

	return true
}
