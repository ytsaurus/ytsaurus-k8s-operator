package controllers

import (
	"context"
)

type actionStep struct {
	baseStep
	action      func(context.Context) error
	statusCheck func(context.Context, *ytsaurusState) (StepStatus, error)
}

func newActionStep(
	name StepName,
	action func(context.Context) error,
	statusCheck func(context.Context, *ytsaurusState) (StepStatus, error),
) *actionStep {
	return &actionStep{
		baseStep: baseStep{
			name: name,
		},
		action:      action,
		statusCheck: statusCheck,
	}
}
func (s *actionStep) Status(ctx context.Context, state *ytsaurusState) (StepStatus, error) {
	return s.statusCheck(ctx, state)
}
func (s *actionStep) Run(ctx context.Context) error {
	return s.action(ctx)
}
