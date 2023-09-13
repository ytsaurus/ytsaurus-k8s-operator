package components

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type podsManager interface {
	removePods(ctx context.Context) error
	arePodsRemoved() bool
	arePodsReady(ctx context.Context) bool
	podsImageCorrespondsToSpec() bool
}

func removePods(ctx context.Context, manager podsManager, c *componentBase) error {
	if !isPodsRemovingStarted(c) {
		if err := manager.removePods(ctx); err != nil {
			return err
		}

		setPodsRemovingStartedCondition(c)
		return nil
	}

	if !manager.arePodsRemoved() {
		return nil
	}

	setPodsRemovedCondition(c)
	return nil
}

func isPodsRemovingStarted(c *componentBase) bool {
	return c.ytsaurus.IsUpdateStatusConditionTrue(c.labeller.GetPodsRemovingStartedCondition())
}

func setPodsRemovingStartedCondition(c *componentBase) {
	c.ytsaurus.SetUpdateStatusCondition(metav1.Condition{
		Type:    c.labeller.GetPodsRemovingStartedCondition(),
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Pods removing was started",
	})
}

func setPodsRemovedCondition(c *componentBase) {
	c.ytsaurus.SetUpdateStatusCondition(metav1.Condition{
		Type:    c.labeller.GetPodsRemovedCondition(),
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Pods removed",
	})
}
