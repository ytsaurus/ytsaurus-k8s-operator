package ytconfig

import (
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

type Discovery struct {
	// Unfortunately AddressList is not applicable here, since
	// config field is named differently.
	Addresses []string `yson:"server_addresses"`
}

type DiscoveryServer struct {
	BasicServer
	DiscoveryServer Discovery `yson:"discovery_server"`
}

func getDiscoveryServerCarcass(spec ytv1.DiscoverySpec) DiscoveryServer {
	var c DiscoveryServer
	c.MonitoringPort = consts.DiscoveryMonitoringPort
	c.RPCPort = consts.DiscoveryRPCPort

	loggingBuilder := newLoggingBuilder(ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeLogs), "master")
	c.Logging = loggingBuilder.addDefaultInfo().addDefaultStderr().logging

	return c
}
