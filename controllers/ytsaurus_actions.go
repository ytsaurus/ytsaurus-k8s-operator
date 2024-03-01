package controllers

import (
	"context"

	"go.ytsaurus.tech/yt/go/yt"
	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/flows"
)

// not sure if we need flows.StepType names
const (
	IsFullUpdateStepName               flows.StepName = "isFullUpdate"
	CheckFullUpdatePossibilityStepName flows.StepName = "checkFullUpdatePossibility"
	EnableSafeModeStepName             flows.StepName = "enableSafeMode"
	BackupTabletCellsStepName          flows.StepName = "backupTabletCells"
	BuildMasterSnapshotsStepName       flows.StepName = "buildMasterSnapshots"
	MasterExitReadOnlyStepName         flows.StepName = "masterExitReadOnly"
	RecoverTabletCellsStepName         flows.StepName = "recoverTabletCells"
	InitOpArchiveStepName              flows.StepName = "initOpArchive"
	UpdateOpArchiveStepName            flows.StepName = "updateOpArchive"
	InitQTStateStepName                flows.StepName = "initQTState"
	DisableSafeModeStepName            flows.StepName = "disableSafeMode"
)

// this is temporary interface.
type ytsaurusClient interface {
	component
	GetYtClient() yt.Client
	HandlePossibilityCheck(context.Context) (bool, string, error)
	EnableSafeMode(context.Context) error
	DisableSafeMode(context.Context) error

	SaveTableCells(context.Context) error
	RemoveTableCells(context.Context) error
	RecoverTableCells(context.Context) error
	AreTabletCellsRemoved(context.Context) (bool, error)
	AreTabletCellsRecovered(context.Context) (bool, error)

	SaveMasterMonitoringPaths(context.Context) error
	StartBuildingMasterSnapshots(context.Context) error
	AreMasterSnapshotsBuilt(context.Context) (bool, error)

	ClearUpdateStatus(context.Context) error
}

//func getFullUpdateStatus(ctx context.Context, yc components.YtsaurusClient) (flows.StepStatus, error) {
//	required, updateReason, err := isFullUpdateRequired(state.getMasterStatus())
//	if err != nil {
//		return flows.StepStatus{}, err
//	}
//	if !required {
//		return flows.StepStatus{StepSyncStatusSkip, updateReason}, nil
//	}
//
//	// NB: here we expect YTsaurus cluster to be running to yt client to work.
//	// TODO: how to check that properly
//	var impossibilityReason string
//	possible, impossibilityReason, err := yc.HandlePossibilityCheck(ctx)
//	msg := updateReason + ": " + impossibilityReason
//	if err != nil {
//		return flows.StepStatus{}, err
//	}
//	if !possible {
//		return flows.StepStatus{StepSyncStatusBlocked, msg}, nil
//	}
//	return flows.StepStatus{StepSyncStatusNeedRun, msg}, nil
//}

func checkFullUpdatePossibility(yc ytsaurusClient) flows.ActionStep {
	preRun := func(ctx context.Context) (flows.StepStatus, error) {
		possible, msg, err := yc.HandlePossibilityCheck(ctx)
		if err != nil {
			return flows.StepStatus{}, err
		}
		if !possible {
			return flows.StepStatus{
				SyncStatus: flows.StepSyncStatusBlocked,
				Message:    msg,
			}, nil
		}
		return flows.StepStatus{
			SyncStatus: flows.StepSyncStatusNeedRun,
			Message:    msg,
		}, nil
	}
	return flows.ActionStep{
		Name:       CheckFullUpdatePossibilityStepName,
		PreRunFunc: preRun,
	}
}

func enableSafeMode(yc ytsaurusClient) flows.ActionStep {
	return flows.ActionStep{
		Name:    EnableSafeModeStepName,
		RunFunc: yc.EnableSafeMode,
	}
}

func backupTabletCells(yc ytsaurusClient) flows.ActionStep {
	preRun := func(ctx context.Context) (flows.StepStatus, error) {
		if err := yc.SaveTableCells(ctx); err != nil {
			return flows.StepStatus{}, err
		}
		return flows.StepStatus{
			SyncStatus: flows.StepSyncStatusNeedRun,
			Message:    "tablet cell bundles are stored in the resource state",
		}, nil
	}
	run := yc.RemoveTableCells
	postRun := func(ctx context.Context) (flows.StepStatus, error) {
		done, err := yc.AreTabletCellsRemoved(ctx)
		if err != nil {
			return flows.StepStatus{}, err
		}
		if done {
			return flows.StepStatus{
				SyncStatus: flows.StepSyncStatusDone,
				Message:    "tablet cells were successfully removed",
			}, nil
		}
		return flows.StepStatus{
			SyncStatus: flows.StepSyncStatusUpdating,
			Message:    "tablet cells not have been removed yet",
		}, nil
	}

	return flows.ActionStep{
		Name:        BackupTabletCellsStepName,
		PreRunFunc:  preRun,
		RunFunc:     run,
		PostRunFunc: postRun,
	}
}

