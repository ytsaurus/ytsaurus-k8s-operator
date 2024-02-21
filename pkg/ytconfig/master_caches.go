package ytconfig

import (
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

type MasterCacheServer struct {
	CommonServer
	AddressList
}

func getMasterCachesLogging(spec *ytv1.MasterCachesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"master-cache",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getMasterCachesCarcass(spec *ytv1.MasterCachesSpec) (MasterCacheServer, error) {
	var mc MasterCacheServer
	mc.RPCPort = consts.MasterCachesRPCPort
	mc.MonitoringPort = consts.MasterCachesMonitoringPort

	mc.Logging = getMasterCachesLogging(spec)

	return mc, nil
}
