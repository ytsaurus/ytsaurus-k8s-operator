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

func (g *Generator) GetHTTPProxiesServiceAddress(role string) string {
	return fmt.Sprintf("%s.%s.svc.%s",
		g.GetHTTPProxiesHeadlessServiceName(role),
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

func (g *Generator) GetHTTPProxiesServiceName(role string) string {
	return g.getName(fmt.Sprintf("http-proxies-%s-lb", role))
}

func (g *Generator) GetHTTPProxiesHeadlessServiceName(role string) string {
	return g.getName(fmt.Sprintf("http-proxies-%s", role))
}

func (g *Generator) GetHTTPProxiesStatefulSetName(role string) string {
	return g.getName(fmt.Sprintf("hp-%s", role))
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
	return g.getName(fmt.Sprintf("rp-%s", role))
}

func (g *Generator) GetRPCProxiesServiceName(role string) string {
	return g.getName(fmt.Sprintf("rpc-proxies-%s-lb", role))
}

func (g *Generator) GetRPCProxiesHeadlessServiceName(role string) string {
	return g.getName(fmt.Sprintf("rpc-proxies-%s", role))
}

func (g *Generator) GetQueryTrackerStatefulSetName() string {
	return g.getName("qt")
}

func (g *Generator) GetQueryTrackerServiceName() string {
	return g.getName("query-trackers")
}
