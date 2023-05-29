package ytconfig

import (
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

type QueryTrackerServer struct {
	CommonServer
	User                       string `yson:"user"`
	CreateStateTablesOnStartup bool   `yson:"create_state_tables_on_startup"`
}

func getQueryTrackerServerCarcass(spec ytv1.QueryTrackerSpec) (QueryTrackerServer, error) {
	var c QueryTrackerServer
	c.RPCPort = consts.QueryTrackerRPCPort
	c.MonitoringPort = consts.QueryTrackerMonitoringPort

	c.User = "query_tracker"
	c.CreateStateTablesOnStartup = true

	c.Logging = createLogging(&spec.InstanceSpec, "query-tracker", []ytv1.LoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})

	return c, nil
}
