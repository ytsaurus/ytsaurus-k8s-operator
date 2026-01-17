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

func (s *StatefulSet) ListPods(ctx context.Context) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := s.proxy.ListObjects(ctx, podList, s.labeller.GetListOptions()...); err != nil {
		return nil, err
	}
	return podList.Items, nil
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
