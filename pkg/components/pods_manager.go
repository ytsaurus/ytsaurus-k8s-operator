package components

import (
	"context"
	"slices"

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

func countReadyPods(pods []corev1.Pod, containerNames []string) int32 {
	var readyPods int32
	for _, pod := range pods {
		if containers := len(containerNames); containers > 0 {
			for _, ct := range pod.Status.ContainerStatuses {
				if ct.Ready && slices.Contains(containerNames, ct.Name) {
					containers--
				}
			}
			if containers == 0 {
				readyPods += 1
			}
		} else if pod.Status.Phase == corev1.PodRunning {
			readyPods += 1
		}
	}
	return readyPods
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
