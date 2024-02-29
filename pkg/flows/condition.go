package flows

import (
	"context"
)

type ConditionStep struct {
	name     StepName
	cond     func(ctx context.Context) (string, error)
	branches map[string][]StepType
}

func NewConditionStep(
	name StepName,
	cond func(ctx context.Context) (string, error),
	branches map[string][]StepType,
) *ConditionStep {
	return &ConditionStep{
		name:     name,
		branches: branches,
		cond:     cond,
	}
}

func (s *ConditionStep) StepName() StepName                 { return s.name }
func (s *ConditionStep) GetBranches() map[string][]StepType { return s.branches }
func (s *ConditionStep) Cacheable() bool {
	// We may want to have not cacheable condition steps in the future, but currently we don't have such need.
	return true
}
func (s *ConditionStep) RunCondition(ctx context.Context) (string, error) {
	return s.cond(ctx)
}
