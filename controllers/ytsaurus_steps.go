package controllers

import (
	"context"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/log"

	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type Step interface {
	GetName() string
	ShouldRun() bool
	Run(ctx context.Context) error
	Done(ctx context.Context) (bool, error)
}

type Ytsaurus struct {
	steps         []Step
	ytsaurusProxy *apiProxy.Ytsaurus
}

func NewYtsaurus(ctx context.Context, ytsaurusProxy *apiProxy.Ytsaurus) (*Ytsaurus, error) {
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
	ytsaurusClientStep := newComponentStep(comps.ytClient)
	var dataNodesSteps []Step
	for _, dn := range comps.dataNodes {
		dataNodesSteps = append(dataNodesSteps, newComponentStep(dn))
	}

	// TODO: not lose enable fullUpdate — it should become blocked status
	steps := concat(
		//enableSafeMode(),
		//saveTabletCells(),
		//removeTabletCells(),
		//buildMasterSnapshots(),
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
		//masterExitReadOnly(),
		//recoverTableCells(),
		//updateOpArchive(),
		//updateQTState(),
		//disableSafeMode(),
	)
	return &Ytsaurus{
		ytsaurusProxy: ytsaurusProxy,
		steps:         steps,
	}, nil
}

func (c *Ytsaurus) Status(ctx context.Context) (components.ComponentStatus, error) {
	logger := log.FromContext(ctx)

	for _, step := range c.steps {
		if !step.ShouldRun() {
			logger.Info(step.GetName() + " step shouldn't run")
			continue
		}
		done, err := step.Done(ctx)
		if err != nil {
			return components.ComponentStatus{}, err
		}
		if done {
			continue
		}

		return components.ComponentStatus{
			SyncStatus: components.SyncStatusPending,
			Message:    step.GetName() + " is not done",
		}, nil
	}
	return components.ComponentStatus{
		SyncStatus: components.SyncStatusReady,
	}, nil
}

func (c *Ytsaurus) Sync(ctx context.Context) error {
	logger := log.FromContext(ctx)

	for _, step := range c.steps {
		if !step.ShouldRun() {
			logger.Info(step.GetName() + " step shouldn't run")
			continue
		}
		done, err := step.Done(ctx)
		if err != nil {
			return err
		}
		if done {
			logger.Info(step.GetName() + " step is already done")
			continue
		}
		logger.Info("Going to run step: " + step.GetName())
		return step.Run(ctx)
	}
	return nil
}

func enableSafeMode() Step {
	action := func(context.Context) error {
		// use ytclient code
		// where we will get ytclient — I suppose it is some common thing
		// we don't exactly call it component maybe?
		// or for simplicity now we can put this method in Ytsaurus component
		return nil
	}
	doneCheck := func(context.Context) (bool, error) {
		// check @enable_safe_mode is set
		return false, nil
	}
	runCondition := func() bool {
		// if master status is syncNeedRecreate
		//    return true
		// if data node status is syncNeedRecreate
		//    return true
		// if tablet node status is syncNeedRecreate
		//    return true
		// if check for update possibility stuff
		//    return true
		return false
	}
	return newActionStep("enableSafeMode", action, doneCheck).WithRunCondition(runCondition)
}
func saveTabletCells() Step {
	action := func(context.Context) error {
		// use ytclient code
		// ytsaurus.GetResource().Status.UpdateStatus.TabletCellBundles = tabletCellBundles
		return nil
	}
	doneCheck := func(context.Context) (bool, error) {
		// check ytsaurus.GetResource().Status.UpdateStatus.TabletCellBundles not empty?
		return false, nil
	}
	runCondition := func() bool {
		// reuse some code from maybeEnableSafeModeStep about master/data/tablet statuses
		return false
	}
	return newActionStep("saveTabletCells", action, doneCheck).WithRunCondition(runCondition)
}
func removeTabletCells() Step {
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
	runCondition := func() bool {
		// reuse some code from maybeEnableSafeModeStep about master/data/tablet statuses
		return false
	}
	return newActionStep("removeTabletCells", action, doneCheck).WithRunCondition(runCondition)
}
func buildMasterSnapshots() Step {
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
	runCondition := func() bool {
		// reuse some code from maybeEnableSafeModeStep about master/data/tablet statuses
		return false
	}
	return newActionStep("buildMasterSnapshots", action, doneCheck).WithRunCondition(runCondition)
}

// maybe it shouldn't be inside master at all
func masterExitReadOnly() Step {
	action := func(context.Context) error {
		// runJob
		return nil
	}
	doneCheck := func(context.Context) (bool, error) {
		// is read only check
		return false, nil
	}
	return newActionStep("masterExitReadOnly", action, doneCheck)
}
func recoverTableCells() Step {
	action := func(context.Context) error {
		// helpers.CreateTabletCells
		// delete status
		return nil
	}
	doneCheck := func(context.Context) (bool, error) {
		// check TabletCellBundles not empty &&  TabletCellBundles != //sys/tablet_cell_bundles ??
		return false, nil
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
func disableSafeMode() Step {
	action := func(context.Context) error {
		// use ytclient code
		return nil
	}
	doneCheck := func(context.Context) (bool, error) {
		// check @enable_safe_mode is false
		return false, nil
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
