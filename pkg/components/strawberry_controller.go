package components

import (
	"context"
	"fmt"
	"path"
	"strings"

	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type StrawberryController struct {
	microserviceComponent

	cfgen              *ytconfig.Generator
	initUserAndUrlJob  *InitJob
	initChytClusterJob *InitJob
	secret             *resources.StringSecret
	caRootBundle       *resources.CABundle
	caBundle           *resources.CABundle
	busClientSecret    *resources.TLSSecret
	busServerSecret    *resources.TLSSecret

	master    Component
	scheduler Component
	dataNodes []Component

	name string
	spec *ytv1.StrawberryControllerSpec
}

const ControllerConfigFileName = "strawberry-controller.yson"
const ChytInitClusterJobConfigFileName = "chyt-init-cluster.yson"

func NewStrawberryController(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	scheduler Component,
	dataNodes []Component,
) *StrawberryController {
	l := cfgen.GetComponentLabeller(consts.StrawberryControllerType, "")

	resource := ytsaurus.GetResource()

	var busClientSecret *resources.TLSSecret
	var busServerSecret *resources.TLSSecret

	if transportSpec := resource.Spec.NativeTransport; transportSpec != nil {
		if transportSpec.TLSSecret != nil {
			busServerSecret = resources.NewTLSSecret(
				transportSpec.TLSSecret.Name,
				consts.BusServerSecretVolumeName,
				consts.BusServerSecretMountPoint)
		}
		if transportSpec.TLSClientSecret != nil {
			busClientSecret = resources.NewTLSSecret(
				transportSpec.TLSClientSecret.Name,
				consts.BusClientSecretVolumeName,
				consts.BusClientSecretMountPoint)
		}
	}

	// TODO: strawberry has a different image and can't be nil/fallback on CoreImage.
	image := ptr.Deref(resource.Spec.StrawberryController.Image, resource.Spec.CoreImage)

	microservice := newMicroservice(
		l,
		ytsaurus,
		image,
		1,
		map[string]ConfigGenerator{
			ControllerConfigFileName: {
				Generator: cfgen.GetStrawberryControllerConfig,
				Format:    ConfigFormatYson,
			},
		},
		"strawberry-controller",
		"strawberry",
		resource.Spec.StrawberryController.Tolerations,
		resource.Spec.StrawberryController.NodeSelector,
	)

	return &StrawberryController{
		microserviceComponent: microserviceComponent{
			component:    newComponent(l, ytsaurus),
			microservice: microservice,
		},

		cfgen: cfgen,
		initUserAndUrlJob: NewInitJobForYtsaurus(
			l,
			ytsaurus,
			"user",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig,
			&ytv1.InstanceSpec{
				PodSpec: resource.Spec.StrawberryController.PodSpec,
			},
		),
		initChytClusterJob: NewInitJobForYtsaurus(
			l,
			ytsaurus,
			"cluster",
			ChytInitClusterJobConfigFileName,
			cfgen.GetStrawberryInitClusterConfig,
			&ytv1.InstanceSpec{
				PodSpec: resource.Spec.StrawberryController.PodSpec,
				Image:   ptr.To(image),
			},
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus),
		caRootBundle:    resources.NewCARootBundle(resource.Spec.CARootBundle),
		caBundle:        resources.NewCABundle(resource.Spec.CABundle),
		busClientSecret: busClientSecret,
		busServerSecret: busServerSecret,
		name:            "strawberry",
		spec:            resource.Spec.StrawberryController,
		master:          master,
		scheduler:       scheduler,
		dataNodes:       dataNodes,
	}
}

func (c *StrawberryController) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, c.microservice, c.initUserAndUrlJob, c.initChytClusterJob, c.secret)
}

func (c *StrawberryController) Exists() bool {
	return resources.Exists(c.microservice, c.initUserAndUrlJob, c.initChytClusterJob, c.secret)
}

func (c *StrawberryController) initUsers() string {
	token, _ := c.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand(consts.StrawberryControllerUserName, "", token, true)
	return strings.Join(commands, "\n")
}

func (c *StrawberryController) createInitUserAndUrlScript() string {
	script := []string{
		initJobWithNativeDriverPrologue(),
		c.initUsers(),
		RunIfNonexistent("//sys/@ui_config", "yt set //sys/@ui_config '{}'"),
		fmt.Sprintf("yt set //sys/@ui_config/chyt_controller_base_url '\"http://%v:%v\"'",
			c.microservice.getHttpService().Name(), consts.StrawberryHTTPAPIPort),
	}

	return strings.Join(script, "\n")
}

