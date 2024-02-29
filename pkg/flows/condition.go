package flows

import (
	"context"
)

type BoolConditionStep struct {
	Name  StepName
	Cond  func(ctx context.Context) (bool, error)
	True  []StepType
	False []StepType
}

func (s BoolConditionStep) StepName() StepName {
	return s.Name
}
func (s BoolConditionStep) GetBranch(branch bool) []StepType {
	if branch {
		return s.True
	}
	return s.False
}
func (s BoolConditionStep) AutoManaged() bool {
	// We may want to have not cacheable condition steps in the future, but currently we don't have such need.
	return true
}
func (s BoolConditionStep) RunCondition(ctx context.Context) (bool, error) {
	return s.Cond(ctx)
}
