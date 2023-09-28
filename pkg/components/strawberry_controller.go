package components

import (
	"context"
	"fmt"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	corev1 "k8s.io/api/core/v1"
	"path"
	"strings"
)

type strawberryController struct {
	componentBase
	microservice       microservice
	initUserJob        *InitJob
	initChytClusterJob *InitJob
	secret             *resources.StringSecret

	master    Component
	scheduler Component
	dataNodes []Component

	name string
}

func getControllerConfigFileName(name string) string {
	if name == "chyt" {
		return "chyt-controller.yson"
	} else {
		return "strawberry-controller.yson"
	}
}

const ChytInitClusterJobConfigFileName = "chyt-init-cluster.yson"

func NewStrawberryController(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	scheduler Component,
	dataNodes []Component) Component {
	resource := ytsaurus.GetResource()

	image := resource.Spec.CoreImage
	name := "chyt"
	componentName := "ChytController"
	if resource.Spec.DeprecatedChytController != nil {
		if resource.Spec.DeprecatedChytController.Image != nil {
			image = *resource.Spec.DeprecatedChytController.Image
		}
	}
	if resource.Spec.StrawberryController != nil {
		name = "strawberry"
		componentName = "StrawberryController"
		if resource.Spec.StrawberryController.Image != nil {
			image = *resource.Spec.StrawberryController.Image
		}
	}

	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: fmt.Sprintf("yt-%s-controller", name),
		ComponentName:  componentName,
	}

	microservice := newMicroservice(
		&l,
		ytsaurus,
		image,
		1,
		map[string]ytconfig.GeneratorDescriptor{
			getControllerConfigFileName(name): {
				F:   cfgen.GetStrawberryControllerConfig,
				Fmt: ytconfig.ConfigFormatYson,
			},
		},
		fmt.Sprintf("%s-controller", name),
		name)

	return &strawberryController{
		componentBase: componentBase{
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
		initChytClusterJob: NewInitJob(
			&l,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"cluster",
			ChytInitClusterJobConfigFileName,
			image,
			cfgen.GetChytInitClusterConfig),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			&l,
			ytsaurus.APIProxy()),
		name:      name,
		master:    master,
		scheduler: scheduler,
		dataNodes: dataNodes,
	}
}

func (c *strawberryController) IsUpdatable() bool {
	return true
}

func (c *strawberryController) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		c.microservice,
		c.initUserJob,
		c.initChytClusterJob,
		c.secret,
	})
}

func (c *strawberryController) initUsers() string {
	token, _ := c.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand(consts.StrawberryControllerUserName, "", token, true)
	return strings.Join(commands, "\n")
}

func (c *strawberryController) createInitUserScript() string {
	script := []string{
		initJobWithNativeDriverPrologue(),
		c.initUsers(),
	}

	return strings.Join(script, "\n")
}

func (c *strawberryController) createInitChytClusterScript() string {
	script := []string{
		initJobPrologue,
		fmt.Sprintf("/usr/bin/chyt-controller --config-path %s init-cluster",
			path.Join(consts.ConfigMountPoint, ChytInitClusterJobConfigFileName)),
	}

	return strings.Join(script, "\n")
}

func (c *strawberryController) getEnvSource() []corev1.EnvFromSource {
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

func (c *strawberryController) prepareInitChytClusterJob() {
	c.initChytClusterJob.SetInitScript(c.createInitChytClusterScript())

	job := c.initChytClusterJob.Build()
	container := &job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []corev1.EnvFromSource{c.secret.GetEnvSource()}
}

func (c *strawberryController) syncComponents(ctx context.Context) (err error) {
	service := c.microservice.buildService()
	service.Spec.Type = "ClusterIP"

	deployment := c.microservice.buildDeployment()
	volumeMounts := []corev1.VolumeMount{
		createConfigVolumeMount(),
	}

	deployment.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Image:   c.microservice.getImage(),
			Name:    consts.UIContainerName,
			EnvFrom: c.getEnvSource(),
			Command: []string{
				"/usr/bin/chyt-controller",
				"--config-path",
				path.Join(consts.ConfigMountPoint, getControllerConfigFileName(c.name)),
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

func (c *strawberryController) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if c.ytsaurus.GetClusterState() == ytv1.ClusterStateRunning && c.microservice.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if c.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(c.ytsaurus, c) {
			if c.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
				if !dry {
					err = removePods(ctx, c.microservice, &c.componentBase)
				}
				return WaitingStatus(SyncStatusUpdating, "pods removal"), err
			}

			if c.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation {
				return NewComponentStatus(SyncStatusReady, "Nothing to do now"), err
			}
		} else {
			return NewComponentStatus(SyncStatusReady, "Not updating component"), err
		}
	}

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
		c.prepareInitChytClusterJob()
	}
	status, err = c.initChytClusterJob.Sync(ctx, dry)
	if err != nil || status.SyncStatus != SyncStatusReady {
		return status, err
	}

	if c.microservice.needSync() {
		if !dry {
			// TODO(psushin): there should be me more sophisticated logic for version updates.
			err = c.syncComponents(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (c *strawberryController) Status(ctx context.Context) ComponentStatus {
	status, err := c.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (c *strawberryController) Sync(ctx context.Context) error {
	_, err := c.doSync(ctx, false)
	return err
}
