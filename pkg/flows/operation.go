package flows

import (
	"context"
)

// OperationStep is a runnable step which is not becoming done instantly and flow needs to check its Status
// until it's done.
type OperationStep struct {
	ActionStep
	statusCheck func(context.Context) (StepStatus, error)
}

func NewOperationStep(
	name StepName,
	statusCheck func(context.Context) (StepStatus, error),
	action func(ctx context.Context) error,
) *OperationStep {
	return &OperationStep{
		ActionStep: ActionStep{
			name:   name,
			action: action,
		},
		statusCheck: statusCheck,
	}
}

func (s *OperationStep) Cacheable() bool {
	// We may want to have not cacheable operation steps in the future, but currently we don't have such need.
	return true
}
func (s *OperationStep) Status(ctx context.Context) (StepStatus, error) {
	return s.statusCheck(ctx)
}
