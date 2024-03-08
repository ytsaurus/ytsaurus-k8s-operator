package ytflow

import (
	"context"
)

type fakeConditionManager struct {
	// TODO: msg
	store map[conditionName]bool
}

func newFakeConditionManager() *fakeConditionManager {
	return &fakeConditionManager{
		store: make(map[conditionName]bool),
	}
}

func (cm *fakeConditionManager) SetTrue(ctx context.Context, cond conditionName, msg string) error {
	return cm.Set(ctx, cond, true, msg)
}
func (cm *fakeConditionManager) SetFalse(ctx context.Context, cond conditionName, msg string) error {
	return cm.Set(ctx, cond, false, msg)
}
func (cm *fakeConditionManager) Set(_ context.Context, cond conditionName, val bool, _ string) error {
	cm.store[cond] = val
	return nil
}
func (cm *fakeConditionManager) IsTrue(condName conditionName) bool {
	return cm.store[condName]
}
func (cm *fakeConditionManager) IsFalse(condName conditionName) bool {
	return !cm.IsTrue(condName)
}
func (cm *fakeConditionManager) IsSatisfied(condDep conditionDependency) bool {
	return condDep.val == cm.store[condDep.name]
}
func (cm *fakeConditionManager) Get(condName conditionName) bool {
	return cm.store[condName]
}
