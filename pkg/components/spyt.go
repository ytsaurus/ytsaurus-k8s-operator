package components

import (
	"context"
	"fmt"
	"strings"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type spyt struct {
	ComponentBase

	initEnvironment *InitJob

	master   Component
	execNode Component

	spytVersion  string
	sparkVersion string
}

func NewSpyt(
	cfgen *ytconfig.Generator,
	apiProxy *apiproxy.APIProxy,
	master Component,
	execNode Component) Component {

	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: "yt-spyt",
		ComponentName:  "SPYT",
	}

	return &spyt{
		ComponentBase: ComponentBase{
			labeller: &labeller,
			apiProxy: apiProxy,
			cfgen:    cfgen,
		},
		master:   master,
		execNode: execNode,

		sparkVersion: ytsaurus.Spec.Spyt.SparkVersion,
		spytVersion:  ytsaurus.Spec.Spyt.SpytVersion,

		initEnvironment: NewInitJob(
			&labeller,
			apiProxy,
			"spyt-environment",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig),
	}
}

func (s *spyt) getSpytGlobalConf() string {
	return fmt.Sprintf(`{
		"latest_spark_cluster_version" = "%s";
		"operation_spec" = {
			"job_cpu_monitor" = {
				"enable_cpu_reclaim" = "false"
			}
		};
		"python_cluster_paths" = {
			"3.7" = "/opt/conda/bin/python3.7";
		};
		"layer_paths" = [
		];
		"worker_num_limit" = 1000;
		"environment" = {
			"ARROW_ENABLE_UNSAFE_MEMORY_ACCESS" = "true";
			"YT_ALLOW_HTTP_REQUESTS_TO_YT_FROM_JOB" = "1";
			"JAVA_HOME" = "/usr/bin/java";
			"ARROW_ENABLE_NULL_CHECK_FOR_GET" = "false";
			"IS_SPARK_CLUSTER" = "true";
		};
		"spark_conf" = {
			"spark.yt.log.enabled" = "false";
			"spark.hadoop.yt.proxyRole" = "spark";
			"spark.datasource.yt.recursiveFileLookup" = "true";
		};
	}`, s.sparkVersion)
}

func (s *spyt) getSparkLaunchConf() string {
	return fmt.Sprintf(`{
		"layer_paths" = [
		];
		"spark_yt_base_path" = "//home/spark/bin/releases/%s";
		"file_paths" = [
			"//home/spark/bin/releases/%s/spark.tgz";
			"//home/spark/bin/releases/%s/spark-yt-launcher.jar";
			"//home/spark/conf/releases/%s/metrics.properties";
			"//home/spark/conf/releases/%s/solomon-agent.template.conf";
        	"//home/spark/conf/releases/%s/solomon-service-master.template.conf";
        	"//home/spark/conf/releases/%s/solomon-service-worker.template.conf";
		];
		"spark_conf" = {
			"spark.yt.jarCaching" = "true";
		};
		"environment" = {};
		"enablers" = {
			"enable_byop" = %%false;
			"enable_arrow" = %%true;
			"enable_mtn" = %%true;
		};
	}`, s.sparkVersion, s.sparkVersion, s.sparkVersion, s.sparkVersion, s.sparkVersion, s.sparkVersion, s.sparkVersion)
}

