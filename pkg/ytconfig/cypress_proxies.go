package ytconfig

import (
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type CypressProxyServer struct {
	CommonServer

	BusClient *Bus `yson:"bus_client,omitempty"`
}

func getCypressProxiesLogging(spec *ytv1.CypressProxiesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"cypress-proxy",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getCypressProxiesCarcass(spec *ytv1.CypressProxiesSpec) (CypressProxyServer, error) {
	var cyps CypressProxyServer
	cyps.RPCPort = consts.CypressProxyRPCPort
	cyps.MonitoringPort = ptr.Deref(spec.MonitoringPort, consts.CypressProxyMonitoringPort)
	cyps.Logging = getCypressProxiesLogging(spec)

	return cyps, nil
}
