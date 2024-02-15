package controllers

import (
	"context"
)

type actionStep struct {
	baseStep
	action      func(context.Context) error
	statusCheck func(context.Context) (StepStatus, error)
}

func newActionStep(
	name string,
	action func(context.Context) error,
	statusCheck func(context.Context) (StepStatus, error),
) *actionStep {
	return &actionStep{
		baseStep: baseStep{
			name: name,
		},
		action:      action,
		statusCheck: statusCheck,
	}
}
func (s *actionStep) Status(ctx context.Context) (StepStatus, error) {
	return s.statusCheck(ctx)
}
func (s *actionStep) Run(ctx context.Context) error {
	return s.action(ctx)
}
