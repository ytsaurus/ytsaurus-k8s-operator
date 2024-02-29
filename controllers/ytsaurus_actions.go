package controllers

import (
	"context"

	"go.ytsaurus.tech/yt/go/yt"

	"github.com/ytsaurus/yt-k8s-operator/pkg/flows"
)

// not sure if we need flows.StepType names
const (
	IsFullUpdateStepName               flows.StepName = "isFullUpdate"
	CheckFullUpdatePossibilityStepName flows.StepName = "checkFullUpdatePossibility"
	EnableSafeModeStepName             flows.StepName = "enableSafeMode"
	BackupTabletCellsStepName          flows.StepName = "backupTabletCells"
	BuildMasterSnapshotsStepName       flows.StepName = "buildMasterSnapshots"
	masterExitReadOnlyStepName         flows.StepName = "masterExitReadOnly"
	RecoverTabletCellsStepName         flows.StepName = "recoverTabletCells"
	updateOpArchiveStepName            flows.StepName = "updateOpArchive"
	updateQTStateStepName              flows.StepName = "updateQTState"
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

func checkFullUpdatePossibility(yc ytsaurusClient) flows.StepType {
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
			SyncStatus: flows.StepSyncStatusDone,
			Message:    msg,
		}, nil
	}
	return flows.ActionStep{
		Name:       CheckFullUpdatePossibilityStepName,
		PreRunFunc: preRun,
	}
}

func enableSafeMode(yc ytsaurusClient) flows.StepType {
	return flows.ActionStep{
		Name:    EnableSafeModeStepName,
		RunFunc: yc.EnableSafeMode,
	}
}

func backupTabletCells(yc ytsaurusClient) flows.StepType {
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

func buildMasterSnapshots(yc ytsaurusClient) flows.StepType {
	preRun := func(ctx context.Context) (flows.StepStatus, error) {
		if err := yc.SaveMasterMonitoringPaths(ctx); err != nil {
			return flows.StepStatus{}, err
		}
		return flows.StepStatus{
			SyncStatus: flows.StepSyncStatusNeedRun,
			Message:    "master monitor paths were saved in state",
		}, nil
	}
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
		RunFunc:     yc.StartBuildingMasterSnapshots,
		PostRunFunc: postRun,
	}
}

//	func masterExitReadOnly(master component) flows.StepType {
//		statusCheck := func(ctx context.Context) (bool, error) {
//			if state.isUpdateStatusConditionTrue(doneCondition.Type) {
//				return true, nil
//			}
//			masterImpl := master.(*components.Master)
//			done, err := masterImpl.IsExitReadOnlyDone(ctx)
//			if err != nil {
//				return false, err
//			}
//			if !done {
//				return false, nil
//			}
//			state.SetUpdateStatusCondition(doneCondition)
//			return true, nil
//		}
//		action := func(ctx context.Context) error {
//			// TODO: this could be extracted from master
//			masterImpl := master.(*components.Master)
//			err := masterImpl.DoExitReadOnly(ctx)
//			if err != nil {
//				return err
//			}
//			return nil
//		}
//		return flows.NewOperationStep(
//			masterExitReadOnlyStepName,
//			statusCheck,
//			action,
//		)
//	}
func recoverTabletCells(yc ytsaurusClient) flows.StepType {
	return flows.ActionStep{
		Name:    RecoverTabletCellsStepName,
		RunFunc: yc.RecoverTableCells,
	}
}

// maybe prepare is needed also?
//func updateOpArchive() flows.StepType {
//	action := func(context.Context) error {
//		// maybe we can use scheduler component here
//		// run job
//		return nil
//	}
//	statusCheck := func(ctx context.Context, _ *ytsaurusState) (flows.StepStatus, error) {
//		// maybe some //sys/cluster_nodes/@config value?
//		// check script and understand how to check if archive is inited
//		return flows.StepStatus{}, nil
//	}
//	return newActionStep(updateOpArchiveStepName, action, statusCheck)
//}
//func updateQTState() flows.StepType {
//	action := func(context.Context) error {
//		// maybe we can use queryTracker component here
//		// run job
//		return nil
//	}
//	statusCheck := func(ctx context.Context, _ *ytsaurusState) (flows.StepStatus, error) {
//		// maybe some //sys/cluster_nodes/@config value?
//		// check /usr/bin/init_query_tracker_state script and understand how to check if qt state is set
//		return flows.StepStatus{}, nil
//	}
//	return newActionStep(updateQTStateStepName, action, statusCheck)
//}

func disableSafeMode(yc ytsaurusClient) flows.StepType {
	return flows.ActionStep{
		Name:    DisableSafeModeStepName,
		RunFunc: yc.DisableSafeMode,
	}
}
