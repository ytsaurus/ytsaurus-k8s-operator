package ytflow

import (
	"context"
)

type fakeConditionManager struct {
	// TODO: msg
	store map[Condition]bool
}

func newFakeConditionManager() *fakeConditionManager {
	return &fakeConditionManager{
		store: make(map[Condition]bool),
	}
}

func (cm *fakeConditionManager) SetTrue(ctx context.Context, cond Condition, msg string) error {
	return cm.Set(ctx, cond, true, msg)
}
func (cm *fakeConditionManager) SetFalse(ctx context.Context, cond Condition, msg string) error {
	return cm.Set(ctx, cond, false, msg)
}
func (cm *fakeConditionManager) Set(_ context.Context, cond Condition, val bool, _ string) error {
	cm.store[cond] = val
	return nil
}
func (cm *fakeConditionManager) IsTrue(condName Condition) bool {
	return cm.store[condName]
}
func (cm *fakeConditionManager) IsFalse(condName Condition) bool {
	return !cm.IsTrue(condName)
}
func (cm *fakeConditionManager) Get(condName Condition) bool {
	return cm.store[condName]
}
