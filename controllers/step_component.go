package controllers

import (
	"context"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/flows"
)

type componentStep struct {
	name flows.StepName
	// status of the component is observed before step is built and stored in step.
	status    components.ComponentStatus
	component component
}

func newComponentStep(component component, status components.ComponentStatus) *componentStep {
	return &componentStep{
		name:      flows.StepName(component.GetName()),
		status:    status,
		component: component,
	}
}

func (s *componentStep) StepName() flows.StepName {
	return s.name
}

func (s *componentStep) Status(_ context.Context) (flows.StepStatus, error) {
	stepSyncStatus := map[components.SyncStatus]flows.StepSyncStatus{
		// NB: no StepSyncStatusSkip here: component step is not meant to be skipped.
		components.SyncStatusReady:           flows.StepSyncStatusDone,
		components.SyncStatusPending:         flows.StepSyncStatusUpdating,
		components.SyncStatusUpdating:        flows.StepSyncStatusUpdating,
		components.SyncStatusBlocked:         flows.StepSyncStatusBlocked,
		components.SyncStatusNeedFullUpdate:  flows.StepSyncStatusNeedRun,
		components.SyncStatusNeedLocalUpdate: flows.StepSyncStatusNeedRun,
	}[s.status.SyncStatus]
	return flows.StepStatus{
		SyncStatus: stepSyncStatus,
		Message:    s.status.Message,
	}, nil
}
func (s *componentStep) Run(ctx context.Context) error {
	return s.component.Sync(ctx)
}
