package flows

import (
	"context"
	"errors"
)

type cachelessStep struct {
	name      StepName
	runTimes  int
	status    StepStatus
	statusErr error
}

func newCachelessStep(name StepName, syncStatus StepSyncStatus) *cachelessStep {
	return &cachelessStep{
		name:   name,
		status: StepStatus{SyncStatus: syncStatus, Message: "test status"},
	}
}

func newStatusErrorStep(name StepName, syncStatus StepSyncStatus) *cachelessStep {
	return &cachelessStep{
		name:      name,
		status:    StepStatus{SyncStatus: syncStatus, Message: "test status"},
		statusErr: errors.New(testStatusErrorMsg),
	}
}

func (s *cachelessStep) StepName() StepName { return s.name }
func (s *cachelessStep) Cacheable() bool    { return false }
func (s *cachelessStep) Run(context.Context) error {
	s.runTimes++
	return nil
}
func (s *cachelessStep) Status(context.Context) (StepStatus, error) {
	return s.status, s.statusErr
}
