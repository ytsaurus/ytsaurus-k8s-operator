package flows

import (
	"context"
	"errors"
)

type selfManagedStep struct {
	name      StepName
	runTimes  int
	status    StepStatus
	statusErr error
}

func newSelfManagedStep(name StepName, syncStatus StepSyncStatus) *selfManagedStep {
	return &selfManagedStep{
		name:   name,
		status: StepStatus{SyncStatus: syncStatus, Message: "test status"},
	}
}

func newSelfManagedErrorStep(name StepName, syncStatus StepSyncStatus) *selfManagedStep {
	return &selfManagedStep{
		name:      name,
		status:    StepStatus{SyncStatus: syncStatus, Message: "test status"},
		statusErr: errors.New(testStatusErrorMsg),
	}
}

func (s *selfManagedStep) StepName() StepName { return s.name }
func (s *selfManagedStep) Run(context.Context) error {
	s.runTimes++
	return nil
}
func (s *selfManagedStep) Status(context.Context) (StepStatus, error) {
	return s.status, s.statusErr
}
