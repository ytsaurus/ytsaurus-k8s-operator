package controllers

import (
	"context"

	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type component interface {
	//Fetch(ctx context.Context) error
	Sync(ctx context.Context) error
	Status(ctx context.Context) (components.ComponentStatus, error)
	GetName() string
	GetType() consts.ComponentType
}

type componentRegistry struct {
	comps  map[string]component
	byType map[consts.ComponentType][]component
}

func (cr *componentRegistry) add(comp component) {
	cr.comps[comp.GetName()] = comp
	compsOfSameType := cr.byType[comp.GetType()]
	compsOfSameType = append(compsOfSameType, comp)
}

func (cr *componentRegistry) list() []component {
	var result []component
	for _, comp := range cr.comps {
		result = append(result, comp)
	}
	return result
}
func (cr *componentRegistry) listByType(types ...consts.ComponentType) []component {
	var result []component
	for _, compType := range types {
		result = append(result, cr.byType[compType]...)
	}
	return result
}

func buildComponentRegistry(
	ytsaurus *apiProxy.Ytsaurus,
) *componentRegistry {
	registry := &componentRegistry{
		comps: make(map[string]component),
	}

	resource := ytsaurus.GetResource()
	clusterDomain := getClusterDomain(ytsaurus.APIProxy().Client())
	cfgen := ytconfig.NewGenerator(resource, clusterDomain)

	yc := components.NewYtsaurusClient(cfgen, ytsaurus)
	registry.add(yc)

	d := components.NewDiscovery(cfgen, ytsaurus)
	registry.add(d)

	m := components.NewMaster(cfgen, ytsaurus, yc)
	registry.add(m)

	for _, hpSpec := range ytsaurus.GetResource().Spec.HTTPProxies {
		hp := components.NewHTTPProxy(cfgen, ytsaurus, hpSpec)
		registry.add(hp)
	}

	nodeCfgGen := ytconfig.NewLocalNodeGenerator(ytsaurus.GetResource(), clusterDomain)
	if resource.Spec.DataNodes != nil && len(resource.Spec.DataNodes) > 0 {
		for _, dndSpec := range ytsaurus.GetResource().Spec.DataNodes {
			dnd := components.NewDataNode(nodeCfgGen, ytsaurus, dndSpec)
			registry.add(dnd)
		}
	}

	if resource.Spec.UI != nil {
		ui := components.NewUI(cfgen, ytsaurus)
		registry.add(ui)
	}

	if resource.Spec.RPCProxies != nil && len(resource.Spec.RPCProxies) > 0 {
		for _, rpSpec := range ytsaurus.GetResource().Spec.RPCProxies {
			rp := components.NewRPCProxy(cfgen, ytsaurus, rpSpec)
			registry.add(rp)
		}
	}

	if resource.Spec.TCPProxies != nil && len(resource.Spec.TCPProxies) > 0 {
		for _, tpSpec := range ytsaurus.GetResource().Spec.TCPProxies {
			tp := components.NewTCPProxy(cfgen, ytsaurus, tpSpec)
			registry.add(tp)
		}
	}

	if resource.Spec.ExecNodes != nil && len(resource.Spec.ExecNodes) > 0 {
		for _, endSpec := range ytsaurus.GetResource().Spec.ExecNodes {
			end := components.NewExecNode(nodeCfgGen, ytsaurus, endSpec)
			registry.add(end)
		}
	}

	tndCount := 0
	if resource.Spec.TabletNodes != nil && len(resource.Spec.TabletNodes) > 0 {
		for idx, tndSpec := range ytsaurus.GetResource().Spec.TabletNodes {
			tnd := components.NewTabletNode(nodeCfgGen, ytsaurus, yc, tndSpec, idx == 0)
			registry.add(tnd)
			tndCount++
		}
	}
	if resource.Spec.Schedulers != nil {
		s := components.NewScheduler(cfgen, ytsaurus, tndCount)
		registry.add(s)
	}

	if resource.Spec.ControllerAgents != nil {
		ca := components.NewControllerAgent(cfgen, ytsaurus)
		registry.add(ca)
	}

	var q component
	if resource.Spec.QueryTrackers != nil && resource.Spec.Schedulers != nil && resource.Spec.TabletNodes != nil && len(resource.Spec.TabletNodes) > 0 {
		q = components.NewQueryTracker(cfgen, ytsaurus, yc, tndCount)
		registry.add(q)
	}

	if resource.Spec.QueueAgents != nil && resource.Spec.TabletNodes != nil && len(resource.Spec.TabletNodes) > 0 {
		qa := components.NewQueueAgent(cfgen, ytsaurus, yc, tndCount)
		registry.add(qa)
	}

	if resource.Spec.YQLAgents != nil {
		yqla := components.NewYQLAgent(cfgen, ytsaurus)
		registry.add(yqla)
	}

	if (resource.Spec.DeprecatedChytController != nil || resource.Spec.StrawberryController != nil) && resource.Spec.Schedulers != nil {
		strawberry := components.NewStrawberryController(cfgen, ytsaurus)
		registry.add(strawberry)
	}

	if resource.Spec.MasterCaches != nil {
		mc := components.NewMasterCache(cfgen, ytsaurus)
		registry.add(mc)
	}

	return registry
}
