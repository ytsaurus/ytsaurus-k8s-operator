package controllers

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type Step interface {
	GetName() StepName
	Status(ctx context.Context, state *ytsaurusState) (StepStatus, error)
	Run(ctx context.Context, state *ytsaurusState) error
}

type YtsaurusSteps struct {
	steps []Step
	//ytsaurusProxy *apiProxy.Ytsaurus
	state *ytsaurusState
}

const (
	enableSafeModeStepName                StepName = "enableSafeMode"
	saveTabletCellsStepName               StepName = "saveTabletCells"
	removeTabletCellsStepName             StepName = "removeTabletCells"
	saveMasterSnapshotMonitoringStepName  StepName = "saveMasterSnapshotMonitoring"
	startBuildingMasterSnapshotsStepName  StepName = "startBuildingMasterSnapshots"
	finishBuildingMasterSnapshotsStepName StepName = "finishBuildingMasterSnapshots"
	masterExitReadOnlyStepName            StepName = "masterExitReadOnly"
	recoverTableCellsStepName             StepName = "recoverTableCells"
	updateOpArchiveStepName               StepName = "updateOpArchive"
	updateQTStateStepName                 StepName = "updateQTState"
	disableSafeModeStepName               StepName = "disableSafeMode"
)

func NewYtsaurusSteps(comps componentsStore, ytsaurusStatus *ytv1.YtsaurusStatus, apiproxy *apiProxy.Ytsaurus) (*YtsaurusSteps, error) {
	discoveryStep := newComponentStep(comps.discovery)
	masterStep := newComponentStep(comps.master)
	var httpProxiesSteps []Step
	for _, hp := range comps.httpProxies {
		httpProxiesSteps = append(httpProxiesSteps, newComponentStep(hp))
	}
	yc := comps.ytClient.(components.YtsaurusClient2)
	ytsaurusClientStep := newComponentStep(comps.ytClient)
	var dataNodesSteps []Step
	for _, dn := range comps.dataNodes {
		dataNodesSteps = append(dataNodesSteps, newComponentStep(dn))
	}

	steps := concat(
		// it seems ytsaurusClientStep needs to be synced first because
		// it is needed for action steps, but it can't work until its Sync not called
		// (secret needs to be created)
		ytsaurusClientStep,
		enableSafeMode(yc),
		saveTabletCells(yc),
		removeTabletCells(yc),
		saveMasterMonitoringPaths(yc),
		startBuildingMasterSnapshots(yc),
		finishBuildingMasterSnapshots(yc),
		discoveryStep,
		masterStep,
		httpProxiesSteps,
		dataNodesSteps,
		// (optional) ui (depends on master)
		// (optional) rpcproxies (depends on master)
		// (optional) tcpproxies (depends on master)
		// (optional) execnodes (depends on master)
		// (optional) tabletnodes (depends on master, yt client)
		// (optional) scheduler (depends on master, exec nodes, tablet nodes)
		// (optional) controller agents (depends on master)
		// (optional) querytrackers (depends on yt client and tablet nodes)
		// (optional) queueagents (depend on y cli, master, tablet nodes)
		// (optional) yqlagents (depend on master)
		// (optional) strawberry (depend on master, scheduler, data nodes)
		//masterExitReadOnly(yc, comps.master),
		//recoverTableCells(yc),
		//updateOpArchive(),
		//updateQTState(),
		disableSafeMode(yc),
	)

	state := newYtsaurusState(comps, ytsaurusStatus)

	return &YtsaurusSteps{
		//ytsaurusProxy: apiproxy,
		steps: steps,
		state: state,
	}, nil
}

func (s *YtsaurusSteps) Sync(ctx context.Context) (StepSyncStatus, error) {
	logger := log.FromContext(ctx)
	step, status, err := s.getNextStep(ctx)
	if err != nil {
		return "", err
	}
	if step == nil {
		return StepSyncStatusDone, nil
	}

	stepSyncStatus := status.SyncStatus
	switch stepSyncStatus {
	case StepSyncStatusUpdating:
		return StepSyncStatusUpdating, nil
	case StepSyncStatusBlocked:
		return StepSyncStatusBlocked, nil
	case StepSyncStatusNeedRun:
		err = step.Run(ctx, s.state)
		logger.Info(fmt.Sprintf("finish %s step execution", step.GetName()))
		return StepSyncStatusUpdating, err
	default:
		return "", errors.New("unexpected step sync status: " + string(stepSyncStatus))
	}
}

