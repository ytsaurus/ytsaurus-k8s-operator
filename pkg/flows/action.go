package flows

import (
	"context"
)

// ActionStep implements autoManagedRunnableStep.
// It has no New* method for readability in a place of creation,
// because any of preRun, run, postRun may be omitted.
// Intended usage is simple create a structure.
type ActionStep struct {
	Name        StepName
	PreRunFunc  func(context.Context) (StepStatus, error)
	RunFunc     func(ctx context.Context) error
	PostRunFunc func(context.Context) (StepStatus, error)
}

func (s ActionStep) StepName() StepName { return s.Name }
func (s ActionStep) PreRun(ctx context.Context) (StepStatus, error) {
	if s.PreRunFunc == nil {
		return StepStatus{SyncStatus: StepSyncStatusNeedRun, Message: "no pre-run action for step"}, nil
	}
	return s.PreRunFunc(ctx)
}
func (s ActionStep) Run(ctx context.Context) error {
	if s.RunFunc == nil {
		return nil
	}
	return s.RunFunc(ctx)
}
func (s ActionStep) PostRun(ctx context.Context) (StepStatus, error) {
	if s.PostRunFunc == nil {
		return StepStatus{SyncStatus: StepSyncStatusDone, Message: "no post-run action for step"}, nil
	}
	return s.PostRunFunc(ctx)
}
