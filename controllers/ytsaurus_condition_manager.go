package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/flows"
)

type stepsStateManager struct {
	ytsaurusProxy *apiProxy.Ytsaurus
}

func newStepsStateManager(ytsaurusProxy *apiProxy.Ytsaurus) *stepsStateManager {
	return &stepsStateManager{
		ytsaurusProxy: ytsaurusProxy,
	}
}

func (m *stepsStateManager) StoreDone(ctx context.Context, name flows.StepName) error {
	condition := metav1.Condition{
		Type:    m.getDoneConditionName(name),
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: fmt.Sprintf("Step %s is done", name),
	}
	return m.setConditionWithRetries(ctx, condition)
}
func (m *stepsStateManager) StoreRun(ctx context.Context, name flows.StepName) error {
	// TODO: store Status for run?
	condition := metav1.Condition{
		Type:    m.getRunConditionName(name),
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: fmt.Sprintf("Step %s has been run", name),
	}
	return m.setConditionWithRetries(ctx, condition)

}
func (m *stepsStateManager) StoreConditionResult(ctx context.Context, name flows.StepName, result bool) error {
	condition := metav1.Condition{
		Type: m.getBoolConditionName(name),
		Status: map[bool]metav1.ConditionStatus{
			true:  metav1.ConditionTrue,
			false: metav1.ConditionFalse,
		}[result],
		Reason:  "Update",
		Message: fmt.Sprintf("Condition step %s results in %t branch ", name, result),
	}
	return m.setConditionWithRetries(ctx, condition)
}
func (m *stepsStateManager) setConditionWithRetries(ctx context.Context, cond metav1.Condition) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Fetch the resource here; you need to refetch it on every try, since
		// if you got a conflict on the last update attempt then you need to get
		// the current version before making your own changes.
		name := m.ytsaurusProxy.GetResource().Name
		ytsaurusResource := ytv1.Ytsaurus{}
		if err := m.ytsaurusProxy.APIProxy().FetchObject(ctx, name, &ytsaurusResource); err != nil {
			return err
		}

		meta.SetStatusCondition(&ytsaurusResource.Status.UpdateStatus.Conditions, cond)

		// You have to return err itself here (not wrapped inside another error)
		// so that RetryOnConflict can identify it correctly.
		return m.ytsaurusProxy.APIProxy().Client().Status().Update(ctx, &ytsaurusResource)
	})
	if err != nil {
		// May be conflict if max retries were hit, or may be something unrelated
		// like permissions or a network error
		return fmt.Errorf("failed to set condition %s with retries: %w", cond.Type, err)
	}
	return nil
}
func (m *stepsStateManager) IsDone(name flows.StepName) bool {
	return m.ytsaurusProxy.IsUpdateStatusConditionTrue(m.getDoneConditionName(name))
}
func (m *stepsStateManager) HasRun(name flows.StepName) bool {
	return m.ytsaurusProxy.IsUpdateStatusConditionTrue(m.getRunConditionName(name))
}
func (m *stepsStateManager) GetConditionResult(name flows.StepName) (result bool, exists bool) {
	conditionName := m.getBoolConditionName(name)
	if m.ytsaurusProxy.IsUpdateStatusConditionTrue(conditionName) {
		return true, true
	}
	if m.ytsaurusProxy.IsUpdateStatusConditionFalse(conditionName) {
		return false, true
	}

	return false, false
}
func (m *stepsStateManager) Clear(ctx context.Context) error {
	// TODO(l0kix2): here we are clearing ALL the update fields
	// this may unexpected in some cases.
	return m.ytsaurusProxy.ClearUpdateStatus(ctx)
}

func (m *stepsStateManager) SaveUpdateStatus(ctx context.Context) error {
	return m.ytsaurusProxy.APIProxy().UpdateStatus(ctx)
}

func (m *stepsStateManager) getDoneConditionName(stepName flows.StepName) string {
	return fmt.Sprintf("%sDone", stepName)
}
func (m *stepsStateManager) getRunConditionName(stepName flows.StepName) string {
	return fmt.Sprintf("%sRun", stepName)
}
func (m *stepsStateManager) getBoolConditionName(stepName flows.StepName) string {
	return fmt.Sprintf("%sCond", stepName)
}
func (m *stepsStateManager) getAllSetConditions() []string {
	var result []string
	for _, cond := range m.ytsaurusProxy.GetResource().Status.UpdateStatus.Conditions {
		result = append(result, cond.Type)
	}
	return result
}
