package flows

type StepSyncStatus string
type StepName string

const (
	// StepSyncStatusDone means that step is done.
	StepSyncStatusDone StepSyncStatus = "Done"
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

type BaseStep struct {
	name StepName
}

func (s *BaseStep) GetName() StepName {
	return s.name
}