func buildMasterSnapshots(yc ytsaurusClient) flows.ActionStep {
	preRun := func(ctx context.Context) (flows.StepStatus, error) {
		if err := yc.SaveMasterMonitoringPaths(ctx); err != nil {
			return flows.StepStatus{}, err
		}
		return flows.StepStatus{
			SyncStatus: flows.StepSyncStatusNeedRun,
			Message:    "master monitor paths were saved in state",
		}, nil
	}
	run := yc.StartBuildingMasterSnapshots
	postRun := func(ctx context.Context) (flows.StepStatus, error) {
		done, err := yc.AreMasterSnapshotsBuilt(ctx)
		if err != nil {
			return flows.StepStatus{}, err
		}
		if done {
			return flows.StepStatus{
				SyncStatus: flows.StepSyncStatusDone,
				Message:    "master snapshots were successfully built",
			}, nil
		}
		return flows.StepStatus{
			SyncStatus: flows.StepSyncStatusUpdating,
			Message:    "master snapshots haven't been not removed yet",
		}, nil
	}

	return flows.ActionStep{
		Name:        BuildMasterSnapshotsStepName,
		PreRunFunc:  preRun,
		RunFunc:     run,
		PostRunFunc: postRun,
	}
}

func masterExitReadOnly(job *components.JobStateless) flows.ActionStep {
	return newJobStep(
		MasterExitReadOnlyStepName,
		job,
		components.CreateExitReadOnlyScript(),
	)
}

func recoverTabletCells(yc ytsaurusClient) flows.ActionStep {
	return flows.ActionStep{
		Name:    RecoverTabletCellsStepName,
		RunFunc: yc.RecoverTableCells,
	}
}

func initOpArchive(job *components.JobStateless, scheduler *components.Scheduler) flows.ActionStep {
	if !scheduler.NeedOpArchiveInit() {
		// no-op step
		return flows.ActionStep{Name: InitOpArchiveStepName}
	}
	return newJobStep(
		InitOpArchiveStepName,
		job,
		scheduler.GetInitOpArchiveScript(),
	)
}

func updateOpArchive(job *components.JobStateless, scheduler *components.Scheduler) flows.ActionStep {
	// this wrapper is lousy
	jobStep := newJobStep(
		UpdateOpArchiveStepName,
		job,
		scheduler.GetUpdateOpArchiveScript(),
	)
	run := func(ctx context.Context) error {
		job.SetInitScript(scheduler.GetUpdateOpArchiveScript())
		batchJob := job.Build()
		container := &batchJob.Spec.Template.Spec.Containers[0]
		container.EnvFrom = []corev1.EnvFromSource{scheduler.GetSecretEnv()}
		return job.Sync(ctx)
	}
	return flows.ActionStep{
		Name:        UpdateOpArchiveStepName,
		PreRunFunc:  jobStep.PreRunFunc,
		RunFunc:     run,
		PostRunFunc: jobStep.PostRunFunc,
	}
}

func initQueryTracker(job *components.JobStateless, queryTracker *components.QueryTracker) flows.ActionStep {
	// this wrapper is lousy
	jobStep := newJobStep(
		InitQTStateStepName,
		job,
		queryTracker.GetInitQueryTrackerJobScript(),
	)
	run := func(ctx context.Context) error {
		job.SetInitScript(queryTracker.GetInitQueryTrackerJobScript())
		batchJob := job.Build()
		container := &batchJob.Spec.Template.Spec.Containers[0]
		container.EnvFrom = []corev1.EnvFromSource{queryTracker.GetSecretEnv()}
		return job.Sync(ctx)
	}
	return flows.ActionStep{
		Name:        InitQTStateStepName,
		PreRunFunc:  jobStep.PreRunFunc,
		RunFunc:     run,
		PostRunFunc: jobStep.PostRunFunc,
	}
}

func disableSafeMode(yc ytsaurusClient) flows.ActionStep {
	return flows.ActionStep{
		Name:    DisableSafeModeStepName,
		RunFunc: yc.DisableSafeMode,
	}
}
