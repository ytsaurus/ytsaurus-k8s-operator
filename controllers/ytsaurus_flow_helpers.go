package controllers

import (
	"context"
	"errors"

	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/flows"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func buildComponentRegistry(ytsaurus *apiProxy.Ytsaurus) *componentRegistry {
	registry := newComponentRegistry()

	resource := ytsaurus.GetResource()
	domain := getClusterDomain(ytsaurus.APIProxy().Client())
	cfgen := ytconfig.NewGenerator(resource, domain)
	nodeCfgGen := ytconfig.NewLocalNodeGenerator(ytsaurus.GetResource(), domain)

	registry.add(components.NewDiscovery(cfgen, ytsaurus))
	m := components.NewMaster(cfgen, ytsaurus)
	registry.add(m)

	masterExitReadOnlyJob := components.NewMasterExitReadOnlyJob(cfgen, ytsaurus)
	registry.add(masterExitReadOnlyJob)

	var hps []components.Component
	for _, hpSpec := range ytsaurus.GetResource().Spec.HTTPProxies {
		hp := components.NewHTTPProxy(cfgen, ytsaurus, m, hpSpec)
		hps = append(hps, hp)
		registry.add(hp)
	}
	yc := components.NewYtsaurusClient(cfgen, ytsaurus, hps[0])
	registry.add(yc)

	var dnds []components.Component
	if resource.Spec.DataNodes != nil && len(resource.Spec.DataNodes) > 0 {
		for _, dndSpec := range ytsaurus.GetResource().Spec.DataNodes {
			dnd := components.NewDataNode(nodeCfgGen, ytsaurus, m, dndSpec)
			dnds = append(dnds, dnd)
			registry.add(dnd)
		}
	}

	if resource.Spec.UI != nil {
		ui := components.NewUI(cfgen, ytsaurus, m)
		registry.add(ui)
	}

	if resource.Spec.RPCProxies != nil && len(resource.Spec.RPCProxies) > 0 {
		var rps []components.Component
		for _, rpSpec := range ytsaurus.GetResource().Spec.RPCProxies {
			rp := components.NewRPCProxy(cfgen, ytsaurus, m, rpSpec)
			rps = append(rps, rp)
			registry.add(rp)
		}
	}

	if resource.Spec.TCPProxies != nil && len(resource.Spec.TCPProxies) > 0 {
		var tps []components.Component
		for _, tpSpec := range ytsaurus.GetResource().Spec.TCPProxies {
			tp := components.NewTCPProxy(cfgen, ytsaurus, m, tpSpec)
			tps = append(tps, tp)
			registry.add(tp)
		}
	}

	var ends []components.Component
	if resource.Spec.ExecNodes != nil && len(resource.Spec.ExecNodes) > 0 {
		for _, endSpec := range ytsaurus.GetResource().Spec.ExecNodes {
			end := components.NewExecNode(nodeCfgGen, ytsaurus, m, endSpec)
			ends = append(ends, end)
			registry.add(end)
		}
	}

	var tnds []components.Component
	if resource.Spec.TabletNodes != nil && len(resource.Spec.TabletNodes) > 0 {
		for idx, tndSpec := range ytsaurus.GetResource().Spec.TabletNodes {
			tnd := components.NewTabletNode(nodeCfgGen, ytsaurus, yc, tndSpec, idx == 0)
			tnds = append(tnds, tnd)
			registry.add(tnd)
		}
	}

	var s components.Component
	if resource.Spec.Schedulers != nil {
		s = components.NewScheduler(cfgen, ytsaurus, m, ends, tnds)
		registry.add(s)
		registry.add(components.NewInitOpArchiveJob(cfgen, ytsaurus))
		registry.add(components.NewUpdateOpArchiveJob(cfgen, ytsaurus))
	}

	if resource.Spec.ControllerAgents != nil {
		ca := components.NewControllerAgent(cfgen, ytsaurus, m)
		registry.add(ca)
	}

	var q components.Component
	if resource.Spec.QueryTrackers != nil && resource.Spec.Schedulers != nil && resource.Spec.TabletNodes != nil && len(resource.Spec.TabletNodes) > 0 {
		q = components.NewQueryTracker(cfgen, ytsaurus, yc, tnds)
		registry.add(q)
		registry.add(components.NewInitQueryTrackerJob(cfgen, ytsaurus))
	}

	if resource.Spec.QueueAgents != nil && resource.Spec.TabletNodes != nil && len(resource.Spec.TabletNodes) > 0 {
		qa := components.NewQueueAgent(cfgen, ytsaurus, yc, m, tnds)
		registry.add(qa)
	}

	if resource.Spec.YQLAgents != nil {
		yqla := components.NewYQLAgent(cfgen, ytsaurus, m)
		registry.add(yqla)
	}

	if (resource.Spec.DeprecatedChytController != nil || resource.Spec.StrawberryController != nil) && resource.Spec.Schedulers != nil {
		strawberry := components.NewStrawberryController(cfgen, ytsaurus, m, s, dnds)
		registry.add(strawberry)
	}

	return registry
}

