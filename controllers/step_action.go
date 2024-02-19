package controllers

import (
	"context"
)

type actionStep struct {
	baseStep
	action      func(context.Context, *ytsaurusState) error
	statusCheck func(context.Context, *ytsaurusState) (StepStatus, error)
	doneCheck   func(context.Context, *ytsaurusState) (bool, error)
}

func newActionStep(
	name StepName,
	action func(context.Context, *ytsaurusState) error,
	statusCheck func(context.Context, *ytsaurusState) (StepStatus, error),
	doneFunc func(context.Context, *ytsaurusState) (bool, error),
) *actionStep {
	return &actionStep{
		baseStep: baseStep{
			name: name,
		},
		action:      action,
		statusCheck: statusCheck,
		doneCheck:   doneFunc,
	}
}

func newActionStepWithDoneCondition(
	name StepName,
	action func(context.Context, *ytsaurusState) error,
	statusCheck func(context.Context, *ytsaurusState) (StepStatus, error),
	doneCondition string,
) *actionStep {
	return newActionStep(
		name,
		action,
		statusCheck,
		func(ctx context.Context, state *ytsaurusState) (bool, error) {
			return state.isUpdateStatusConditionTrue(doneCondition), nil
		},
	)
}

func (s *actionStep) Status(ctx context.Context, state *ytsaurusState) (StepStatus, error) {
	status, err := s.statusCheck(ctx, state)
	if err != nil {
		return StepStatus{}, err
	}
	if status.SyncStatus != StepSyncStatusNeedRun {
		return status, nil
	}
	done, err := s.doneCheck(ctx, state)
	if err != nil {
		return StepStatus{}, err
	}
	if done {
		return StepStatus{StepSyncStatusDone, ""}, nil
	}
	return status, nil
}
func (s *actionStep) Run(ctx context.Context, state *ytsaurusState) error {
	return s.action(ctx, state)
}
