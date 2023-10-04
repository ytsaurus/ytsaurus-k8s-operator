package ytconfig

import (
	"fmt"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

func (g *Generator) getName(shortName string) string {
	if g.ytsaurus.Spec.UseShortNames {
		return shortName
	} else {
		return fmt.Sprintf("%s-%s", shortName, g.ytsaurus.Name)
	}
}

func (g *Generator) GetMastersStatefulSetName() string {
	return g.getName("ms")
}

func (g *Generator) GetDiscoveryStatefulSetName() string {
	return g.getName("ds")
}

func (g *Generator) GetYQLAgentStatefulSetName() string {
	return g.getName("yqla")
}

func (g *Generator) GetMastersServiceName() string {
	return g.getName("masters")
}

func (g *Generator) GetDiscoveryServiceName() string {
	return g.getName("discovery")
}

func (g *Generator) GetYQLAgentServiceName() string {
	return g.getName("yql-agents")
}

func (g *Generator) GetMasterPodNames() []string {
	podNames := make([]string, 0, g.ytsaurus.Spec.PrimaryMasters.InstanceSpec.InstanceCount)
	for i := 0; i < int(g.ytsaurus.Spec.PrimaryMasters.InstanceSpec.InstanceCount); i++ {
		podNames = append(podNames, fmt.Sprintf("%s-%d", g.GetMastersStatefulSetName(), i))
	}

	return podNames
}

func (g *Generator) GetDiscoveryPodNames() []string {
	podNames := make([]string, 0, g.ytsaurus.Spec.Discovery.InstanceSpec.InstanceCount)
	for i := 0; i < int(g.ytsaurus.Spec.Discovery.InstanceSpec.InstanceCount); i++ {
		podNames = append(podNames, fmt.Sprintf("%s-%d", g.GetDiscoveryStatefulSetName(), i))
	}

	return podNames
}

func (g *Generator) GetYQLAgentPodNames() []string {
	podNames := make([]string, 0, g.ytsaurus.Spec.YQLAgents.InstanceSpec.InstanceCount)
	for i := 0; i < int(g.ytsaurus.Spec.YQLAgents.InstanceSpec.InstanceCount); i++ {
		podNames = append(podNames, fmt.Sprintf("%s-%d", g.GetYQLAgentStatefulSetName(), i))
	}

	return podNames
}

func (g *Generator) GetQueueAgentPodNames() []string {
	podNames := make([]string, 0, g.ytsaurus.Spec.QueueAgents.InstanceSpec.InstanceCount)
	for i := 0; i < int(g.ytsaurus.Spec.QueueAgents.InstanceSpec.InstanceCount); i++ {
		podNames = append(podNames, fmt.Sprintf("%s-%d", g.GetQueueAgentStatefulSetName(), i))
	}

	return podNames
}

func (g *Generator) GetHTTPProxiesServiceAddress(role string) string {
	return fmt.Sprintf("%s.%s.svc.%s",
		g.GetHTTPProxiesHeadlessServiceName(role),
		g.ytsaurus.Namespace,
		g.clusterDomain)
}

func (g *Generator) GetStrawberryControllerServiceAddress() string {
	return fmt.Sprintf("%s.%s.svc.%s",
		g.GetStrawberryControllerHeadlessServiceName(),
		g.ytsaurus.Namespace,
		g.clusterDomain)
}

func (g *Generator) GetStrawberryControllerHeadlessServiceName() string {
	return g.getName("strawberry")
}

func (g *Generator) GetHTTPProxiesServiceName(role string) string {
	return g.getName(fmt.Sprintf("%s-lb", g.FormatComponentStringWithDefault("http-proxies", role)))
}

func (g *Generator) GetHTTPProxiesHeadlessServiceName(role string) string {
	return g.getName(g.FormatComponentStringWithDefault("http-proxies", role))
}

func (g *Generator) GetHTTPProxiesStatefulSetName(role string) string {
	return g.getName(g.FormatComponentStringWithDefault("hp", role))
}

func (g *Generator) GetHTTPProxiesAddress(role string) string {
	return fmt.Sprintf("%s.%s.svc.%s",
		g.GetHTTPProxiesServiceName(role),
		g.ytsaurus.Namespace,
		g.clusterDomain)
}

func (g *Generator) GetSchedulerStatefulSetName() string {
	return g.getName("sch")
}

func (g *Generator) GetSchedulerServiceName() string {
	return g.getName("schedulers")
}

func (g *Generator) GetRPCProxiesStatefulSetName(role string) string {
	return g.getName(g.FormatComponentStringWithDefault("rp", role))
}

func (g *Generator) GetRPCProxiesServiceName(role string) string {
	return g.getName(fmt.Sprintf("%s-lb", g.FormatComponentStringWithDefault("rpc-proxies", role)))
}

func (g *Generator) GetRPCProxiesHeadlessServiceName(role string) string {
	return g.getName(g.FormatComponentStringWithDefault("rpc-proxies", role))
}

func (g *Generator) GetTCPProxiesStatefulSetName(role string) string {
	return g.getName(g.FormatComponentStringWithDefault("tp", role))
}

func (g *Generator) GetTCPProxiesServiceName(role string) string {
	return g.getName(fmt.Sprintf("%s-lb", g.FormatComponentStringWithDefault("tcp-proxies", role)))
}

func (g *Generator) GetTCPProxiesHeadlessServiceName(role string) string {
	return g.getName(g.FormatComponentStringWithDefault("tcp-proxies", role))
}

func (g *Generator) GetQueryTrackerStatefulSetName() string {
	return g.getName("qt")
}

func (g *Generator) GetQueryTrackerServiceName() string {
	return g.getName("query-trackers")
}

func (g *Generator) GetQueueAgentStatefulSetName() string {
	return g.getName("qa")
}

func (g *Generator) GetQueueAgentServiceName() string {
	return g.getName("queue-agents")
}

func (g *Generator) GetDataNodesStatefulSetName(name string) string {
	return g.getName(g.FormatComponentStringWithDefault("dnd", name))
}

func (g *Generator) GetDataNodesServiceName(name string) string {
	return g.getName(g.FormatComponentStringWithDefault("data-nodes", name))
}

func (g *Generator) GetExecNodesStatefulSetName(name string) string {
	return g.getName(g.FormatComponentStringWithDefault("end", name))
}

func (g *Generator) GetExecNodesServiceName(name string) string {
	return g.getName(g.FormatComponentStringWithDefault("exec-nodes", name))
}

func (g *Generator) GetTabletNodesStatefulSetName(name string) string {
	return g.getName(g.FormatComponentStringWithDefault("tnd", name))
}

func (g *Generator) GetTabletNodesServiceName(name string) string {
	return g.getName(g.FormatComponentStringWithDefault("tablet-nodes", name))
}

func (g *Generator) FormatComponentStringWithDefault(base string, name string) string {
	if name != consts.DefaultName {
		return fmt.Sprintf("%s-%s", base, name)
	}
	return base
}
