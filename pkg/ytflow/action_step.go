package ytflow

import (
	"context"
	"fmt"
)

type ActionPreRunSubStatus string
type ActionPostRunSubStatus string

const (
	// ActionBlocked means that step can't be executed for some reason,
	// and it is possible that human needed to proceed.
	ActionBlocked ActionPreRunSubStatus = "Blocked"
	// ActionNeedRun means that step should be executed.
	ActionNeedRun ActionPreRunSubStatus = "NeedRun"

	// ActionDone means that step is done.
	ActionDone ActionPostRunSubStatus = "Done"
	// ActionUpdating means that step execution is in progress,
	// but currently nothing can be done by operator but wait.
	ActionUpdating ActionPostRunSubStatus = "Updating"
)

type ActionPreRunStatus struct {
	ActionSubStatus ActionPreRunSubStatus
	Message         string
}
type ActionPostRunStatus struct {
	ActionSubStatus ActionPostRunSubStatus
	Message         string
}

type actionStep struct {
	name        stepName
	preRunFunc  func(context.Context) (ActionPreRunStatus, error)
	runFunc     func(ctx context.Context) error
	postRunFunc func(context.Context) (ActionPostRunStatus, error)

	conds conditionManagerType
}

func (s actionStep) Run(ctx context.Context) error {
	var err error

	if s.postRunFunc != nil {
		var status ActionPreRunStatus
		status, err = s.preRunFunc(ctx)
		if err != nil {
			return err
		}

		switch status.ActionSubStatus {
		case ActionBlocked:
			return s.conds.SetTrue(ctx, isBlocked(s.name), status.Message)
		case ActionNeedRun:
			// can proceed to run
		default:
			return fmt.Errorf("unexpected status %s for pre-run", status)
		}
	}

	if s.runFunc != nil {
		if err = s.runFunc(ctx); err != nil {
			return err
		}
		if err = s.conds.SetTrue(ctx, isRun(s.name), ""); err != nil {
			return err
		}
	}

	if s.postRunFunc != nil {
		var status ActionPostRunStatus
		status, err = s.postRunFunc(ctx)
		if err != nil {
			return err
		}

		switch status.ActionSubStatus {
		case ActionDone:
			return s.conds.SetTrue(ctx, isDone(s.name), status.Message)
		case ActionUpdating:
			return s.conds.SetTrue(ctx, isUpdating(s.name), status.Message)
		default:
			return fmt.Errorf("unexpected status %s for post-run", status)
		}
	}
	return nil
}
