package ytconfig

import (
	ytv1 "github.com/YTsaurus/yt-k8s-operator/api/v1"
	"github.com/YTsaurus/yt-k8s-operator/pkg/consts"
)

type YqlEmbedded struct {
	MRJobBinary  string `yson:"mr_job_binary"`
	UdfDirectory string `yson:"udf_directory"`
}

type YqlAgent struct {
	YqlEmbedded
	AdditionalClusters map[string]string `yson:"additional_clusters"`
}

type YqlAgentServer struct {
	CommonServer
	User     string   `yson:"user"`
	YqlAgent YqlAgent `yson:"yql_agent"`
}

func getYqlAgentServerCarcass(spec ytv1.YqlAgentSpec) (YqlAgentServer, error) {
	var c YqlAgentServer
	c.RpcPort = consts.YqlAgentRpcPort
	c.MonitoringPort = consts.YqlAgentMonitoringPort

	c.User = "yql_agent"

	c.YqlAgent.UdfDirectory = "/usr/lib/yql"
	c.YqlAgent.MRJobBinary = "/usr/bin/mrjob"

	loggingBuilder := newLoggingBuilder(ytv1.FindFirstLocation(spec.InstanceGroup.Locations, ytv1.LocationTypeLogs), "yql-agent")
	c.Logging = loggingBuilder.addDefaultInfo().addDefaultStderr().addDefaultDebug().logging

	return c, nil
}
