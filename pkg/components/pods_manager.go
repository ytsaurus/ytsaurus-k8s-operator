package components

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

// TODO: move to Updatable
type podsManager interface {
	removePods(ctx context.Context) error
	arePodsRemoved(ctx context.Context) bool
	arePodsReady(ctx context.Context) bool
	podsImageCorrespondsToSpec() bool
}

func removePods(ctx context.Context, manager podsManager, conditionsManager apiproxy.UpdateConditionManager, labeller *labeller.Labeller) error {
	started := labeller.GetPodsRemovingStartedCondition()
	if !conditionsManager.IsUpdateStatusConditionTrue(started) {
		if err := manager.removePods(ctx); err != nil {
			return err
		}
		conditionsManager.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    started,
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: "Pods removing was started",
		})
		return nil
	}

	if !manager.arePodsRemoved(ctx) {
		return nil
	}

	conditionsManager.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    labeller.GetPodsRemovedCondition(),
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Pods removed",
	})
	return nil
}
