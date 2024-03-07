package ytflow

import (
	"context"

	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

// Aliases just to be short.
type componentStatus = components.ComponentStatus
type syncStatus = components.SyncStatus

type component interface {
	GetName() string
	Fetch(ctx context.Context) error
	Status(ctx context.Context) componentStatus
	Sync(ctx context.Context) error
}

type componentRegistry struct {
	single map[ComponentName]component
	multi  map[ComponentName]map[string]component
}

func buildComponents(ytsaurus *apiProxy.Ytsaurus, clusterDomain string) *componentRegistry {
	resource := ytsaurus.GetResource()
	cfgen := ytconfig.NewGenerator(resource, clusterDomain)
	nodeCfgGen := ytconfig.NewLocalNodeGenerator(ytsaurus.GetResource(), clusterDomain)

	single := make(map[ComponentName]component)
	multi := make(map[ComponentName]map[string]component)

	single[MasterName] = components.NewMaster(cfgen, ytsaurus)
	single[DiscoveryName] = components.NewDiscovery(cfgen, ytsaurus)
	single[YtsaurusClientName] = components.NewYtsaurusClient(cfgen, ytsaurus, nil)

	dnds := make(map[string]component)
	if resource.Spec.DataNodes != nil && len(resource.Spec.DataNodes) > 0 {
		for _, dndSpec := range ytsaurus.GetResource().Spec.DataNodes {
			dnd := components.NewDataNode(nodeCfgGen, ytsaurus, nil, dndSpec)
			dnds[dnd.GetName()] = dnd
		}
	}
	multi[DataNodeName] = dnds

	hps := make(map[string]component)
	for _, hpSpec := range ytsaurus.GetResource().Spec.HTTPProxies {
		hp := components.NewHTTPProxy(cfgen, ytsaurus, nil, hpSpec)
		hps[hp.GetName()] = hp
	}
	multi[HttpProxyName] = hps

	return &componentRegistry{
		single: single,
		multi:  multi,
	}
}
