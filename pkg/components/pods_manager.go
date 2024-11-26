package components

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: move to Updatable
type podsManager interface {
	removePods(ctx context.Context) error
	arePodsRemoved(ctx context.Context) bool
	arePodsReady(ctx context.Context) bool
	podsImageCorrespondsToSpec() bool
}

func removePods(ctx context.Context, manager podsManager, c *localComponent) error {
	if !isPodsRemovingStarted(c) {
		if err := manager.removePods(ctx); err != nil {
			return err
		}

		setPodsRemovingStartedCondition(ctx, c)
		return nil
	}

	if !manager.arePodsRemoved(ctx) {
		return nil
	}

	setPodsRemovedCondition(ctx, c)
	return nil
}

func isPodsRemovingStarted(c *localComponent) bool {
	return c.ytsaurus.IsUpdateStatusConditionTrue(c.labeller.GetPodsRemovingStartedCondition())
}

func setPodsRemovingStartedCondition(ctx context.Context, c *localComponent) {
	c.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    c.labeller.GetPodsRemovingStartedCondition(),
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Pods removing was started",
	})
}

func setPodsRemovedCondition(ctx context.Context, c *localComponent) {
	c.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    c.labeller.GetPodsRemovedCondition(),
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Pods removed",
	})
}
