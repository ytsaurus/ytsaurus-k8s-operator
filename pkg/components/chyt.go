package components

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	corev1 "k8s.io/api/core/v1"
)

type chytController struct {
	ComponentBase
	microservice    *Microservice
	initUserJob     *InitJob
	initClusterJob  *InitJob
	initChPublicJob *InitJob
	secret          *resources.StringSecret

	master   Component
	dataNode Component
}

const ChytControllerConfigFileName = "chyt-controller.yson"
const ChytInitClusterJobConfigFileName = "chyt-init-cluster.yson"

func NewChytController(
	cfgen *ytconfig.Generator,
	apiProxy *apiproxy.APIProxy,
	master Component,
	dataNode Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: "yt-chyt-controller",
		ComponentName:  "ChytController",
	}

	microservice := NewMicroservice(
		&labeller,
		apiProxy,
		ytsaurus.Spec.CoreImage,
		1,
		cfgen.GetChytControllerConfig,
		ChytControllerConfigFileName,
		"chyt-deployment",
		"chyt")

	return &chytController{
		ComponentBase: ComponentBase{
			labeller: &labeller,
			apiProxy: apiProxy,
			cfgen:    cfgen,
		},
		microservice: microservice,
		initUserJob: NewInitJob(
			&labeller,
			apiProxy,
			"user",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig),
		initClusterJob: NewInitJob(
			&labeller,
			apiProxy,
			"cluster",
			ChytInitClusterJobConfigFileName,
			cfgen.GetChytInitClusterConfig),
		initChPublicJob: NewInitJob(
			&labeller,
			apiProxy,
			"ch-public",
			"",
			nil),
		secret: resources.NewStringSecret(
			labeller.GetSecretName(),
			&labeller,
			apiProxy),
		master:   master,
		dataNode: dataNode,
	}
}

func (c *chytController) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		c.microservice,
		c.initUserJob,
		c.initClusterJob,
		c.initChPublicJob,
		c.secret,
	})
}

func (c *chytController) initUsers() string {
	token, _ := c.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand(consts.ChytUserName, "", token, true)
	commands = append(commands, createUserCommand("yt-clickhouse", "", "", true)...)
	return strings.Join(commands, "\n")
}

func (c *chytController) createInitUserScript() string {
	script := []string{
		initJobWithNativeDriverPrologue(),
		c.initUsers(),
		"/usr/bin/yt create user --attributes '{name=\"yt-clickhouse\"}' --ignore-existing",
		"/usr/bin/yt create map_node //sys/bin --ignore-existing",
		"/usr/bin/yt create map_node //sys/bin/clickhouse-trampoline --ignore-existing",
		"/usr/bin/yt create map_node //sys/bin/ytserver-log-tailer --ignore-existing",
		"/usr/bin/yt create map_node //sys/bin/ytserver-clickhouse --ignore-existing",
		"if [[ `/usr/bin/yt exists //sys/bin/clickhouse-trampoline/clickhouse-trampoline` == 'false' ]]; then /usr/bin/yt write-file //sys/bin/clickhouse-trampoline/clickhouse-trampoline < /usr/bin/clickhouse-trampoline; fi",
		"/usr/bin/yt set //sys/bin/clickhouse-trampoline/clickhouse-trampoline/@executable %true",
		"if [[ `/usr/bin/yt exists //sys/bin/ytserver-log-tailer/ytserver-log-tailer` == 'false' ]]; then /usr/bin/yt write-file //sys/bin/ytserver-log-tailer/ytserver-log-tailer < /usr/bin/ytserver-log-tailer; fi",
		"/usr/bin/yt set //sys/bin/ytserver-log-tailer/ytserver-log-tailer/@executable %true",
		"if [[ `/usr/bin/yt exists //sys/bin/ytserver-clickhouse/ytserver-clickhouse` == 'false' ]]; then /usr/bin/yt write-file //sys/bin/ytserver-clickhouse/ytserver-clickhouse < /usr/bin/ytserver-clickhouse; fi",
		"/usr/bin/yt set //sys/bin/ytserver-clickhouse/ytserver-clickhouse/@executable %true",
		"/usr/bin/yt create access_control_object_namespace --attributes '{name=chyt}' --ignore-existing",
	}

	return strings.Join(script, "\n")
}

