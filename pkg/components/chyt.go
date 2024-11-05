package components

import (
	"context"
	"fmt"
	"strings"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type Chyt struct {
	localComponent

	chyt  *apiproxy.Chyt
	cfgen *ytconfig.Generator

	secret *resources.StringSecret

	initUser        *InitJob
	initEnvironment *InitJob
	initChPublicJob *InitJob
}

var _ LocalComponent = &Chyt{}

func NewChyt(
	cfgen *ytconfig.Generator,
	chyt *apiproxy.Chyt,
	ytsaurusApi *apiproxy.Ytsaurus) *Chyt {
	ytsaurus := ytsaurusApi.Resource()

	l := labeller.Labeller{
		ObjectMeta:        &chyt.Resource().ObjectMeta,
		ComponentType:     consts.ChytType,
		ComponentNamePart: chyt.Resource().Name,
		Annotations:       ytsaurus.Spec.ExtraPodAnnotations,
	}

	return &Chyt{
		localComponent: newLocalComponent(&l, ytsaurusApi),
		chyt:           chyt,
		cfgen:          cfgen,
		initUser: NewInitJob(
			&l,
			chyt.APIProxy(),
			chyt,
			ytsaurus.Spec.ImagePullSecrets,
			"user",
			consts.ClientConfigFileName,
			ytsaurus.Spec.CoreImage,
			cfgen.GetNativeClientConfig,
			ytsaurus.Spec.Tolerations,
			ytsaurus.Spec.NodeSelector,
		),
		initEnvironment: NewInitJob(
			&l,
			chyt.APIProxy(),
			chyt,
			ytsaurus.Spec.ImagePullSecrets,
			"release",
			consts.ClientConfigFileName,
			chyt.Resource().Spec.Image,
			cfgen.GetNativeClientConfig,
			ytsaurus.Spec.Tolerations,
			ytsaurus.Spec.NodeSelector,
		),
		initChPublicJob: NewInitJob(
			&l,
			chyt.APIProxy(),
			chyt,
			ytsaurus.Spec.ImagePullSecrets,
			"ch-public",
			consts.ClientConfigFileName,
			chyt.Resource().Spec.Image,
			cfgen.GetNativeClientConfig,
			ytsaurus.Spec.Tolerations,
			ytsaurus.Spec.NodeSelector,
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			&l,
			chyt.APIProxy()),
	}
}

func (c *Chyt) createInitUserScript() string {
	token, _ := c.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand("chyt_releaser", "", token, true)
	script := []string{
		initJobWithNativeDriverPrologue(),
	}
	script = append(script, commands...)

	return strings.Join(script, "\n")
}

func (c *Chyt) createInitScript() string {
	script := "/setup_cluster_for_chyt.sh"

	if c.chyt.Resource().Spec.MakeDefault {
		script += " --make-default"
	}

	return script
}

func (c *Chyt) createInitChPublicScript() string {
	script := []string{
		initJobPrologue,
		fmt.Sprintf("export YT_PROXY=%v CHYT_CTL_ADDRESS=%v YT_LOG_LEVEL=debug", c.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole), c.cfgen.GetStrawberryControllerServiceAddress()),
		"yt create scheduler_pool --attributes '{name=chyt; pool_tree=default}' --ignore-existing",
		"yt clickhouse ctl create ch_public || true",
		"yt clickhouse ctl set-option --alias ch_public pool chyt",
		"yt clickhouse ctl set-option --alias ch_public enable_geodata '%false'",
		"yt clickhouse ctl set-option --alias ch_public instance_cpu 1",
		"yt clickhouse ctl set-option --alias ch_public instance_memory '{reader=100000000;chunk_meta_cache=100000000;compressed_cache=100000000;clickhouse=100000000;clickhouse_watermark=10;footprint=500000000;log_tailer=100000000;watchdog_oom_watermark=0;watchdog_oom_window_watermark=0}'",
		"yt clickhouse ctl set-option --alias ch_public instance_count 1",
		"yt clickhouse ctl start ch_public",
	}

	return strings.Join(script, "\n")
}

func (c *Chyt) prepareChPublicJob() {
	c.initChPublicJob.SetInitScript(c.createInitChPublicScript())

	job := c.initChPublicJob.Build()
	container := &job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []corev1.EnvFromSource{c.secret.GetEnvSource()}
}

func (c *Chyt) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if c.ytsaurus.GetClusterState() != ytv1.ClusterStateRunning {
		return WaitingStatus(SyncStatusBlocked, "ytsaurus running"), err
	}

	// Create a user for chyt initialization.
	if c.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			secretSpec := c.secret.Build()
			secretSpec.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = c.secret.Sync(ctx)
		}
		c.chyt.Resource().Status.ReleaseStatus = ytv1.ChytReleaseStatusCreatingUserSecret
		return WaitingStatus(SyncStatusPending, c.secret.Name()), err
	}

	if !dry {
		c.initUser.SetInitScript(c.createInitUserScript())
	}

	status, err := c.initUser.Sync(ctx, dry)
	if status.SyncStatus != SyncStatusReady {
		c.chyt.Resource().Status.ReleaseStatus = ytv1.ChytReleaseStatusCreatingUser
		return status, err
	}

	if !dry {
		c.initEnvironment.SetInitScript(c.createInitScript())
		job := c.initEnvironment.Build()
		container := &job.Spec.Template.Spec.Containers[0]
		token, _ := c.secret.GetValue(consts.TokenSecretKey)
		container.Env = []corev1.EnvVar{
			{
				Name:  "YT_PROXY",
				Value: c.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole),
			},
			{
				Name:  "YT_TOKEN",
				Value: token,
			},
		}
	}

	status, err = c.initEnvironment.Sync(ctx, dry)
	if err != nil || status.SyncStatus != SyncStatusReady {
		c.chyt.Resource().Status.ReleaseStatus = ytv1.ChytReleaseStatusUploadingIntoCypress
		return status, err
	}

	createPublicClique := c.chyt.Resource().Spec.CreatePublicClique
	if c.ytsaurus.Resource().Spec.StrawberryController != nil && createPublicClique != nil && *createPublicClique {
		if !dry {
			c.prepareChPublicJob()
		}
		status, err = c.initChPublicJob.Sync(ctx, dry)
		if err != nil || status.SyncStatus != SyncStatusReady {
			c.chyt.Resource().Status.ReleaseStatus = ytv1.ChytReleaseStatusCreatingChPublicClique
			return status, err
		}
	}

	c.chyt.Resource().Status.ReleaseStatus = ytv1.ChytReleaseStatusFinished

	return SimpleStatus(SyncStatusReady), err
}

func (c *Chyt) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		c.initUser,
		c.initEnvironment,
		c.initChPublicJob,
		c.secret,
	)
}
