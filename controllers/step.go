package controllers

type StepSyncStatus string

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
	name string
}

func (s *baseStep) GetName() string {
	return s.name
}
