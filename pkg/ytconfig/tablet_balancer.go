package ytconfig

import (
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type TabletBalancer struct {
	WorkerThreadPoolSize int `yson:"worker_thread_pool_size"`
}

type TabletBalancerServer struct {
	CommonServer

	TabletBalancer TabletBalancer `yson:"tablet_balancer"`
}

func getTabletBalancerLogging(spec *ytv1.TabletBalancerSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		consts.GetServiceKebabCase(consts.TabletBalancerType),
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getTabletBalancerServerCarcass(spec *ytv1.TabletBalancerSpec) (TabletBalancerServer, error) {
	var c TabletBalancerServer

	c.MonitoringPort = ptr.Deref(spec.InstanceSpec.MonitoringPort, consts.TabletBalancerMonitoringPort)
	c.RPCPort = consts.TabletBalancerRPCPort

	c.Logging = getTabletBalancerLogging(spec)

	c.TabletBalancer = TabletBalancer{
		WorkerThreadPoolSize: 3,
	}

	return c, nil
}
