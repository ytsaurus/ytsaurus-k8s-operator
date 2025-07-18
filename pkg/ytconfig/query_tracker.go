package ytconfig

import (
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type QueryTrackerServer struct {
	CommonServer
	User                       string `yson:"user"`
	CreateStateTablesOnStartup bool   `yson:"create_state_tables_on_startup"`
}

func getQueryTrackerLogging(spec *ytv1.QueryTrackerSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		consts.GetServiceKebabCase(consts.QueryTrackerType),
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getQueryTrackerServerCarcass(spec *ytv1.QueryTrackerSpec) (QueryTrackerServer, error) {
	var c QueryTrackerServer
	c.RPCPort = consts.QueryTrackerRPCPort
	c.MonitoringPort = ptr.Deref(spec.InstanceSpec.MonitoringPort, consts.QueryTrackerMonitoringPort)
	c.User = "query_tracker"
	c.CreateStateTablesOnStartup = true

	c.Logging = getQueryTrackerLogging(spec)

	return c, nil
}
