package controllers

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/log"

	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type Step interface {
	GetName() StepName
	Status(ctx context.Context, execStats executionStats) (StepStatus, error)
	Run(ctx context.Context) error
}

type YtsaurusSteps struct {
	steps         []Step
	ytsaurusProxy *apiProxy.Ytsaurus
}

const (
	enableSafeModeStepName       StepName = "enableSafeMode"
	saveTabletCellsStepName      StepName = "saveTabletCells"
	removeTabletCellsStepName    StepName = "removeTabletCells"
	buildMasterSnapshotsStepName StepName = "buildMasterSnapshots"
	masterExitReadOnlyStepName   StepName = "masterExitReadOnly"
	recoverTableCellsStepName    StepName = "recoverTableCells"
	updateOpArchiveStepName      StepName = "updateOpArchive"
	updateQTStateStepName        StepName = "updateQTState"
	disableSafeModeStepName      StepName = "disableSafeMode"
)

func NewYtsaurusSteps(ctx context.Context, ytsaurusProxy *apiProxy.Ytsaurus) (*YtsaurusSteps, error) {
	componentManager, err := NewComponentManager(ytsaurusProxy)
	if err != nil {
		return nil, err
	}
	for _, c := range componentManager.allComponents {
		err = c.Fetch(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch status for %s: %w", c.GetName(), err)
		}
	}
	comps := componentManager.allStructured

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
		enableSafeMode(yc, comps.master),
		saveTabletCells(yc, comps.master),
		removeTabletCells(yc, comps.master),
		buildMasterSnapshots(yc, comps.master),
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
		masterExitReadOnly(yc, comps.master),
		recoverTableCells(yc),
		//updateOpArchive(),
		//updateQTState(),
		disableSafeMode(yc),
	)
	return &YtsaurusSteps{
		ytsaurusProxy: ytsaurusProxy,
		steps:         steps,
	}, nil
}