func (c *StrawberryController) getCommand(action, config string) []string {
	command := []string{
		"/usr/bin/chyt-controller",
		action,
		"--config-path",
		path.Join(consts.ConfigMountPoint, config),
	}
	if c.spec.LogToStderr {
		command = append(command, "--log-to-stderr")
	}
	return command
}

func (c *StrawberryController) createInitChytClusterScript() string {
	script := []string{
		initJobPrologue,
		strings.Join(c.getCommand("init-cluster", ChytInitClusterJobConfigFileName), " "),
	}

	return strings.Join(script, "\n")
}

func (c *StrawberryController) getEnvSource() []corev1.EnvFromSource {
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

func (c *StrawberryController) prepareInitChytClusterJob() {
	c.initChytClusterJob.SetInitScript(c.createInitChytClusterScript())

	job := c.initChytClusterJob.Build()
	container := &job.Spec.Template.Spec.Containers[0]
	container.EnvFrom = []corev1.EnvFromSource{c.secret.GetEnvSource()}
}

func (c *StrawberryController) syncComponents(ctx context.Context) (err error) {
	service := c.microservice.buildService()
	service.Spec.Type = "ClusterIP"

	deployment := c.microservice.buildDeployment()
	volumeMounts := []corev1.VolumeMount{
		createConfigVolumeMount(),
	}

	deployment.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Image:   c.microservice.getImage(),
			Name:    consts.StrawberryContainerName,
			EnvFrom: c.getEnvSource(),
			Env:     getDefaultEnv(),
			Command: c.getCommand("run", ControllerConfigFileName),
			Ports: []corev1.ContainerPort{
				{
					Name:          consts.HTTPPortName,
					ContainerPort: consts.StrawberryHTTPAPIPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.FromInt32(consts.StrawberryHTTPAPIPort),
					},
				},
			},
			VolumeMounts: volumeMounts,
		},
	}

	deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
		createConfigVolume(consts.ConfigVolumeName, c.labeller.GetMainConfigMapName(), nil),
	}

	c.caRootBundle.AddVolume(&deployment.Spec.Template.Spec)
	c.caRootBundle.AddVolumeMount(&deployment.Spec.Template.Spec.Containers[0])
	c.caRootBundle.AddContainerEnv(&deployment.Spec.Template.Spec.Containers[0])

	// Strawberry forwards native transport certificates via operation secure vault.
	c.caBundle.AddVolume(&deployment.Spec.Template.Spec)
	c.caBundle.AddVolumeMount(&deployment.Spec.Template.Spec.Containers[0])

	c.busClientSecret.AddVolume(&deployment.Spec.Template.Spec)
	c.busClientSecret.AddVolumeMount(&deployment.Spec.Template.Spec.Containers[0])

	c.busServerSecret.AddVolume(&deployment.Spec.Template.Spec)
	c.busServerSecret.AddVolumeMount(&deployment.Spec.Template.Spec.Containers[0])

	return c.microservice.Sync(ctx)
}

func (c *StrawberryController) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if c.ytsaurus.IsReadyToUpdate() && c.NeedUpdate() {
		return SimpleStatus(SyncStatusNeedUpdate), err
	}

	if c.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(c.ytsaurus, c) {
			if c.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
				if !dry {
					err = removePods(ctx, c.microservice, &c.component)
				}
				return ComponentStatusUpdateStep("pods removal"), err
			}

			if c.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation {
				return ComponentStatusReady(), err
			}
		} else {
			return ComponentStatusReadyAfter("Not updating component"), nil
		}
	}

	masterStatus, err := c.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !masterStatus.IsRunning() {
		return ComponentStatusBlockedBy(c.master.GetFullName()), err
	}

	schStatus, err := c.scheduler.Status(ctx)
	if err != nil {
		return schStatus, err
	}
	if !schStatus.IsRunning() {
		return ComponentStatusBlockedBy(c.scheduler.GetFullName()), err
	}

	for _, dataNode := range c.dataNodes {
		dndStatus, err := dataNode.Status(ctx)
		if err != nil {
			return dndStatus, err
		}
		if !dndStatus.IsRunning() {
			return ComponentStatusBlockedBy(dataNode.GetFullName()), err
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
		return ComponentStatusWaitingFor(c.secret.Name()), err
	}

	if !dry {
		c.initUserAndUrlJob.SetInitScript(c.createInitUserAndUrlScript())
	}
	status, err := c.initUserAndUrlJob.Sync(ctx, dry)
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

	if c.NeedSync() {
		if !dry {
			err = c.syncComponents(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	return ComponentStatusReady(), err
}

func (c *StrawberryController) Status(ctx context.Context) (ComponentStatus, error) {
	return c.doSync(ctx, true)
}

func (c *StrawberryController) Sync(ctx context.Context) error {
	_, err := c.doSync(ctx, false)
	return err
}
