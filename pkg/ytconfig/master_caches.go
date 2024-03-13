package ytconfig

import (
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

type MasterCacheServer struct {
	CommonServer
}

func getMasterCachesLogging(spec *ytv1.MasterCachesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"master-cache",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getMasterCachesCarcass(spec *ytv1.MasterCachesSpec) (MasterCacheServer, error) {
	var mcs MasterCacheServer
	mcs.RPCPort = consts.MasterCachesRPCPort
	mcs.MonitoringPort = consts.MasterCachesMonitoringPort

	mcs.Logging = getMasterCachesLogging(spec)

	return mcs, nil
}
