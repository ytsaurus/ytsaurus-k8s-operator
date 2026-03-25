package components

import (
	"context"
	"slices"

	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

// TODO: move to Updatable
type podsManager interface {
	getInstanceCount() int32
	setInstanceCount(instanceCount int32)
	getMinReadyInstanceCount(margin int32) int32
	listPods(ctx context.Context) ([]corev1.Pod, error)

	removePods(ctx context.Context) error
	arePodsReady(ctx context.Context) bool
	podsImageCorrespondsToSpec() bool
	arePodsUpdatedToNewRevision(ctx context.Context) bool
}

func podConditionIsTrue(pod *corev1.Pod, conditionType corev1.PodConditionType) bool {
	return slices.ContainsFunc(pod.Status.Conditions, func(c corev1.PodCondition) bool {
		return c.Type == conditionType && c.Status == corev1.ConditionTrue
	})
}

func podContainersAreRunning(pod *corev1.Pod, names []string) bool {
	return !slices.ContainsFunc(pod.Status.ContainerStatuses, func(s corev1.ContainerStatus) bool {
		return (!ptr.Deref(s.Started, false) || s.State.Running == nil) && (len(names) == 0 || slices.Contains(names, s.Name))
	})
}

func podContainersAreReady(pod *corev1.Pod, names []string) bool {
	if len(names) == 0 && !podConditionIsTrue(pod, corev1.ContainersReady) {
		return false
	}
	return !slices.ContainsFunc(pod.Status.ContainerStatuses, func(s corev1.ContainerStatus) bool {
		return !s.Ready && slices.Contains(names, s.Name)
	})
}

func countPods(pods []corev1.Pod, containerNames []string) (running, ready int32) {
	for _, pod := range pods {
		if podContainersAreRunning(&pod, containerNames) {
			running += 1
		}
		if podContainersAreReady(&pod, containerNames) {
			ready += 1
		}
	}
	return running, ready
}

func removePods(ctx context.Context, manager podsManager, c *component) error {
	removingStarted := c.labeller.GetPodsRemovingStartedCondition()
	if !c.ytsaurus.IsUpdateStatusConditionTrue(removingStarted) {
		if err := manager.removePods(ctx); err != nil {
			return err
		}
		c.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    removingStarted,
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: "Pods removing was started",
		})
		return nil
	}
	if pods, err := manager.listPods(ctx); err != nil {
		return err
	} else if len(pods) == 0 {
		c.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    c.labeller.GetPodsRemovedCondition(),
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: "Pods removed",
		})
	}
	return nil
}