func (s *YtsaurusSteps) getNextStep(ctx context.Context) (Step, StepStatus, error) {
	logger := log.FromContext(ctx)
	execStat := newExecutionStats(s.steps)
	if err := s.state.Build(ctx); err != nil {
		return nil, StepStatus{}, err
	}

	for _, step := range s.steps {
		status, err := step.Status(ctx, s.state)
		execStat.Collect(step.GetName(), status)

		stepName := string(step.GetName())
		if err != nil {
			return nil, StepStatus{}, fmt.Errorf("failed to get status for step `%s`: %w", stepName, err)
		}

		stepSyncStatus := status.SyncStatus
		switch stepSyncStatus {
		case StepSyncStatusDone:
			continue
		case StepSyncStatusSkip:
			continue
		default:
			for _, line := range execStat.AsLines() {
				logger.Info(line)
			}
			return step, status, nil
		}
	}

	for _, line := range execStat.AsLines() {
		logger.Info(line)
	}
	return nil, StepStatus{}, nil
}

func statusToIcon(status StepSyncStatus) string {
	return map[StepSyncStatus]string{
		StepSyncStatusDone:     "[v]",
		StepSyncStatusSkip:     "[-]",
		StepSyncStatusUpdating: "[.]",
		StepSyncStatusBlocked:  "[x]",
		StepSyncStatusNeedRun:  "[ ]",
	}[status]
}

func isFullUpdateRequired(masterStatus components.ComponentStatus) (bool, string, error) {
	// FIXME: should we support that really?
	// if data node status is syncNeedRecreate
	//    return true
	// if tablet node status is syncNeedRecreate
	//    return true
	if masterStatus.SyncStatus == components.SyncStatusNeedFullUpdate {
		return true, "master needs recreating", nil
	}
	return false, "master doesn't need recreating", nil
}

func getFullUpdateStatus(ctx context.Context, yc components.YtsaurusClient2, state *ytsaurusState) (StepStatus, error) {
	required, updateReason, err := isFullUpdateRequired(state.getMasterStatus())
	if err != nil {
		return StepStatus{}, err
	}
	if !required {
		return StepStatus{StepSyncStatusSkip, updateReason}, nil
	}

	// NB: here we expect YTsaurus cluster to be running to yt client to work.
	// TODO: how to check that properly
	var impossibilityReason string
	possible, impossibilityReason, err := yc.HandlePossibilityCheck(ctx)
	msg := updateReason + ": " + impossibilityReason
	if err != nil {
		return StepStatus{}, err
	}
	if !possible {
		return StepStatus{StepSyncStatusBlocked, msg}, nil
	}
	return StepStatus{StepSyncStatusNeedRun, msg}, nil

}

