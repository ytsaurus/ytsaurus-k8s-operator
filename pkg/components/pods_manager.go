package components

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
)

// TODO: move to Updatable
type podsManager interface {
	removePods(ctx context.Context) error
	arePodsRemoved(ctx context.Context) bool
	arePodsReady(ctx context.Context) bool
	podsImageCorrespondsToSpec() bool
}

func removePods(ctx context.Context, manager podsManager, c *ytsaurusComponent) error {
	if !isPodsRemovingStarted(c) {
		if err := manager.removePods(ctx); err != nil {
			return err
		}

		setPodsRemovingStartedCondition(c)
		return nil
	}

	if !manager.arePodsRemoved(ctx) {
		return nil
	}

	setPodsRemovedCondition(c)
	return nil
}

func isPodsRemovingStarted(c *ytsaurusComponent) bool {
	return c.ytsaurus.IsUpdateStatusConditionTrue(c.labeller.GetPodsRemovingStartedCondition())
}

func setPodsRemovingStartedCondition(c *ytsaurusComponent) {
	c.ytsaurus.SetUpdateStatusCondition(metav1.Condition{
		Type:    c.labeller.GetPodsRemovingStartedCondition(),
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Pods removing was started",
	})
}

func setPodsRemovedCondition(c *ytsaurusComponent) {
	c.ytsaurus.SetUpdateStatusCondition(metav1.Condition{
		Type:    labeller.GetPodsRemovedCondition(c.GetName()),
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Pods removed",
	})
}
