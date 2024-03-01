package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

func (m *stepsStateManager) StoreDone(name flows.StepName) {
	resource := m.ytsaurusProxy.GetResource()
	meta.SetStatusCondition(&resource.Status.UpdateStatus.Conditions, metav1.Condition{
		Type:    m.getDoneConditionName(name),
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: fmt.Sprintf("Step %s is done", name),
	})
}
func (m *stepsStateManager) StoreRun(name flows.StepName) {
	// TODO: store Status for run?
	resource := m.ytsaurusProxy.GetResource()
	meta.SetStatusCondition(&resource.Status.UpdateStatus.Conditions, metav1.Condition{
		Type:    m.getRunConditionName(name),
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: fmt.Sprintf("Step %s has been run", name),
	})
}
func (m *stepsStateManager) StoreConditionResult(name flows.StepName, result bool) {
	resource := m.ytsaurusProxy.GetResource()
	meta.SetStatusCondition(&resource.Status.UpdateStatus.Conditions, metav1.Condition{
		Type: m.getBoolConditionName(name),
		Status: map[bool]metav1.ConditionStatus{
			true:  metav1.ConditionTrue,
			false: metav1.ConditionFalse,
		}[result],
		Reason:  "Update",
		Message: fmt.Sprintf("Condition step %s results in %t branch ", name, result),
	})
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
