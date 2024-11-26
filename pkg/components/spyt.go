package components

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type Spyt struct {
	labeller *labeller.Labeller
	spyt     *apiproxy.Spyt
	cfgen    *ytconfig.Generator
	ytsaurus *ytv1.Ytsaurus

	secret *resources.StringSecret

	initUser        *InitJob
	initEnvironment *InitJob
}

func NewSpyt(
	cfgen *ytconfig.Generator,
	spyt *apiproxy.Spyt,
	ytsaurus *ytv1.Ytsaurus) *Spyt {
	l := labeller.Labeller{
		ObjectMeta:        &spyt.Resource().ObjectMeta,
		ComponentType:     consts.SpytType,
		ComponentNamePart: spyt.Resource().Name,
		Annotations:       ytsaurus.Spec.ExtraPodAnnotations,
	}

	return &Spyt{
		labeller: &l,
		spyt:     spyt,
		cfgen:    cfgen,
		ytsaurus: ytsaurus,
		initUser: NewInitJob(
			&l,
			spyt.APIProxy(),
			spyt,
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
			spyt.APIProxy(),
			spyt,
			ytsaurus.Spec.ImagePullSecrets,
			"spyt-environment",
			consts.ClientConfigFileName,
			spyt.Resource().Spec.Image,
			cfgen.GetNativeClientConfig,
			ytsaurus.Spec.Tolerations,
			ytsaurus.Spec.NodeSelector,
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			&l,
			spyt.APIProxy()),
	}
}

func (s *Spyt) createInitUserScript() string {
	token, _ := s.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand("spyt_releaser", "", token, true)
	script := []string{
		initJobWithNativeDriverPrologue(),
	}
	script = append(script, commands...)

	return strings.Join(script, "\n")
}

func (s *Spyt) createInitScript() string {
	script := []string{
		"/entrypoint.sh",
	}

	return strings.Join(script, "\n")
}

func (s *Spyt) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if s.ytsaurus.Status.State != ytv1.ClusterStateRunning {
		return WaitingStatus(SyncStatusBlocked, s.ytsaurus.GetName()), err
	}

	if s.spyt.Resource().Status.ReleaseStatus == ytv1.SpytReleaseStatusFinished {
		return SimpleStatus(SyncStatusReady), err
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
		s.spyt.Resource().Status.ReleaseStatus = ytv1.SpytReleaseStatusCreatingUserSecret
		return WaitingStatus(SyncStatusPending, s.secret.Name()), err
	}

	if !dry {
		s.initUser.SetInitScript(s.createInitUserScript())
	}
	status, err := s.initUser.Sync(ctx, dry)
	if status.SyncStatus != SyncStatusReady {
		s.spyt.Resource().Status.ReleaseStatus = ytv1.SpytReleaseStatusCreatingUser
		return status, err
	}

	if !dry {
		s.initEnvironment.SetInitScript(s.createInitScript())
		job := s.initEnvironment.Build()
		container := &job.Spec.Template.Spec.Containers[0]
		token, _ := s.secret.GetValue(consts.TokenSecretKey)
		container.Env = []corev1.EnvVar{
			{
				Name:  "YT_PROXY",
				Value: s.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole),
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
	}

	status, err = s.initEnvironment.Sync(ctx, dry)
	if status.SyncStatus != SyncStatusReady {
		s.spyt.Resource().Status.ReleaseStatus = ytv1.SpytReleaseStatusUploadingIntoCypress
		return status, err
	}

	s.spyt.Resource().Status.ReleaseStatus = ytv1.SpytReleaseStatusFinished

	return SimpleStatus(SyncStatusReady), nil
}

func (s *Spyt) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		s.initUser,
		s.initEnvironment,
		s.secret,
	)
}
