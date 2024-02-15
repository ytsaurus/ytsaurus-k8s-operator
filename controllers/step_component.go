package controllers

import (
	"context"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type componentStep struct {
	baseStep
	component components.Component2
}

func newComponentStep(component components.Component2) *componentStep {
	return &componentStep{
		baseStep: baseStep{
			name: StepName(component.GetName()),
		},
		component: component,
	}
}
func (s *componentStep) Status(ctx context.Context, _ executionStats) (StepStatus, error) {
	componentStatus, err := s.component.Status2(ctx)
	if err != nil {
		return StepStatus{}, err
	}
	stepSyncStatus := map[components.SyncStatus]StepSyncStatus{
		// NB: no StepSyncStatusSkip here: component step is not meant to be skipped.
		components.SyncStatusReady:           StepSyncStatusDone,
		components.SyncStatusPending:         StepSyncStatusUpdating,
		components.SyncStatusUpdating:        StepSyncStatusUpdating,
		components.SyncStatusBlocked:         StepSyncStatusBlocked,
		components.SyncStatusNeedFullUpdate:  StepSyncStatusNeedRun,
		components.SyncStatusNeedLocalUpdate: StepSyncStatusNeedRun,
	}[componentStatus.SyncStatus]
	return StepStatus{
		SyncStatus: stepSyncStatus,
		Message:    componentStatus.Message,
	}, nil
}
func (s *componentStep) Run(ctx context.Context) error {
	err := s.component.Fetch(ctx)
	if err != nil {
		return err
	}
	return s.component.Sync2(ctx)
}
