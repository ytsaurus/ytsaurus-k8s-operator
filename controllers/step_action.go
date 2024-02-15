package controllers

import (
	"context"
)

type actionStep struct {
	baseStep
	action      func(context.Context) error
	statusCheck func(context.Context, executionStats) (StepStatus, error)
}

func newActionStep(
	name StepName,
	action func(context.Context) error,
	statusCheck func(context.Context, executionStats) (StepStatus, error),
) *actionStep {
	return &actionStep{
		baseStep: baseStep{
			name: name,
		},
		action:      action,
		statusCheck: statusCheck,
	}
}
func (s *actionStep) Status(ctx context.Context, execStats executionStats) (StepStatus, error) {
	return s.statusCheck(ctx, execStats)
}
func (s *actionStep) Run(ctx context.Context) error {
	return s.action(ctx)
}
