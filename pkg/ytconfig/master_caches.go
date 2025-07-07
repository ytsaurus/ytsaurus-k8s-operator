package ytconfig

import (
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type MasterCacheServer struct {
	CommonServer

	BusClient *Bus `yson:"bus_client,omitempty"`
}

func getMasterCachesLogging(spec *ytv1.MasterCachesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		consts.GetServiceKebabCase(consts.MasterCacheType),
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getMasterCachesCarcass(spec *ytv1.MasterCachesSpec) (MasterCacheServer, error) {
	var mcs MasterCacheServer
	mcs.RPCPort = consts.MasterCachesRPCPort
	mcs.MonitoringPort = ptr.Deref(spec.MonitoringPort, consts.MasterCachesMonitoringPort)
	mcs.Logging = getMasterCachesLogging(spec)

	return mcs, nil
}
