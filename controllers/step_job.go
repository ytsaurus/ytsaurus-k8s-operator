package controllers

import (
	"context"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/flows"
)

func newJobStep(stepName flows.StepName, job *components.JobStateless, script string) flows.ActionStep {
	preRun := func(ctx context.Context) (flows.StepStatus, error) {
		if !job.IsPrepared() {
			if err := job.Prepare(ctx); err != nil {
				return flows.StepStatus{}, err
			}
			return flows.StepStatus{SyncStatus: flows.StepSyncStatusUpdating}, nil
		}

		return flows.StepStatus{
			SyncStatus: flows.StepSyncStatusNeedRun,
			Message:    "job is prepared",
		}, nil
	}
	run := func(ctx context.Context) error {
		job.SetInitScript(script)
		return job.Sync(ctx)
	}
	postRun := func(ctx context.Context) (flows.StepStatus, error) {
		if !job.IsCompleted() {
			return flows.StepStatus{SyncStatus: flows.StepSyncStatusUpdating}, nil
		}
		return flows.StepStatus{SyncStatus: flows.StepSyncStatusDone}, nil
	}
	return flows.ActionStep{
		Name:        stepName,
		PreRunFunc:  preRun,
		RunFunc:     run,
		PostRunFunc: postRun,
	}
}
