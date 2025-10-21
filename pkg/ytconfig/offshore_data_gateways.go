package ytconfig

import (
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type OffshoreDataGatewayServer struct {
	CommonServer

	BusClient *Bus `yson:"bus_client,omitempty"`
}

func getOffshoreDataGatewaysLogging(spec *ytv1.OffshoreDataGatewaySpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"offshore-data-gateway",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getOffshoreDataGatewaysCarcass(spec *ytv1.OffshoreDataGatewaySpec) (OffshoreDataGatewayServer, error) {
	var onps OffshoreDataGatewayServer
	onps.RPCPort = consts.OffshoreDataGatewayRPCPort
	onps.MonitoringPort = ptr.Deref(spec.MonitoringPort, consts.OffshoreDataGatewayMonitoringPort)
	onps.Logging = getOffshoreDataGatewaysLogging(spec)

	return onps, nil
}
