package controllers

import (
	"context"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type Step interface {
	GetName() string
	ShouldRun(context.Context) (bool, string, error)
	Run(ctx context.Context) error
	// Status return step status. It is not nice to use ComponentStatus for anything
	// but components, but let us do that for simplicity now and migrate later.
	Status(ctx context.Context) (components.ComponentStatus, error)
	// Done(ctx context.Context) (bool, error)
}

type YtsaurusSteps struct {
	steps         []Step
	ytsaurusProxy *apiProxy.Ytsaurus
}

func NewYtsaurusSteps(ytsaurusProxy *apiProxy.Ytsaurus) (*YtsaurusSteps, error) {
	componentManager, err := NewComponentManager(ytsaurusProxy)
	if err != nil {
		return nil, err
	}
	comps := componentManager.allStructured

	discoveryStep := newComponentStep(comps.discovery)
	masterStep := newComponentStep(comps.master)
	var httpProxiesSteps []Step
	for _, hp := range comps.httpProxies {
		httpProxiesSteps = append(httpProxiesSteps, newComponentStep(hp))
	}
	yc := comps.ytClient.(components.YtsaurusClient)
	ytsaurusClientStep := newComponentStep(comps.ytClient)
	var dataNodesSteps []Step
	for _, dn := range comps.dataNodes {
		dataNodesSteps = append(dataNodesSteps, newComponentStep(dn))
	}

	steps := concat(
		enableSafeMode(yc, comps.master),
		saveTabletCells(yc, comps.master),
		removeTabletCells(yc, comps.master),
		buildMasterSnapshots(yc, comps.master),
		discoveryStep,
		masterStep,
		httpProxiesSteps,
		ytsaurusClientStep,
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
		recoverTableCells(yc, comps.master),
		//updateOpArchive(),
		//updateQTState(),
		disableSafeMode(yc),
	)
	return &YtsaurusSteps{
		ytsaurusProxy: ytsaurusProxy,
		steps:         steps,
	}, nil
}

func (s *YtsaurusSteps) Sync(ctx context.Context) (ytv1.ClusterState, error) {
	logger := log.FromContext(ctx)

	for _, step := range s.steps {
		shouldRun, comment, err := step.ShouldRun(ctx)
		if err != nil {
			return "", err
		}
		if !shouldRun {
			logger.Info(step.GetName() + " step shouldn't run: " + comment)
			continue
		}
		status, err := step.Status(ctx)
		if err != nil {
			return "", err
		}
		if status.IsReady() {
			logger.Info(step.GetName() + " step is ready")
			continue
		} else {
			logger.Info(step.GetName()+" step is NOT ready", "status", status)
		}

		stepSyncStatus := status.SyncStatus
		switch stepSyncStatus {
		case components.SyncStatusBlocked:
			logger.Info(step.GetName()+" step is blocked", "status", status)
			return ytv1.ClusterStateCancelUpdate, nil
		case components.SyncStatusUpdating:
			logger.Info("Waiting for "+step.GetName()+" to finish", "status", status)
			return ytv1.ClusterStateUpdating, nil
		default:
			logger.Info("Going to run step: "+step.GetName(), "status", status)
			err = step.Run(ctx)
			return ytv1.ClusterStateUpdating, err
		}
	}
	return ytv1.ClusterStateRunning, nil
}

func getFullUpdateCondition(yc components.YtsaurusClient, master components.Component2) func(context.Context) (bool, string, error) {
	return func(ctx context.Context) (bool, string, error) {
		if master.Status(ctx).SyncStatus != components.SyncStatusNeedFullUpdate {
			return false, "master doesn't need recreating", nil
		}
		isPossible, msg, err := yc.HandlePossibilityCheck(ctx)
		if err != nil {
			return false, "", err
		}
		if !isPossible {
			return false, msg, nil
		}
		// FIXME: should we support that really?
		// if data node status is syncNeedRecreate
		//    return true
		// if tablet node status is syncNeedRecreate
		//    return true
		return true, "", nil
	}
}

func enableSafeMode(yc components.YtsaurusClient, master components.Component2) Step {
	action := func(ctx context.Context) error {
		return yc.EnableSafeMode(ctx)
	}
	doneCheck := func(ctx context.Context) (bool, error) {
		return yc.IsSafeModeEnabled(ctx)
	}
	runCondition := getFullUpdateCondition(yc, master)
	return newActionStep("enableSafeMode", action, doneCheck).WithRunCondition(runCondition)
}
func saveTabletCells(yc components.YtsaurusClient, master components.Component2) Step {
	action := func(ctx context.Context) error {
		return yc.SaveTableCellsAndUpdateState(ctx)
	}
	doneCheck := func(context.Context) (bool, error) {
		return yc.IsTableCellsSaved(), nil
	}
	runCondition := getFullUpdateCondition(yc, master)
	return newActionStep("saveTabletCells", action, doneCheck).WithRunCondition(runCondition)
}
func removeTabletCells(yc components.YtsaurusClient, master components.Component2) Step {
	action := func(ctx context.Context) error {
		return yc.RemoveTableCells(ctx)
	}
	doneCheck := func(ctx context.Context) (bool, error) {
		return yc.AreTabletCellsRemoved(ctx)
	}
	runCondition := getFullUpdateCondition(yc, master)
	return newActionStep("removeTabletCells", action, doneCheck).WithRunCondition(runCondition)
}
func buildMasterSnapshots(yc components.YtsaurusClient, master components.Component2) Step {
	action := func(context.Context) error {
		// use ytclient code
		// yc.ytClient.RemoveNode
		return nil
	}
	doneCheck := func(context.Context) (bool, error) {
		// check //sys/tablet_cells is empty?
		// like in UpdateStateWaitingForTabletCellsRemoved
		return false, nil
	}
	runCondition := getFullUpdateCondition(yc, master)
	return newActionStep("buildMasterSnapshots", action, doneCheck).WithRunCondition(runCondition)
}

// maybe it shouldn't be inside master at all
func masterExitReadOnly(yc components.YtsaurusClient, master components.Component2) Step {
	action := func(ctx context.Context) error {
		masterImpl := master.(*components.Master)
		return masterImpl.DoExitReadOnly(ctx)
	}
	doneCheck := func(ctx context.Context) (bool, error) {
		return yc.IsMasterReadOnly(ctx)
	}
	return newActionStep("masterExitReadOnly", action, doneCheck)
}
func recoverTableCells(yc components.YtsaurusClient, master components.Component2) Step {
	action := func(ctx context.Context) error {
		return yc.RecoverTableCells(ctx)
	}
	doneCheck := func(ctx context.Context) (bool, error) {
		return yc.AreTabletCellsRecovered(ctx)
	}
	return newActionStep("recoverTableCells", action, doneCheck)
}

// maybe prepare is needed also?
func updateOpArchive() Step {
	action := func(context.Context) error {
		// maybe we can use scheduler component here
		// run job
		return nil
	}
	doneCheck := func(context.Context) (bool, error) {
		// maybe some //sys/cluster_nodes/@config value?
		// check script and understand how to check if archive is inited
		return false, nil
	}
	return newActionStep("updateOpArchive", action, doneCheck)
}
func updateQTState() Step {
	action := func(context.Context) error {
		// maybe we can use queryTracker component here
		// run job
		return nil
	}
	doneCheck := func(context.Context) (bool, error) {
		// maybe some //sys/cluster_nodes/@config value?
		// check /usr/bin/init_query_tracker_state script and understand how to check if qt state is set
		return false, nil
	}
	return newActionStep("updateQTState", action, doneCheck)
}
func disableSafeMode(yc components.YtsaurusClient) Step {
	action := func(ctx context.Context) error {
		return yc.DisableSafeMode(ctx)
	}
	doneCheck := func(ctx context.Context) (bool, error) {
		enabled, err := yc.IsSafeModeEnabled(ctx)
		return !enabled, err
	}
	return newActionStep("disableSafeMode", action, doneCheck)
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
