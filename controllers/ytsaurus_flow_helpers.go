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

	var s components.Component

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

	if resource.Spec.Schedulers != nil {
		s = components.NewScheduler(cfgen, ytsaurus, m, ends, tnds)
		registry.add(s)
	}

	if resource.Spec.ControllerAgents != nil {
		ca := components.NewControllerAgent(cfgen, ytsaurus, m)
		registry.add(ca)
	}

	var q components.Component
	if resource.Spec.QueryTrackers != nil && resource.Spec.Schedulers != nil && resource.Spec.TabletNodes != nil && len(resource.Spec.TabletNodes) > 0 {
		q = components.NewQueryTracker(cfgen, ytsaurus, yc, tnds)
		registry.add(q)
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
	discovery, ok := registry.getByType(consts.ComponentTypeDiscovery)
	if !ok {
		return nil, errors.New("missing discovery component")
	}
	discoveryStep := newComponentStep(discovery, statuses[discovery.GetName()])

	master, ok := registry.getByType(consts.ComponentTypeMaster)
	if !ok {
		return nil, errors.New("missing master component")
	}
	masterStep := newComponentStep(master, statuses[master.GetName()])
	masterStatus := statuses["Master"]

	var httpProxiesSteps []flows.StepType
	for _, hp := range registry.getListByType(consts.ComponentTypeHTTPProxy) {
		httpProxiesSteps = append(httpProxiesSteps, newComponentStep(hp, statuses[hp.GetName()]))
	}
	yc, ok := registry.getByType(consts.ComponentTypeClient)
	if !ok {
		return nil, errors.New("missing client component")
	}
	// temporary
	ycClient := yc.(ytsaurusClient)
	ytsaurusClientStep := newComponentStep(yc, statuses[yc.GetName()])

	var dataNodesSteps []flows.StepType
	for _, dn := range registry.getListByType(consts.ComponentTypeDataNode) {
		dataNodesSteps = append(dataNodesSteps, newComponentStep(dn, statuses[dn.GetName()]))
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
		// (optional) scheduler (depends on master, exec nodes, tablet nodes)
		// (optional) controller agents (depends on master)
		// (optional) querytrackers (depends on yt client and tablet nodes)
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
		//masterExitReadOnly(master),
		recoverTabletCells(ycClient),
		//updateOpArchive(),
		//updateQTState(),
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
