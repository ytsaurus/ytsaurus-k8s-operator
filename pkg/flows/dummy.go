package flows

import (
	"context"
)

type DummyStep struct {
	Name StepName
}

func (d DummyStep) StepName() StepName { return d.Name }
func (d DummyStep) Status(context.Context) (StepStatus, error) {
	return StepStatus{SyncStatus: StepSyncStatusDone, Message: "dummy step"}, nil
}
func (d DummyStep) Run(_ context.Context) error { return nil }
