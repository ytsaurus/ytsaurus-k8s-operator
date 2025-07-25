package apiproxy

import (
	"cmp"
	"context"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ConditionManager interface {
	SetStatusCondition(condition metav1.Condition)
	IsStatusConditionTrue(conditionType string) bool
	IsStatusConditionFalse(conditionType string) bool

	SetUpdateStatusCondition(ctx context.Context, condition metav1.Condition)
	IsUpdateStatusConditionTrue(condition string) bool
}

func NewConditionManager(
	conditions *[]metav1.Condition,
	updateConditions *[]metav1.Condition,
) ConditionManager {
	return &conditionManager{
		conditions:       conditions,
		updateConditions: updateConditions,
	}
}

type conditionManager struct {
	conditions       *[]metav1.Condition
	updateConditions *[]metav1.Condition
}

func sortConditions(conditions []metav1.Condition) {
	slices.SortStableFunc(conditions, func(a, b metav1.Condition) int {
		statusOrder := []metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse, metav1.ConditionUnknown}
		if diff := cmp.Compare(slices.Index(statusOrder, a.Status), slices.Index(statusOrder, b.Status)); diff != 0 {
			return diff
		}
		return a.LastTransitionTime.Compare(b.LastTransitionTime.Time)
	})
}

func (c *conditionManager) SetStatusCondition(condition metav1.Condition) {
	meta.SetStatusCondition(c.conditions, condition)
	sortConditions(*c.conditions)
}

func (c *conditionManager) IsStatusConditionTrue(conditionType string) bool {
	return meta.IsStatusConditionTrue(*c.conditions, conditionType)
}

func (c *conditionManager) IsStatusConditionFalse(conditionType string) bool {
	return meta.IsStatusConditionFalse(*c.conditions, conditionType)
}

func (c *conditionManager) IsUpdateStatusConditionTrue(condition string) bool {
	return meta.IsStatusConditionTrue(*c.updateConditions, condition)
}

func (c *conditionManager) SetUpdateStatusCondition(ctx context.Context, condition metav1.Condition) {
	logger := log.FromContext(ctx)
	logger.Info("Setting update status condition", "condition", condition)
	meta.SetStatusCondition(c.updateConditions, condition)
	sortConditions(*c.updateConditions)
}
