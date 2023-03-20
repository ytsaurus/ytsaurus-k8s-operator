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

func (g *Generator) GetYqlAgentStatefulSetName() string {
	return g.getName("yqla")
}

func (g *Generator) GetMastersServiceName() string {
	return g.getName("masters")
}

func (g *Generator) GetDiscoveryServiceName() string {
	return g.getName("discovery")
}

func (g *Generator) GetYqlAgentServiceName() string {
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

func (g *Generator) GetYqlAgentPodNames() []string {
	podNames := make([]string, 0, g.ytsaurus.Spec.YqlAgents.InstanceGroup.InstanceCount)
	for i := 0; i < int(g.ytsaurus.Spec.YqlAgents.InstanceGroup.InstanceCount); i++ {
		podNames = append(podNames, fmt.Sprintf("%s-%d", g.GetYqlAgentStatefulSetName(), i))
	}

	return podNames
}

func (g *Generator) GetHttpProxiesServiceAddress() string {
	return fmt.Sprintf("%s.%s.svc.%s",
		g.GetHttpProxiesHeadlessServiceName(),
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

func (g *Generator) GetHttpProxiesServiceName() string {
	return g.getName("http-proxies-lb")
}

func (g *Generator) GetHttpProxiesHeadlessServiceName() string {
	return g.getName("http-proxies")
}

func (g *Generator) GetHttpProxiesStatefulSetName() string {
	return g.getName("hp")
}

func (g *Generator) GetHttpProxiesAddress() string {
	return fmt.Sprintf("%s.%s.svc.%s",
		g.GetHttpProxiesServiceName(),
		g.ytsaurus.Namespace,
		g.clusterDomain)
}

func (g *Generator) GetSchedulerStatefulSetName() string {
	return g.getName("sch")
}

func (g *Generator) GetSchedulerServiceName() string {
	return g.getName("schedulers")
}

func (g *Generator) GetRpcProxiesStatefulSetName() string {
	return g.getName("rp")
}

func (g *Generator) GetRpcProxiesServiceName() string {
	return g.getName("rpc-proxies")
}

func (g *Generator) GetQueryTrackerStatefulSetName() string {
	return g.getName("qt")
}

func (g *Generator) GetQueryTrackerServiceName() string {
	return g.getName("query-trackers")
}
