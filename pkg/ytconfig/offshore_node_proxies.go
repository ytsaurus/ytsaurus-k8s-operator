package ytconfig

import (
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type OffshoreNodeProxyServer struct {
	CommonServer

	BusClient *Bus `yson:"bus_client,omitempty"`
}

func getOffshoreNodeProxiesLogging(spec *ytv1.OffshoreNodeProxiesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"offshore-node-proxy",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getOffshoreNodeProxiesCarcass(spec *ytv1.OffshoreNodeProxiesSpec) (OffshoreNodeProxyServer, error) {
	var onps OffshoreNodeProxyServer
	onps.RPCPort = consts.OffshoreNodeProxyRPCPort
	onps.MonitoringPort = ptr.Deref(spec.MonitoringPort, consts.OffshoreNodeProxyMonitoringPort)
	onps.Logging = getOffshoreNodeProxiesLogging(spec)

	return onps, nil
}
