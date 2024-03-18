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
	components map[ComponentName]component
}

func buildComponents(ytsaurus *apiProxy.Ytsaurus, clusterDomain string) *componentRegistry {
	resource := ytsaurus.GetResource()
	cfgen := ytconfig.NewGenerator(resource, clusterDomain)
	nodeCfgGen := ytconfig.NewLocalNodeGenerator(ytsaurus.GetResource(), clusterDomain)

	single := make(map[ComponentName]component)

	ytClient := components.NewYtsaurusClient(cfgen, ytsaurus, nil)

	single[MasterName] = components.NewMaster(cfgen, ytsaurus)
	single[DiscoveryName] = components.NewDiscovery(cfgen, ytsaurus)
	single[YtsaurusClientName] = ytClient

	dnds := make(map[string]component)
	if resource.Spec.DataNodes != nil && len(resource.Spec.DataNodes) > 0 {
		for _, dndSpec := range resource.Spec.DataNodes {
			dnd := components.NewDataNode(nodeCfgGen, ytsaurus, nil, dndSpec)
			dnds[dnd.GetName()] = dnd
		}
	}
	single[DataNodeName] = newMultiComponent(DataNodeName, dnds)

	tnds := make(map[string]component)
	if resource.Spec.TabletNodes != nil && len(resource.Spec.TabletNodes) > 0 {
		for idx, tndSpec := range resource.Spec.TabletNodes {
			tnd := components.NewTabletNode(nodeCfgGen, ytsaurus, ytClient, tndSpec, idx == 0)
			tnds[tnd.GetName()] = tnd
		}
	}
	single[TabletNodeName] = newMultiComponent(TabletNodeName, tnds)

	ends := make(map[string]component)
	if resource.Spec.ExecNodes != nil && len(resource.Spec.ExecNodes) > 0 {
		for _, endSpec := range resource.Spec.ExecNodes {
			end := components.NewExecNode(nodeCfgGen, ytsaurus, nil, endSpec)
			ends[end.GetName()] = end
		}
	}
	single[ExecNodeName] = newMultiComponent(ExecNodeName, ends)

	hps := make(map[string]component)
	for _, hpSpec := range resource.Spec.HTTPProxies {
		hp := components.NewHTTPProxy(cfgen, ytsaurus, nil, hpSpec)
		hps[hp.GetName()] = hp
	}
	single[HttpProxyName] = newMultiComponent(HttpProxyName, hps)

	return &componentRegistry{
		components: single,
	}
}
