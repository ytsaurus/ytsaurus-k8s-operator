package ytconfig

import (
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
		"query-tracker",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getQueryTrackerServerCarcass(spec *ytv1.QueryTrackerSpec) (QueryTrackerServer, error) {
	var c QueryTrackerServer
	c.RPCPort = consts.QueryTrackerRPCPort
	c.MonitoringPort = consts.QueryTrackerMonitoringPort
	if spec.InstanceSpec.MonitoringPort != nil {
		c.MonitoringPort = *spec.InstanceSpec.MonitoringPort
	}

	c.User = "query_tracker"
	c.CreateStateTablesOnStartup = true

	c.Logging = getQueryTrackerLogging(spec)

	return c, nil
}