func (s *spyt) createInitScript(sparkVersion string, spytVersion string) string {
	metricsPath := fmt.Sprintf("//home/spark/conf/releases/%s/metrics.properties", sparkVersion)
	sparkLaunchConfPath := fmt.Sprintf("//home/spark/conf/releases/%s/spark-launch-conf", sparkVersion)
	sparkPath := fmt.Sprintf("//home/spark/spark/releases/%s/spark.tgz", sparkVersion)
	sparkYtDataSourcePath := fmt.Sprintf("//home/spark/spyt/releases/%s/spark-yt-data-source.jar", spytVersion)
	spytPath := fmt.Sprintf("//home/spark/spyt/releases/%s/spyt.zip", spytVersion)
	sparkYtLauncherPath := fmt.Sprintf("//home/spark/bin/releases/%s/spark-yt-launcher.jar", sparkVersion)
	solomonAgentPath := fmt.Sprintf("//home/spark/conf/releases/%s/solomon-agent.template.conf", sparkVersion)
	solomonServiceMasterPath := fmt.Sprintf("//home/spark/conf/releases/%s/solomon-service-master.template.conf", sparkVersion)
	solomonServiceWorkerPath := fmt.Sprintf("//home/spark/conf/releases/%s/solomon-service-worker.template.conf", sparkVersion)

	script := []string{
		initJobWithNativeDriverPrologue(),

		// Configs.
		"/usr/bin/yt create document //home/spark/conf/global --ignore-existing -r",
		fmt.Sprintf("/usr/bin/yt set //home/spark/conf/global '%s'", s.getSpytGlobalConf()),

		"/usr/bin/yt create file " + metricsPath + " --ignore-existing -r",
		"/usr/bin/yt set " + metricsPath + "/@replication_factor 1",
		"cat /usr/bin/metrics.properties | /usr/bin/yt write-file " + metricsPath,

		"/usr/bin/yt create file " + solomonAgentPath + " --ignore-existing",
		"/usr/bin/yt set " + solomonAgentPath + "/@replication_factor 1",
		"cat /usr/bin/solomon-agent.template.conf | /usr/bin/yt upload " + solomonAgentPath,

		"/usr/bin/yt create file " + solomonServiceMasterPath + " --ignore-existing",
		"/usr/bin/yt set " + solomonServiceMasterPath + "/@replication_factor 1",
		"cat /usr/bin/solomon-service-master.template.conf | /usr/bin/yt upload " + solomonServiceMasterPath,

		"/usr/bin/yt create file " + solomonServiceWorkerPath + " --ignore-existing",
		"/usr/bin/yt set " + solomonServiceWorkerPath + "/@replication_factor 1",
		"cat /usr/bin/solomon-service-worker.template.conf | /usr/bin/yt upload " + solomonServiceWorkerPath,

		"/usr/bin/yt create document " + sparkLaunchConfPath + " --ignore-existing",
		fmt.Sprintf("/usr/bin/yt set "+sparkLaunchConfPath+" '%s'", s.getSparkLaunchConf()),

		// spark and spyt jars.

		"/usr/bin/yt create file " + sparkPath + " --ignore-existing -r",
		"/usr/bin/yt set " + sparkPath + "/@replication_factor 1",
		"cat /usr/bin/spark.tgz | /usr/bin/yt write-file " + sparkPath,

		"/usr/bin/yt create file " + sparkYtDataSourcePath + " --ignore-existing -r",
		"/usr/bin/yt set " + sparkYtDataSourcePath + "/@replication_factor 1",
		"cat /usr/bin/spark-yt-data-source.jar | /usr/bin/yt upload " + sparkYtDataSourcePath,

		"/usr/bin/yt create file " + spytPath + " --ignore-existing",
		"/usr/bin/yt set " + spytPath + "/@replication_factor 1",
		"cat /usr/bin/spyt.zip | /usr/bin/yt upload " + spytPath,

		"/usr/bin/yt create file " + sparkYtLauncherPath + " --ignore-existing -r",
		"/usr/bin/yt set " + sparkYtLauncherPath + "/@replication_factor 1",
		"cat /usr/bin/spark-yt-launcher.jar | /usr/bin/yt upload " + sparkYtLauncherPath,

		fmt.Sprintf("/usr/bin/yt link //home/spark/spark/releases/%s/spark.tgz //home/spark/bin/releases/%s/spark.tgz -f",
			sparkVersion,
			sparkVersion),

		// TODO: remove this after updating ytsaurus-spyt package
		"/usr/bin/yt link //home //sys/spark",
		"/usr/bin/yt set //home/spark/conf/global/ytserver_proxy_path '\"//sys/spark\"'",
	}

	return strings.Join(script, "\n")
}

func (s *spyt) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error

	// TODO: add update logic.

	if s.master.Status(ctx) != SyncStatusReady {
		return SyncStatusBlocked, err
	}

	if s.execNode.Status(ctx) != SyncStatusReady {
		return SyncStatusBlocked, err
	}

	if !dry {
		s.initEnvironment.SetInitScript(s.createInitScript(s.sparkVersion, s.spytVersion))
	}

	return s.initEnvironment.Sync(ctx, dry)
}

func (s *spyt) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		s.initEnvironment,
	})
}

func (s *spyt) Status(ctx context.Context) SyncStatus {
	status, err := s.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (s *spyt) Sync(ctx context.Context) error {
	_, err := s.doSync(ctx, false)
	return err
}