func (s *YtsaurusSteps) Sync(ctx context.Context) (StepSyncStatus, error) {
	logger := log.FromContext(ctx)
	execStat := newExecutionStats()

	for _, step := range s.steps {
		status, err := step.Status(ctx, execStat)
		execStat.Collect(step.GetName(), status)

		stepName := string(step.GetName())
		if err != nil {
			return "", fmt.Errorf("failed to get status for step `%s`: %w", stepName, err)
		}

		logMsg := statusToIcon(status.SyncStatus) + " " + stepName
		if status.Message != "" {
			logMsg += ": " + status.Message
		}
		logger.Info(logMsg)

		stepSyncStatus := status.SyncStatus
		switch stepSyncStatus {
		case StepSyncStatusDone:
			continue
		case StepSyncStatusSkip:
			continue
		case StepSyncStatusUpdating:
			return StepSyncStatusUpdating, nil
		case StepSyncStatusBlocked:
			return StepSyncStatusBlocked, nil
		case StepSyncStatusNeedRun:
			err = step.Run(ctx)
			logger.Info("finish " + stepName + " execution")
			return StepSyncStatusUpdating, err
		default:
			return "", errors.New("unexpected step sync status: " + string(stepSyncStatus))
		}
	}

	return StepSyncStatusDone, nil
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

func isFullUpdateRequired(ctx context.Context, master components.Component2) (bool, string, error) {
	masterStatus, err := master.Status2(ctx)
	// FIXME: should we support that really?
	// if data node status is syncNeedRecreate
	//    return true
	// if tablet node status is syncNeedRecreate
	//    return true
	if err != nil {
		return false, "", err
	}
	if masterStatus.SyncStatus == components.SyncStatusNeedFullUpdate {
		return true, "master needs recreating", nil
	}
	return false, "master doesn't need recreating", nil
}

func getFullUpdateStatus(ctx context.Context, yc components.YtsaurusClient2, master components.Component2) (StepStatus, error) {
	required, updateReason, err := isFullUpdateRequired(ctx, master)
	if err != nil {
		return StepStatus{}, err
	}
	if !required {
		return StepStatus{StepSyncStatusSkip, updateReason}, nil
	}
	possible, possibilityReason, err := yc.HandlePossibilityCheck(ctx)
	msg := updateReason + ": " + possibilityReason
	if err != nil {
		return StepStatus{}, err
	}
	if !possible {
		return StepStatus{StepSyncStatusBlocked, msg}, nil
	}
	return StepStatus{StepSyncStatusNeedRun, msg}, nil

}

func enableSafeMode(yc components.YtsaurusClient2, master components.Component2) Step {
	action := func(ctx context.Context) error {
		return yc.EnableSafeMode(ctx)
	}
	statusCheck := func(ctx context.Context, _ executionStats) (StepStatus, error) {
		status, err := getFullUpdateStatus(ctx, yc, master)
		if err != nil {
			return StepStatus{}, err
		}
		if status.SyncStatus != StepSyncStatusNeedRun {
			return status, nil
		}
		done, err := yc.IsSafeModeEnabled(ctx)
		if err != nil {
			return StepStatus{}, err
		}
		if done {
			return StepStatus{StepSyncStatusDone, status.Message}, nil
		}
		return StepStatus{StepSyncStatusNeedRun, status.Message}, nil
	}
	return newActionStep(enableSafeModeStepName, action, statusCheck)
}
func saveTabletCells(yc components.YtsaurusClient2, master components.Component2) Step {
	action := func(ctx context.Context) error {
		return yc.SaveTableCellsAndUpdateState(ctx)
	}
	statusCheck := func(ctx context.Context, execStat executionStats) (StepStatus, error) {
		if execStat.isSkipped(enableSafeModeStepName) {
			// This is done to prevent situation when state of the system is changed between steps,
			// so it is impossible to launch save/remove TabletCells, buildMasterSnapshots without safe mode.
			return StepStatus{
				StepSyncStatusSkip,
				fmt.Sprintf("mustn't run if %s is skipped", enableSafeModeStepName),
			}, nil
		}
		status, err := getFullUpdateStatus(ctx, yc, master)
		if err != nil {
			return StepStatus{}, err
		}
		if status.SyncStatus != StepSyncStatusNeedRun {
			return status, nil
		}
		done := yc.IsTableCellsSaved()
		if done {
			return StepStatus{StepSyncStatusDone, status.Message}, nil
		}
		return StepStatus{StepSyncStatusNeedRun, status.Message}, nil
	}
	return newActionStep(saveTabletCellsStepName, action, statusCheck)
}
func removeTabletCells(yc components.YtsaurusClient2, master components.Component2) Step {
	action := func(ctx context.Context) error {
		return yc.RemoveTableCells(ctx)
	}
	statusCheck := func(ctx context.Context, execStat executionStats) (StepStatus, error) {
		if execStat.isSkipped(saveTabletCellsStepName) {
			return StepStatus{
				StepSyncStatusSkip,
				fmt.Sprintf("mustn't run if %s is skipped", saveTabletCellsStepName),
			}, nil
		}
		status, err := getFullUpdateStatus(ctx, yc, master)
		if err != nil {
			return StepStatus{}, err
		}
		if status.SyncStatus != StepSyncStatusNeedRun {
			return status, nil
		}
		done, err := yc.AreTabletCellsRemoved(ctx)
		if err != nil {
			return StepStatus{}, err
		}
		if done {
			return StepStatus{StepSyncStatusDone, status.Message}, nil
		}
		return StepStatus{StepSyncStatusNeedRun, status.Message}, nil
	}
	return newActionStep(removeTabletCellsStepName, action, statusCheck)
}
func buildMasterSnapshots(yc components.YtsaurusClient2, master components.Component2) Step {
	action := func(ctx context.Context) error {
		return yc.StartBuildMasterSnapshots(ctx)
	}
	areSnapshotsBuild := func() (bool, error) {
		// TODO: is there a way to check if snapshots are built
		// without using k8s condition?
		return true, nil
	}
	statusCheck := func(ctx context.Context, execStat executionStats) (StepStatus, error) {
		if execStat.isSkipped(removeTabletCellsStepName) {
			return StepStatus{
				StepSyncStatusSkip,
				fmt.Sprintf("mustn't run if %s is skipped", removeTabletCellsStepName),
			}, nil
		}
		status, err := getFullUpdateStatus(ctx, yc, master)
		if err != nil {
			return StepStatus{}, err
		}
		if status.SyncStatus != StepSyncStatusNeedRun {
			return status, nil
		}
		done, err := areSnapshotsBuild()
		if err != nil {
			return StepStatus{}, err
		}
		if done {
			return StepStatus{StepSyncStatusDone, status.Message}, nil
		}
		return StepStatus{StepSyncStatusNeedRun, status.Message}, nil
	}
	return newActionStep(buildMasterSnapshotsStepName, action, statusCheck)
}

// maybe it shouldn't be inside master at all
func masterExitReadOnly(yc components.YtsaurusClient2, master components.Component2) Step {
	action := func(ctx context.Context) error {
		masterImpl := master.(*components.Master)
		return masterImpl.DoExitReadOnly(ctx)
	}
	statusCheck := func(ctx context.Context, _ executionStats) (StepStatus, error) {
		isReadOnly, err := yc.IsMasterReadOnly(ctx)
		if err != nil {
			return StepStatus{}, err
		}
		if !isReadOnly {
			return StepStatus{StepSyncStatusDone, ""}, nil
		}
		return StepStatus{StepSyncStatusNeedRun, ""}, nil
	}
	return newActionStep(masterExitReadOnlyStepName, action, statusCheck)
}
func recoverTableCells(yc components.YtsaurusClient2) Step {
	action := func(ctx context.Context) error {
		return yc.RecoverTableCells(ctx)
	}
	statusCheck := func(ctx context.Context, _ executionStats) (StepStatus, error) {
		done, err := yc.AreTabletCellsRecovered(ctx)
		if err != nil {
			return StepStatus{}, err
		}
		if done {
			return StepStatus{StepSyncStatusDone, ""}, nil
		}
		return StepStatus{StepSyncStatusNeedRun, ""}, nil
	}
	return newActionStep(recoverTableCellsStepName, action, statusCheck)
}

// maybe prepare is needed also?
func updateOpArchive() Step {
	action := func(context.Context) error {
		// maybe we can use scheduler component here
		// run job
		return nil
	}
	statusCheck := func(ctx context.Context, _ executionStats) (StepStatus, error) {
		// maybe some //sys/cluster_nodes/@config value?
		// check script and understand how to check if archive is inited
		return StepStatus{}, nil
	}
	return newActionStep(updateOpArchiveStepName, action, statusCheck)
}
func updateQTState() Step {
	action := func(context.Context) error {
		// maybe we can use queryTracker component here
		// run job
		return nil
	}
	statusCheck := func(ctx context.Context, _ executionStats) (StepStatus, error) {
		// maybe some //sys/cluster_nodes/@config value?
		// check /usr/bin/init_query_tracker_state script and understand how to check if qt state is set
		return StepStatus{}, nil
	}
	return newActionStep(updateQTStateStepName, action, statusCheck)
}
func disableSafeMode(yc components.YtsaurusClient2) Step {
	action := func(ctx context.Context) error {
		return yc.DisableSafeMode(ctx)
	}
	statusCheck := func(ctx context.Context, _ executionStats) (StepStatus, error) {
		enabled, err := yc.IsSafeModeEnabled(ctx)
		if err != nil {
			return StepStatus{}, err
		}
		if !enabled {
			return StepStatus{StepSyncStatusDone, ""}, nil
		}
		return StepStatus{StepSyncStatusNeedRun, ""}, nil
	}
	return newActionStep(disableSafeModeStepName, action, statusCheck)
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
