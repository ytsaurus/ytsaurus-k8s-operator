package components

import (
	"context"
	"fmt"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	corev1 "k8s.io/api/core/v1"
	"path"
	"strings"
)

type chytController struct {
	ComponentBase
	microservice   *Microservice
	initUserJob    *InitJob
	initClusterJob *InitJob
	secret         *resources.StringSecret

	master    Component
	scheduler Component
	dataNodes []Component
}

const ChytControllerConfigFileName = "chyt-controller.yson"
const ChytInitClusterJobConfigFileName = "chyt-init-cluster.yson"

func NewChytController(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	scheduler Component,
	dataNodes []Component) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: "yt-chyt-controller",
		ComponentName:  "ChytController",
	}

	image := resource.Spec.CoreImage
	if resource.Spec.ChytController.Image != nil {
		image = *resource.Spec.ChytController.Image
	}

	microservice := NewMicroservice(
		&l,
		ytsaurus,
		image,
		1,
		cfgen.GetChytControllerConfig,
		ChytControllerConfigFileName,
		"chyt-controller",
		"chyt")

	return &chytController{
		ComponentBase: ComponentBase{
			labeller: &l,
			ytsaurus: ytsaurus,
			cfgen:    cfgen,
		},
		microservice: microservice,
		initUserJob: NewInitJob(
			&l,
			ytsaurus.APIProxy(),
			ytsaurus,
			ytsaurus.GetResource().Spec.ImagePullSecrets,
			"user",
			consts.ClientConfigFileName,
			resource.Spec.CoreImage,
			cfgen.GetNativeClientConfig),
		initClusterJob: NewInitJob(
			&l,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"cluster",
			ChytInitClusterJobConfigFileName,
			resource.Spec.CoreImage,
			cfgen.GetChytInitClusterConfig),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			&l,
			ytsaurus.APIProxy()),
		master:    master,
		scheduler: scheduler,
		dataNodes: dataNodes,
	}
}

func (c *chytController) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		c.microservice,
		c.initUserJob,
		c.initClusterJob,
		c.secret,
	})
}

func (c *chytController) initUsers() string {
	token, _ := c.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand(consts.ChytControllerUserName, "", token, true)
	return strings.Join(commands, "\n")
}

func (c *chytController) createInitUserScript() string {
	script := []string{
		initJobWithNativeDriverPrologue(),
		c.initUsers(),
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

func (c *chytController) syncComponents(ctx context.Context) (err error) {
	service := c.microservice.BuildService()
	service.Spec.Type = "ClusterIP"

	deployment := c.microservice.BuildDeployment()
	volumeMounts := []corev1.VolumeMount{
		createConfigVolumeMount(),
	}

	deployment.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Image:   c.microservice.image,
			Name:    consts.UIContainerName,
			EnvFrom: c.getEnvSource(),
			Command: []string{
				"/usr/bin/chyt-controller",
				"--config-path",
				path.Join(consts.ConfigMountPoint, ChytControllerConfigFileName),
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

func (c *chytController) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if c.master.Status(ctx).SyncStatus != SyncStatusReady {
		return WaitingStatus(SyncStatusBlocked, c.master.GetName()), err
	}

	if c.scheduler.Status(ctx).SyncStatus != SyncStatusReady {
		return WaitingStatus(SyncStatusBlocked, c.scheduler.GetName()), err
	}

	for _, dataNode := range c.dataNodes {
		if dataNode.Status(ctx).SyncStatus != SyncStatusReady {
			return WaitingStatus(SyncStatusBlocked, dataNode.GetName()), err
		}
	}

	if c.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			s := c.secret.Build()
			s.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = c.secret.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, c.secret.Name()), err
	}

	if !dry {
		c.initUserJob.SetInitScript(c.createInitUserScript())
	}
	status, err := c.initUserJob.Sync(ctx, dry)
	if err != nil || status.SyncStatus != SyncStatusReady {
		return status, err
	}

	if !dry {
		c.prepareInitClusterJob()
	}
	status, err = c.initClusterJob.Sync(ctx, dry)
	if err != nil || status.SyncStatus != SyncStatusReady {
		return status, err
	}

	if c.microservice.NeedSync() {
		if !dry {
			// TODO(psushin): there should be me more sophisticated logic for version updates.
			err = c.syncComponents(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (c *chytController) Status(ctx context.Context) ComponentStatus {
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