func (c *chytController) createInitChPublicScript() string {
	script := []string{
		initJobPrologue,
		fmt.Sprintf("export YT_PROXY=%v CHYT_CTL_ADDRESS=%v", c.cfgen.GetHTTPProxiesAddress(), c.cfgen.GetChytControllerServiceAddress()),
		"yt clickhouse ctl create ch_public || true",
		"yt clickhouse ctl set-option --alias ch_public enable_geodata '%false'",
		"yt clickhouse ctl set-option --alias ch_public instance_cpu 2",
		"yt clickhouse ctl set-option --alias ch_public instance_memory '{reader=100000000;chunk_meta_cache=100000000;compressed_cache=100000000;clickhouse=100000000;clickhouse_watermark=10;footprint=500000000;log_tailer=100000000;watchdog_oom_watermark=0;watchdog_oom_window_watermark=0}'",
		"yt clickhouse ctl set-option --alias ch_public instance_count 1",
		"yt clickhouse ctl start ch_public --untracked",
	}

	return strings.Join(script, "\n")
}

func (c *chytController) createInitClusterScript() string {
	script := []string{
		initJobPrologue,
		fmt.Sprintf("/usr/bin/chyt-controller --config-path %s init-cluster",
			path.Join(consts.ConfigMountPoint, ChytInitClusterJobConfigFileName)),
	}

	return strings.Join(script, "\n")
}

func (c *chytController) getEnvSource() []corev1.EnvFromSource {
	return []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: c.secret.Name(),
				},
			},
		},
	}
}

func (c *chytController) prepareInitClusterJob() {
	c.initClusterJob.SetInitScript(c.createInitClusterScript())

	job := c.initClusterJob.Build()
	container := &job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []corev1.EnvFromSource{c.secret.GetEnvSource()}
}

func (c *chytController) prepareChPublicJob() {
	c.initChPublicJob.SetInitScript(c.createInitChPublicScript())

	job := c.initChPublicJob.Build()
	container := &job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []corev1.EnvFromSource{c.secret.GetEnvSource()}
}

func (c *chytController) syncComponents(ctx context.Context) (err error) {
	service := c.microservice.BuildService()
	service.Spec.Type = "ClusterIP"

	deployment := c.microservice.BuildDeployment()
	volumeMounts := []corev1.VolumeMount{
		createConfigVolumeMount(),
	}

	deployment.Spec.Template.Spec.Containers = []corev1.Container{
		corev1.Container{
			Image:   c.microservice.image,
			Name:    consts.UIContainerName,
			EnvFrom: c.getEnvSource(),
			Command: []string{
				"/usr/bin/chyt-controller",
				"--config-path",
				path.Join(consts.ConfigMountPoint, ChytControllerConfigFileName),
				"--log-to-stderr",
				"run",
			},
			VolumeMounts: volumeMounts,
		},
	}

	deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
		createConfigVolume(c.labeller.GetMainConfigMapName(), nil),
	}

	return c.microservice.Sync(ctx)
}

func (c *chytController) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error
	if c.master.Status(ctx) != SyncStatusReady {
		return SyncStatusBlocked, err
	}

	if c.dataNode.Status(ctx) != SyncStatusReady {
		return SyncStatusBlocked, err
	}

	if c.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			s := c.secret.Build()
			s.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = c.secret.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	if !dry {
		c.initUserJob.SetInitScript(c.createInitUserScript())
	}
	status, err := c.initUserJob.Sync(ctx, dry)
	if err != nil || status != SyncStatusReady {
		return status, err
	}

	if !dry {
		c.prepareInitClusterJob()
	}
	status, err = c.initClusterJob.Sync(ctx, dry)
	if err != nil || status != SyncStatusReady {
		return status, err
	}

	if !c.microservice.IsInSync() {
		if !dry {
			// TODO(psushin): there should be me more sophisticated logic for version updates.
			err = c.syncComponents(ctx)
		}
		return SyncStatusPending, err
	}

	if !dry {
		c.prepareChPublicJob()
	}
	status, err = c.initChPublicJob.Sync(ctx, dry)
	if err != nil || status != SyncStatusReady {
		return status, err
	}

	return SyncStatusReady, err
}

func (c *chytController) Status(ctx context.Context) SyncStatus {
	status, err := c.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (c *chytController) Sync(ctx context.Context) error {
	_, err := c.doSync(ctx, false)
	return err
}
