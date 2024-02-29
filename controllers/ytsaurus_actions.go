package controllers

import (
	"context"

	"go.ytsaurus.tech/yt/go/yt"

	"github.com/ytsaurus/yt-k8s-operator/pkg/flows"
)

// not sure if we need flows.StepType names
const (
	EnableSafeModeStepName       flows.StepName = "enableSafeMode"
	SaveTabletCellsStepName      flows.StepName = "saveTabletCells"
	RemoveTabletCellsStepName    flows.StepName = "removeTabletCells"
	BuildMasterSnapshotsStepName flows.StepName = "buildMasterSnapshots"
	masterExitReadOnlyStepName   flows.StepName = "masterExitReadOnly"
	RecoverTabletCellsStepName   flows.StepName = "recoverTabletCells"
	updateOpArchiveStepName      flows.StepName = "updateOpArchive"
	updateQTStateStepName        flows.StepName = "updateQTState"
	DisableSafeModeStepName      flows.StepName = "disableSafeMode"
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
	statusCheck := func(ctx context.Context) (flows.StepStatus, error) {
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
	return flows.NewOperationStep(
		RemoveTabletCellsStepName,
		statusCheck,
		func(ctx context.Context) error { return nil },
	)
}

func enableSafeMode(yc ytsaurusClient) flows.StepType {
	return flows.NewActionStep(
		EnableSafeModeStepName,
		yc.EnableSafeMode,
	)
}
func saveTabletCells(yc ytsaurusClient) flows.StepType {
	return flows.NewActionStep(
		SaveTabletCellsStepName,
		yc.SaveTableCells,
	)
}

func removeTabletCells(yc ytsaurusClient) flows.StepType {
	statusCheck := func(ctx context.Context) (flows.StepStatus, error) {
		done, err := yc.AreTabletCellsRemoved(ctx)
		if err != nil {
			return flows.StepStatus{}, err
		}
		if !done {
			return flows.StepStatus{
				SyncStatus: flows.StepSyncStatusNeedRun,
				Message:    "Tablet cells are not removed",
			}, nil
		}
		return flows.StepStatus{
			SyncStatus: flows.StepSyncStatusDone,
			Message:    "Tablet cells were removed",
		}, nil
	}
	return flows.NewOperationStep(
		RemoveTabletCellsStepName,
		statusCheck,
		yc.RemoveTableCells,
	)
}

func buildMasterSnapshots(yc ytsaurusClient) flows.StepType {
	action := func(ctx context.Context) error {
		if err := yc.SaveMasterMonitoringPaths(ctx); err != nil {
			return err
		}
		return yc.StartBuildingMasterSnapshots(ctx)
	}
	statusCheck := func(ctx context.Context) (flows.StepStatus, error) {
		done, err := yc.AreMasterSnapshotsBuilt(ctx)
		if err != nil {
			return flows.StepStatus{}, err
		}
		if !done {
			return flows.StepStatus{
				SyncStatus: flows.StepSyncStatusUpdating,
				Message:    "Snapshots are not built",
			}, nil
		}
		return flows.StepStatus{
			SyncStatus: flows.StepSyncStatusDone,
			Message:    "Snapshots are built",
		}, nil
	}
	return flows.NewOperationStep(
		BuildMasterSnapshotsStepName,
		statusCheck,
		action,
	)
}

//	func saveMasterMonitoringPaths(yc ytsaurusClient) flows.StepType {
//		action := func(ctx context.Context) error {
//			return yc.SaveMasterMonitoringPaths(ctx)
//		}
//		return newActionStep(
//			saveMasterSnapshotMonitoringStepName,
//			action,
//		)
//	}
//
// // FIXME: it is better not to start multiple snapshot building operations, so we use two steps here.
// // though we have allMastersReadOnly check and maybe it could be one step.
//
//	func startBuildingMasterSnapshots(yc ytsaurusClient) flows.StepType {
//		action := func(ctx context.Context) error {
//			return yc.StartBuildingMasterSnapshots(ctx)
//		}
//		return newActionStep(
//			startBuildingMasterSnapshotsStepName,
//			action,
//		)
//	}
//
//	func finishBuildingMasterSnapshots(yc ytsaurusClient) flows.StepType {
//		action := func(ctx context.Context) error {
//			built, err := yc.AreMasterSnapshotsBuilt(ctx)
//			if err != nil {
//				return err
//			}
//			if !built {
//				return nil
//			}
//			return nil
//		}
//		return newActionStep(
//			finishBuildingMasterSnapshotsStepName,
//			action,
//		)
//	}
//
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
	return flows.NewActionStep(
		RecoverTabletCellsStepName,
		yc.RecoverTableCells,
	)
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
	return flows.NewActionStep(
		DisableSafeModeStepName,
		yc.DisableSafeMode,
	)
}
