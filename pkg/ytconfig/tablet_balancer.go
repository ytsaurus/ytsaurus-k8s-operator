package ytconfig

import (
	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"k8s.io/utils/ptr"
)

type TabletBalancer struct {
	Period                      int64 `yson:"period,omitempty"`
	WorkerThreadPoolSize        int   `yson:"worker_thread_pool_size,omitempty"`
	PivotPickerThreadPoolSize   int   `yson:"pivot_picker_thread_pool_size,omitempty"`
	ParameterizedTimeoutOnStart int64 `yson:"parameterized_timeout_on_start,omitempty"`
	ParameterizedTimeout        int64 `yson:"parameterized_timeout,omitempty"`
}

type TabletBalancerServer struct {
	CommonServer
	TabletBalancer TabletBalancer `yson:"tablet_balancer"`
}

func getTabletBalancerLogging(spec *ytv1.TabletBalancerSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"tablet-balancer",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getTabletBalancerServerCarcass(spec *ytv1.TabletBalancerSpec) (TabletBalancerServer, error) {
	var c TabletBalancerServer
	c.RPCPort = consts.TabletBalancerRPCPort
	c.MonitoringPort = ptr.Deref(spec.MonitoringPort, consts.TabletBalancerMonitoringPort)
	c.Logging = getTabletBalancerLogging(spec)

	return c, nil
}
