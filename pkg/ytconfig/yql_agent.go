package ytconfig

import (
	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type ClusterMapping struct {
	Name    string `yson:"name"`
	Cluster string `yson:"cluster"`
	Default bool   `yson:"default"`
}

type MrJobSystemLibsWithMd5 struct {
	File string `yson:"file"`
}

type GatewayConfig struct {
	MRJobBinary            string                   `yson:"mr_job_bin"`
	UDFDirectory           string                   `yson:"mr_job_udfs_dir"`
	ClusterMapping         []ClusterMapping         `yson:"cluster_mapping"`
	MrJobSystemLibsWithMd5 []MrJobSystemLibsWithMd5 `yson:"mr_job_system_libs_with_md5"`
}

type YQLAgent struct {
	GatewayConfig          GatewayConfig `yson:"gateway_config"`
	YqlPluginSharedLibrary string        `yson:"yql_plugin_shared_library"`
	YTTokenPath            string        `yson:"yt_token_path"`

	// For backward compatibility.
	MRJobBinary        string            `yson:"mr_job_binary"`
	UDFDirectory       string            `yson:"udf_directory"`
	AdditionalClusters map[string]string `yson:"additional_clusters"`
	DefaultCluster     string            `yson:"default_cluster"`
}

type YQLAgentServer struct {
	CommonServer
	User     string   `yson:"user"`
	YQLAgent YQLAgent `yson:"yql_agent"`
}

func getYQLAgentLogging(spec *ytv1.YQLAgentSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"yql-agent",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultDebugLoggerSpec(), defaultStderrLoggerSpec()})
}

func getYQLAgentServerCarcass(spec *ytv1.YQLAgentSpec) (YQLAgentServer, error) {
	var c YQLAgentServer
	c.RPCPort = consts.YQLAgentRPCPort
	c.MonitoringPort = *spec.InstanceSpec.MonitoringPort

	c.User = "yql_agent"

	c.YQLAgent.GatewayConfig.UDFDirectory = "/usr/lib/yql"
	if spec.ConfigureMrJobDynamicLibraries {
		c.YQLAgent.GatewayConfig.MrJobSystemLibsWithMd5 = []MrJobSystemLibsWithMd5{
			MrJobSystemLibsWithMd5{
				File: "/usr/lib/yql/libiconv.so",
			},
			MrJobSystemLibsWithMd5{
				File: "/usr/lib/yql/liblibidn-dynamic.so",
			},
		}
	}
	c.YQLAgent.GatewayConfig.MRJobBinary = "/usr/bin/mrjob"
	c.YQLAgent.YqlPluginSharedLibrary = "/usr/lib/yql/libyqlplugin.so"

	// For backward compatibility.
	c.YQLAgent.UDFDirectory = "/usr/lib/yql"
	c.YQLAgent.MRJobBinary = "/usr/bin/mrjob"
	c.YQLAgent.YTTokenPath = consts.DefaultYqlTokenPath

	c.Logging = getYQLAgentLogging(spec)

	return c, nil
}
