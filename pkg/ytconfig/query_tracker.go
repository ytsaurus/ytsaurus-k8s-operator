package ytconfig

import (
	ytv1 "github.com/YTsaurus/yt-k8s-operator/api/v1"
	"github.com/YTsaurus/yt-k8s-operator/pkg/consts"
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

	loggingBuilder := newLoggingBuilder(ytv1.FindFirstLocation(spec.InstanceGroup.Locations, ytv1.LocationTypeLogs), "query-tracker")
	c.Logging = loggingBuilder.addDefaultInfo().addDefaultStderr().logging

	return c, nil
}