func enableSafeMode(yc components.YtsaurusClient2) Step {
	statusCheck := func(ctx context.Context, state *ytsaurusState) (StepStatus, error) {
		return getFullUpdateStatus(ctx, yc, state)
	}
	action := func(ctx context.Context, state *ytsaurusState) error {
		if err := yc.EnableSafeMode(ctx); err != nil {
			return err
		}
		state.SetUpdateStatusCondition(SafeModeEnabledCondition)
		return nil
	}
	return newActionStepWithDoneCondition(
		enableSafeModeStepName,
		action,
		statusCheck,
		SafeModeEnabledCondition.Type,
	)
}
func saveTabletCells(yc components.YtsaurusClient2) Step {
	statusCheck := func(ctx context.Context, state *ytsaurusState) (StepStatus, error) {
		return getFullUpdateStatus(ctx, yc, state)
	}
	action := func(ctx context.Context, state *ytsaurusState) error {
		if err := yc.SaveTableCells(ctx); err != nil {
			return err
		}
		state.SetUpdateStatusCondition(TabletCellsSavedCondition)
		return nil
	}
	return newActionStepWithDoneCondition(
		saveTabletCellsStepName,
		action,
		statusCheck,
		TabletCellsSavedCondition.Type,
	)
}
func removeTabletCells(yc components.YtsaurusClient2) Step {
	statusCheck := func(ctx context.Context, state *ytsaurusState) (StepStatus, error) {
		return getFullUpdateStatus(ctx, yc, state)
	}
	// FIXME: do we need condition for that operation, i suppose not,
	// because removing nodes is idempotent and we don't expect races with operator loops.
	action := func(ctx context.Context, state *ytsaurusState) error {
		return yc.RemoveTableCells(ctx)
	}
	doneCheck := func(ctx context.Context, state *ytsaurusState) (bool, error) {
		return yc.AreTabletCellsRemoved(ctx)
	}
	return newActionStep(removeTabletCellsStepName, action, statusCheck, doneCheck)
}
func saveMasterMonitoringPaths(yc components.YtsaurusClient2) Step {
	statusCheck := func(ctx context.Context, state *ytsaurusState) (StepStatus, error) {
		return getFullUpdateStatus(ctx, yc, state)
	}
	action := func(ctx context.Context, state *ytsaurusState) error {
		if err := yc.SaveMasterMonitoringPaths(ctx); err != nil {
			return err
		}
		state.SetUpdateStatusCondition(SnapshotsMonitoringInfoSavedCondition)
		return nil
	}
	return newActionStepWithDoneCondition(
		saveMasterSnapshotMonitoringStepName,
		action,
		statusCheck,
		SnapshotsMonitoringInfoSavedCondition.Type,
	)
}

// FIXME: it is better not to start multiple snapshot building operations, so we use two steps here.
// though we have allMastersReadOnly check and maybe it could be one step.
func startBuildingMasterSnapshots(yc components.YtsaurusClient2) Step {
	statusCheck := func(ctx context.Context, state *ytsaurusState) (StepStatus, error) {
		return getFullUpdateStatus(ctx, yc, state)
	}
	action := func(ctx context.Context, state *ytsaurusState) error {
		if err := yc.StartBuildingMasterSnapshots(ctx); err != nil {
			return err
		}
		state.SetUpdateStatusCondition(SnapshotsBuildingStartedCondition)
		return nil
	}
	return newActionStepWithDoneCondition(
		startBuildingMasterSnapshotsStepName,
		action,
		statusCheck,
		SnapshotsBuildingStartedCondition.Type,
	)
}
func finishBuildingMasterSnapshots(yc components.YtsaurusClient2) Step {
	statusCheck := func(ctx context.Context, state *ytsaurusState) (StepStatus, error) {
		return getFullUpdateStatus(ctx, yc, state)
	}
	action := func(ctx context.Context, state *ytsaurusState) error {
		state.SetUpdateStatusCondition(MasterSnapshotsBuiltCondition)
		return nil
	}
	doneCheck := func(ctx context.Context, _ *ytsaurusState) (bool, error) {
		return yc.AreMasterSnapshotsBuilt(ctx)
	}
	return newActionStep(
		finishBuildingMasterSnapshotsStepName,
		action,
		statusCheck,
		doneCheck,
	)
}

