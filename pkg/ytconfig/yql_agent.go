package ytconfig

import (
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

type YQLPlugin struct {
	MRJobBinary            string `yson:"mr_job_binary"`
	UDFDirectory           string `yson:"udf_directory"`
	YqlPluginSharedLibrary string `yson:"yql_plugin_shared_library"`
}

type YQLAgent struct {
	YQLPlugin
	AdditionalClusters map[string]string `yson:"additional_clusters"`
	DefaultCluster     string            `yson:"default_cluster"`
}

type YQLAgentServer struct {
	CommonServer
	User     string   `yson:"user"`
	YQLAgent YQLAgent `yson:"yql_agent"`
}

func getYQLAgentLogging(spec ytv1.YQLAgentSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"yql-agent",
		[]ytv1.LoggerSpec{defaultInfoLoggerSpec(), defaultDebugLoggerSpec(), defaultStderrLoggerSpec()})
}

func getYQLAgentServerCarcass(spec ytv1.YQLAgentSpec) (YQLAgentServer, error) {
	var c YQLAgentServer
	c.RPCPort = consts.YQLAgentRPCPort
	c.MonitoringPort = consts.YQLAgentMonitoringPort

	c.User = "yql_agent"

	c.YQLAgent.UDFDirectory = "/usr/lib/yql"
	c.YQLAgent.MRJobBinary = "/usr/bin/mrjob"
	c.YQLAgent.YqlPluginSharedLibrary = "/usr/lib/yql/libyqlplugin.so"

	c.Logging = getYQLAgentLogging(spec)

	return c, nil
}