// TODO: split into buildComponents and buildSteps (steps should be visible at the highest level of code)
func buildSteps(
	registry *componentRegistry,
	statuses map[string]components.ComponentStatus,
) ([]flows.StepType, error) {
	discovery, schedulerExists := registry.getByType(consts.ComponentTypeDiscovery)
	if !schedulerExists {
		return nil, errors.New("missing discovery component")
	}
	discoveryStep := newComponentStep(discovery.(statefulComponent), statuses[discovery.GetName()])

	master, schedulerExists := registry.getByName("Master")
	if !schedulerExists {
		return nil, errors.New("missing master component")
	}
	masterStep := newComponentStep(master.(statefulComponent), statuses[master.GetName()])
	masterStatus := statuses["Master"]

	masterExitReadOnlyJob, schedulerExists := registry.getByName("MasterExitReadOnlyJob")
	if !schedulerExists {
		return nil, errors.New("missing MasterExitReadOnlyJob")
	}

	var httpProxiesSteps []flows.StepType
	for _, hp := range registry.getListByType(consts.ComponentTypeHTTPProxy) {
		httpProxiesSteps = append(httpProxiesSteps, newComponentStep(hp.(statefulComponent), statuses[hp.GetName()]))
	}
	yc, schedulerExists := registry.getByType(consts.ComponentTypeClient)
	if !schedulerExists {
		return nil, errors.New("missing client component")
	}
	// temporary
	ycClient := yc.(ytsaurusClient)
	ytsaurusClientStep := newComponentStep(yc.(statefulComponent), statuses[yc.GetName()])

	var dataNodesSteps []flows.StepType
	for _, dn := range registry.getListByType(consts.ComponentTypeDataNode) {
		dataNodesSteps = append(dataNodesSteps, newComponentStep(dn.(statefulComponent), statuses[dn.GetName()]))
	}

	// Scheduler is optional
	scheduler, schedulerExists := registry.getByName("Scheduler")
	var schedulerStep flows.StepType
	var initOpArchiveStep flows.StepType
	var updateOpArchiveStep flows.StepType
	if !schedulerExists {
		schedulerStep = flows.DummyStep{Name: "Scheduler"}
		initOpArchiveStep = flows.DummyStep{Name: InitOpArchiveStepName}
		updateOpArchiveStep = flows.DummyStep{Name: UpdateOpArchiveStepName}
	} else {
		schedulerStep = newComponentStep(scheduler.(statefulComponent), statuses[scheduler.GetName()])
		initJob, ok := registry.getByName("InitOpArchiveJob")
		if !ok {
			return nil, errors.New("missing InitOpArchiveJob")
		}
		initOpArchiveStep = initOpArchive(initJob.(*components.JobStateless), scheduler.(*components.Scheduler))
		updateJob, ok := registry.getByName("UpdateOpArchiveJob")
		if !ok {
			return nil, errors.New("missing UpdateOpArchiveJob")
		}
		updateOpArchiveStep = updateOpArchive(updateJob.(*components.JobStateless), scheduler.(*components.Scheduler))
	}

	// QueryTracker is optional
	queryTracker, qtExists := registry.getByName("QueryTracker")
	var queryTrackerStep flows.StepType
	var initQTStep flows.StepType
	if !qtExists {
		queryTrackerStep = flows.DummyStep{Name: "QueryTracker"}
		initQTStep = flows.DummyStep{Name: InitQTStateStepName}
	} else {
		queryTrackerStep = newComponentStep(queryTracker.(statefulComponent), statuses[queryTracker.GetName()])
		initJob, ok := registry.getByName("InitQueryTrackerJob")
		if !ok {
			return nil, errors.New("missing InitQueryTrackerJob")
		}
		initQTStep = initQueryTracker(initJob.(*components.JobStateless), queryTracker.(*components.QueryTracker))
	}

	isFullUpdateNeeded := func(ctx context.Context) (bool, error) {
		// FIXME: should we support that ?
		// if data node status is syncNeedRecreate
		//    return true
		// if tablet node status is syncNeedRecreate
		//    return true
		return masterStatus.SyncStatus == components.SyncStatusNeedFullUpdate, nil
	}

	componentsUpdateChain, err := flows.FlattenSteps(
		discoveryStep,
		masterStep,
		httpProxiesSteps,
		dataNodesSteps,
		// (optional) ui (depends on master)
		// (optional) rpcproxies (depends on master)
		// (optional) tcpproxies (depends on master)
		// (optional) execnodes (depends on master)
		// (optional) tabletnodes (depends on master, yt client)
		schedulerStep, //(optional) (depends on master, exec nodes, tablet nodes)
		// (optional) controller agents (depends on master)
		queryTrackerStep, // (optional) (depends on yt client and tablet nodes)
		// (optional) queueagents (depend on y cli, master, tablet nodes)
		// (optional) yqlagents (depend on master)
		// (optional) strawberry (depend on master, scheduler, data nodes)
	)
	if err != nil {
		return nil, err
	}

	fullUpdateComponetsChain, err := flows.FlattenSteps(
		checkFullUpdatePossibility(ycClient),
		enableSafeMode(ycClient),
		backupTabletCells(ycClient),
		buildMasterSnapshots(ycClient),
		componentsUpdateChain,
		masterExitReadOnly(masterExitReadOnlyJob.(*components.JobStateless)),
		recoverTabletCells(ycClient),
		initOpArchiveStep,
		updateOpArchiveStep,
		initQTStep,
		disableSafeMode(ycClient),
	)
	if err != nil {
		return nil, err
	}

	return []flows.StepType{
		ytsaurusClientStep,
		flows.BoolConditionStep{
			Name:  IsFullUpdateStepName,
			Cond:  isFullUpdateNeeded,
			True:  fullUpdateComponetsChain,
			False: componentsUpdateChain,
		},
	}, nil
}