//func masterExitReadOnly(yc components.YtsaurusClient2, master components.Component2) Step {
//	statusCheck := func(ctx context.Context, _ *ytsaurusState) (StepStatus, error) {
//		isReadOnly, err := yc.IsMasterReadOnly(ctx)
//		if err != nil {
//			return StepStatus{}, err
//		}
//		if !isReadOnly {
//			return StepStatus{StepSyncStatusDone, ""}, nil
//		}
//		return StepStatus{StepSyncStatusNeedRun, ""}, nil
//	}
//	action := func(ctx context.Context) error {
//		masterImpl := master.(*components.Master)
//		return masterImpl.DoExitReadOnly(ctx)
//	}
//	return newActionStep(masterExitReadOnlyStepName, action, statusCheck)
//}
//func recoverTableCells(yc components.YtsaurusClient2) Step {
//	action := func(ctx context.Context) error {
//		return yc.RecoverTableCells(ctx)
//	}
//	statusCheck := func(ctx context.Context, _ *ytsaurusState) (StepStatus, error) {
//		done, err := yc.AreTabletCellsRecovered(ctx)
//		if err != nil {
//			return StepStatus{}, err
//		}
//		if done {
//			return StepStatus{StepSyncStatusDone, ""}, nil
//		}
//		return StepStatus{StepSyncStatusNeedRun, ""}, nil
//	}
//	return newActionStep(recoverTableCellsStepName, action, statusCheck)
//}

// maybe prepare is needed also?
//func updateOpArchive() Step {
//	action := func(context.Context) error {
//		// maybe we can use scheduler component here
//		// run job
//		return nil
//	}
//	statusCheck := func(ctx context.Context, _ *ytsaurusState) (StepStatus, error) {
//		// maybe some //sys/cluster_nodes/@config value?
//		// check script and understand how to check if archive is inited
//		return StepStatus{}, nil
//	}
//	return newActionStep(updateOpArchiveStepName, action, statusCheck)
//}
//func updateQTState() Step {
//	action := func(context.Context) error {
//		// maybe we can use queryTracker component here
//		// run job
//		return nil
//	}
//	statusCheck := func(ctx context.Context, _ *ytsaurusState) (StepStatus, error) {
//		// maybe some //sys/cluster_nodes/@config value?
//		// check /usr/bin/init_query_tracker_state script and understand how to check if qt state is set
//		return StepStatus{}, nil
//	}
//	return newActionStep(updateQTStateStepName, action, statusCheck)
//}

//	func disableSafeMode(yc components.YtsaurusClient2) Step {
//		action := func(ctx context.Context) error {
//			return yc.DisableSafeMode(ctx)
//		}
//		statusCheck := func(ctx context.Context, _ *ytsaurusState) (StepStatus, error) {
//			enabled, err := yc.IsSafeModeEnabled(ctx)
//			if err != nil {
//				return StepStatus{}, err
//			}
//			if !enabled {
//				return StepStatus{StepSyncStatusDone, ""}, nil
//			}
//			return StepStatus{StepSyncStatusNeedRun, ""}, nil
//		}
//		return newActionStep(disableSafeModeStepName, action, statusCheck)
//	}

// I think disableSafeMode could and should be implemented without conditions
// (with just checking cypress node), but let's think about in the next refactoring.
func disableSafeMode(yc components.YtsaurusClient2) Step {
	statusCheck := func(ctx context.Context, state *ytsaurusState) (StepStatus, error) {
		if !state.isUpdateStatusConditionTrue(SafeModeEnabledCondition.Type) {
			return StepStatus{
				SyncStatus: StepSyncStatusSkip,
				Message:    "safe mode wasn't enabled",
			}, nil
		}
		return StepStatus{StepSyncStatusNeedRun, ""}, nil
	}
	action := func(ctx context.Context, state *ytsaurusState) error {
		if err := yc.DisableSafeMode(ctx); err != nil {
			return err
		}
		// state.SetUpdateStatusCondition(SafeModeDisabledCondition)
		// I suppose this should be in ytsaurusState, but it needs ytsaurus proxy dependency.
		return yc.ClearUpdateStatus(ctx)
	}
	return newActionStepWithDoneCondition(
		disableSafeModeStepName,
		action,
		statusCheck,
		SafeModeDisabledCondition.Type,
	)
}

func concat(items ...interface{}) []Step {
	var result []Step
	for _, item := range items {
		if reflect.TypeOf(item).Kind() == reflect.Slice {
			result = append(result, item.([]Step)...)
			continue
		}
		if value, ok := item.(Step); ok {
			result = append(result, value)
			continue
		}
		panic("concat expect only Step or []Step in arguments")
	}
	return result
}
