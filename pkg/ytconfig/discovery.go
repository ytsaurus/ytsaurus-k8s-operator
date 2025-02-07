package ytconfig

import (
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type Discovery struct {
	// Unfortunately AddressList is not applicable here, since
	// config field is named differently.
	Addresses []string `yson:"server_addresses"`
}

type DiscoveryServer struct {
	CommonServer
	DiscoveryServer Discovery `yson:"discovery_server"`
}

func getDiscoveryLogging(spec *ytv1.DiscoverySpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"discovery",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getDiscoveryServerCarcass(spec *ytv1.DiscoverySpec) (DiscoveryServer, error) {
	var c DiscoveryServer

	c.MonitoringPort = ptr.Deref(spec.InstanceSpec.MonitoringPort, consts.DiscoveryMonitoringPort)
	c.RPCPort = consts.DiscoveryRPCPort

	c.Logging = getDiscoveryLogging(spec)

	return c, nil
}
