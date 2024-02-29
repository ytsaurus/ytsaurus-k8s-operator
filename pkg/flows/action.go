package flows

import (
	"context"
)

// ActionStep is a runnable step
// which becoming done instantly after being run.
type ActionStep struct {
	name   StepName
	action func(ctx context.Context) error
}

func NewActionStep(
	name StepName,
	action func(ctx context.Context) error,
) *ActionStep {
	return &ActionStep{
		name:   name,
		action: action,
	}
}

func (s *ActionStep) StepName() StepName { return s.name }
func (s *ActionStep) Run(ctx context.Context) error {
	return s.action(ctx)
}
