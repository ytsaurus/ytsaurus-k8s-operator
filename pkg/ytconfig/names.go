package ytconfig

import "fmt"

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
	podNames := make([]string, 0, g.ytsaurus.Spec.Masters.InstanceGroup.InstanceCount)
	for i := 0; i < int(g.ytsaurus.Spec.Masters.InstanceGroup.InstanceCount); i++ {
		podNames = append(podNames, fmt.Sprintf("%s-%d", g.GetMastersStatefulSetName(), i))
	}

	return podNames
}

func (g *Generator) GetDiscoveryPodNames() []string {
	podNames := make([]string, 0, g.ytsaurus.Spec.Discovery.InstanceGroup.InstanceCount)
	for i := 0; i < int(g.ytsaurus.Spec.Discovery.InstanceGroup.InstanceCount); i++ {
		podNames = append(podNames, fmt.Sprintf("%s-%d", g.GetDiscoveryStatefulSetName(), i))
	}

	return podNames
}

func (g *Generator) GetYQLAgentPodNames() []string {
	podNames := make([]string, 0, g.ytsaurus.Spec.YQLAgents.InstanceGroup.InstanceCount)
	for i := 0; i < int(g.ytsaurus.Spec.YQLAgents.InstanceGroup.InstanceCount); i++ {
		podNames = append(podNames, fmt.Sprintf("%s-%d", g.GetYQLAgentStatefulSetName(), i))
	}

	return podNames
}

func (g *Generator) GetHTTPProxiesServiceAddress() string {
	return fmt.Sprintf("%s.%s.svc.%s",
		g.GetHTTPProxiesHeadlessServiceName(),
		g.ytsaurus.Namespace,
		g.clusterDomain)
}

func (g *Generator) GetChytControllerServiceAddress() string {
	return fmt.Sprintf("%s.%s.svc.%s",
		g.GetChytControllerHeadlessServiceName(),
		g.ytsaurus.Namespace,
		g.clusterDomain)
}

func (g *Generator) GetChytControllerHeadlessServiceName() string {
	return g.getName("chyt")
}

func (g *Generator) GetHTTPProxiesServiceName() string {
	return g.getName("http-proxies-lb")
}

func (g *Generator) GetHTTPProxiesHeadlessServiceName() string {
	return g.getName("http-proxies")
}

func (g *Generator) GetHTTPProxiesStatefulSetName() string {
	return g.getName("hp")
}

func (g *Generator) GetHTTPProxiesAddress() string {
	return fmt.Sprintf("%s.%s.svc.%s",
		g.GetHTTPProxiesServiceName(),
		g.ytsaurus.Namespace,
		g.clusterDomain)
}

func (g *Generator) GetSchedulerStatefulSetName() string {
	return g.getName("sch")
}

func (g *Generator) GetSchedulerServiceName() string {
	return g.getName("schedulers")
}

func (g *Generator) GetRPCProxiesStatefulSetName() string {
	return g.getName("rp")
}

func (g *Generator) GetRPCProxiesServiceName() string {
	return g.getName("rpc-proxies-lb")
}

func (g *Generator) GetRPCProxiesHeadlessServiceName() string {
	return g.getName("rpc-proxies")
}

func (g *Generator) GetQueryTrackerStatefulSetName() string {
	return g.getName("qt")
}

func (g *Generator) GetQueryTrackerServiceName() string {
	return g.getName("query-trackers")
}
