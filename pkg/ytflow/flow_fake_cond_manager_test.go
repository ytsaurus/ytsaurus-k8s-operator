package ytflow

import (
	"context"
)

type fakeConditionManager struct {
	// TODO: msg
	store map[condition]bool
}

func newFakeConditionManager() *fakeConditionManager {
	return &fakeConditionManager{
		store: make(map[condition]bool),
	}
}

func (cm *fakeConditionManager) SetTrue(ctx context.Context, cond condition, msg string) error {
	return cm.Set(ctx, cond, true, msg)
}
func (cm *fakeConditionManager) SetFalse(ctx context.Context, cond condition, msg string) error {
	return cm.Set(ctx, cond, false, msg)
}
func (cm *fakeConditionManager) Set(_ context.Context, cond condition, val bool, _ string) error {
	cm.store[cond] = val
	return nil
}
func (cm *fakeConditionManager) IsTrue(condName condition) bool {
	return cm.store[condName]
}
func (cm *fakeConditionManager) IsFalse(condName condition) bool {
	return !cm.IsTrue(condName)
}
func (cm *fakeConditionManager) IsSatisfied(condDep conditionDependency) bool {
	return condDep.val == cm.store[condDep.name]
}
func (cm *fakeConditionManager) Get(condName condition) bool {
	return cm.store[condName]
}
