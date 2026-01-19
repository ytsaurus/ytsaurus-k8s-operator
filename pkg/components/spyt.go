package components

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/distribution/reference"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type Spyt struct {
	labeller *labeller.Labeller
	spyt     *apiproxy.Spyt
	cfgen    *ytconfig.NodeGenerator
	ytsaurus *ytv1.Ytsaurus

	secret *resources.StringSecret

	initUser        *InitJob
	initEnvironment *InitJob
}

func NewSpyt(cfgen *ytconfig.NodeGenerator, spyt *apiproxy.Spyt, ytsaurus *ytv1.Ytsaurus) *Spyt {
	l := cfgen.GetComponentLabeller(consts.SpytType, spyt.GetResource().Name)
	return &Spyt{
		labeller: l,
		spyt:     spyt,
		cfgen:    cfgen,
		ytsaurus: ytsaurus,
		initUser: NewInitJob(
			l,
			spyt.APIProxy(),
			spyt,
			ytsaurus.Spec.ImagePullSecrets,
			"user",
			consts.ClientConfigFileName,
			ytsaurus.Spec.CoreImage,
			cfgen.GetNativeClientConfig,
			ytsaurus.Spec.Tolerations,
			ytsaurus.Spec.NodeSelector,
			ytsaurus.Spec.DNSConfig,
			&ytsaurus.Spec.CommonSpec,
		),
		initEnvironment: NewInitJob(
			l,
			spyt.APIProxy(),
			spyt,
			ytsaurus.Spec.ImagePullSecrets,
			"spyt-environment",
			consts.ClientConfigFileName,
			spyt.GetResource().Spec.Image,
			cfgen.GetNativeClientConfig,
			ytsaurus.Spec.Tolerations,
			ytsaurus.Spec.NodeSelector,
			ytsaurus.Spec.DNSConfig,
			&ytsaurus.Spec.CommonSpec,
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			spyt.APIProxy()),
	}
}

func (s *Spyt) createInitUserScript() string {
	token, _ := s.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand("spyt_releaser", "", token, true)
	qtConfigCommand := s.buildQueryTrackerDynamicConfigCommand()
	if qtConfigCommand != "" {
		commands = append(commands, qtConfigCommand)
	}
	script := []string{
		initJobWithNativeDriverPrologue(),
	}
	script = append(script, commands...)

	return strings.Join(script, "\n")
}

func (s *Spyt) buildQueryTrackerDynamicConfigCommand() string {
	sparkVersions := s.spyt.GetResource().Spec.SparkVersions
	if len(sparkVersions) == 0 {
		return ""
	}
	spytVersion := extractImageTag(s.spyt.GetResource().Spec.Image)
	if spytVersion == "" {
		return ""
	}

	return fmt.Sprintf(
		"/usr/bin/yt create document //sys/query_tracker/config --attributes '{value={}}' --recursive --ignore-existing\n"+
			"/usr/bin/yt set //sys/query_tracker/config '{query_tracker={spyt_connect_engine={spark_version=\"%s\";spyt_version=\"%s\";};}}'",
		sparkVersions[0],
		spytVersion,
	)
}

func extractImageTag(image string) string {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return ""
	}
	if tagged, ok := ref.(reference.Tagged); ok {
		return tagged.Tag()
	}
	return ""
}

func (s *Spyt) createInitScript() string {
	script := []string{
		"/entrypoint.sh",
	}

	return strings.Join(script, "\n")
}

func (s *Spyt) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if s.ytsaurus.Status.State != ytv1.ClusterStateRunning {
		return ComponentStatusBlockedBy(s.ytsaurus.GetName()), err
	}

	if s.spyt.GetResource().Status.ReleaseStatus == ytv1.SpytReleaseStatusFinished {
		return ComponentStatusReady(), err
	}

	// Create user for spyt initialization.
	if s.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			secretSpec := s.secret.Build()
			secretSpec.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = s.secret.Sync(ctx)
		}
		s.spyt.GetResource().Status.ReleaseStatus = ytv1.SpytReleaseStatusCreatingUserSecret
		return ComponentStatusWaitingFor(s.secret.Name()), err
	}

	if !dry {
		s.initUser.SetInitScript(s.createInitUserScript())
	}
	status, err := s.initUser.Sync(ctx, dry)
	if status.SyncStatus != SyncStatusReady {
		s.spyt.GetResource().Status.ReleaseStatus = ytv1.SpytReleaseStatusCreatingUser
		return status, err
	}

	if !dry {
		s.initEnvironment.SetInitScript(s.createInitScript())
		job := s.initEnvironment.Build()
		container := &job.Spec.Template.Spec.Containers[0]
		token, _ := s.secret.GetValue(consts.TokenSecretKey)
		env := []corev1.EnvVar{
			{
				Name:  "YT_PROXY",
				Value: s.cfgen.GetHTTPProxiesAddress(&s.ytsaurus.Spec, consts.DefaultHTTPProxyRole),
			},
			{
				Name:  "YT_TOKEN",
				Value: token,
			},
			{
				Name:  "EXTRA_PUBLISH_CLUSTER_OPTIONS",
				Value: "--ignore-existing",
			},
		}

		if len(s.spyt.GetResource().Spec.SparkVersions) > 0 {
			env = append(env, corev1.EnvVar{
				Name:  "EXTRA_SPARK_VERSIONS",
				Value: strings.Join(s.spyt.GetResource().Spec.SparkVersions, " "),
			})
		}

		container.Env = env
	}

	status, err = s.initEnvironment.Sync(ctx, dry)
	if status.SyncStatus != SyncStatusReady {
		s.spyt.GetResource().Status.ReleaseStatus = ytv1.SpytReleaseStatusUploadingIntoCypress
		return status, err
	}

	s.spyt.GetResource().Status.ReleaseStatus = ytv1.SpytReleaseStatusFinished

	return ComponentStatusReady(), nil
}

func (s *Spyt) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		s.initUser,
		s.initEnvironment,
		s.secret,
	)
}

func (s *Spyt) Status(ctx context.Context) ComponentStatus {
	status, err := s.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (s *Spyt) Sync(ctx context.Context) error {
	_, err := s.doSync(ctx, false)
	return err
}
