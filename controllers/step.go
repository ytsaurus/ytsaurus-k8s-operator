package controllers

type StepSyncStatus string
type StepName string

const (
	// StepSyncStatusDone means that step is done.
	StepSyncStatusDone StepSyncStatus = "Done"
	// StepSyncStatusSkip means that step must be skipped.
	StepSyncStatusSkip StepSyncStatus = "Skip"
	// StepSyncStatusUpdating means that step execution is in progress,
	// but currently nothing can be done by operator but wait.
	StepSyncStatusUpdating StepSyncStatus = "Updating"
	// StepSyncStatusBlocked means that step can't be executed for some reason,
	// and it is possible that human needed to proceed.
	StepSyncStatusBlocked StepSyncStatus = "Blocked"
	// StepSyncStatusNeedRun means that step should be executed.
	StepSyncStatusNeedRun StepSyncStatus = "NeedRun"
)

type StepStatus struct {
	SyncStatus StepSyncStatus
	Message    string
}

type baseStep struct {
	name StepName
}

func (s *baseStep) GetName() StepName {
	return s.name
}

type executionStats struct {
	statuses map[StepName]StepStatus
}

func newExecutionStats() executionStats {
	return executionStats{
		statuses: make(map[StepName]StepStatus),
	}
}

func (s *executionStats) Collect(name StepName, status StepStatus) {
	s.statuses[name] = status
}

func (s *executionStats) isSkipped(name StepName) bool {
	status, wasExecuted := s.statuses[name]
	if !wasExecuted {
		return false
	}
	return status.SyncStatus == StepSyncStatusSkip
}
