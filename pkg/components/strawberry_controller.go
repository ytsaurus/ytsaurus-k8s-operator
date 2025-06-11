package components

import (
	"context"
	"fmt"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type StrawberryController struct {
	localComponent
	cfgen              *ytconfig.Generator
	microservice       microservice
	initUserAndUrlJob  *InitJob
	initChytClusterJob *InitJob
	secret             *resources.StringSecret
	caBundle           *resources.CABundle
	busClientSecret    *resources.TLSSecret
	busServerSecret    *resources.TLSSecret

	master    Component
	scheduler Component
	dataNodes []Component

	name string
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

	// TODO: strawberry has a different image and can't be nil/fallback on CoreImage.
	image := resource.Spec.CoreImage
	if resource.Spec.StrawberryController.Image != nil {
		image = *resource.Spec.StrawberryController.Image
	}

	var caBundle *resources.CABundle
	var busClientSecret *resources.TLSSecret
	var busServerSecret *resources.TLSSecret

	if caBundleSpec := resource.Spec.CABundle; caBundleSpec != nil {
		caBundle = resources.NewCABundle(
			caBundleSpec.Name,
			consts.CABundleVolumeName,
			consts.CABundleMountPoint)
	}

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

	microservice := newMicroservice(
		l,
		ytsaurus,
		image,
		1,
		map[string]ytconfig.GeneratorDescriptor{
			ControllerConfigFileName: {
				F:   cfgen.GetStrawberryControllerConfig,
				Fmt: ytconfig.ConfigFormatYson,
			},
		},
		"strawberry-controller",
		"strawberry",
		resource.Spec.StrawberryController.Tolerations,
		resource.Spec.StrawberryController.NodeSelector,
	)

	return &StrawberryController{
		localComponent: newLocalComponent(l, ytsaurus),
		cfgen:          cfgen,
		microservice:   microservice,
		initUserAndUrlJob: NewInitJob(
			l,
			ytsaurus.APIProxy(),
			ytsaurus,
			ytsaurus.GetResource().Spec.ImagePullSecrets,
			"user",
			consts.ClientConfigFileName,
			resource.Spec.CoreImage,
			cfgen.GetNativeClientConfig,
			getTolerationsWithDefault(resource.Spec.StrawberryController.Tolerations, resource.Spec.Tolerations),
			getNodeSelectorWithDefault(resource.Spec.StrawberryController.NodeSelector, resource.Spec.NodeSelector),
			getDNSConfigWithDefault(resource.Spec.StrawberryController.DNSConfig, resource.Spec.DNSConfig),
			&resource.Spec.CommonSpec,
		),
		initChytClusterJob: NewInitJob(
			l,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"cluster",
			ChytInitClusterJobConfigFileName,
			image,
			cfgen.GetStrawberryInitClusterConfig,
			getTolerationsWithDefault(resource.Spec.StrawberryController.Tolerations, resource.Spec.Tolerations),
			getNodeSelectorWithDefault(resource.Spec.StrawberryController.NodeSelector, resource.Spec.NodeSelector),
			getDNSConfigWithDefault(resource.Spec.StrawberryController.DNSConfig, resource.Spec.DNSConfig),
			&resource.Spec.CommonSpec,
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus.APIProxy()),
		caBundle:        caBundle,
		busClientSecret: busClientSecret,
		busServerSecret: busServerSecret,
		name:            "strawberry",
		master:          master,
		scheduler:       scheduler,
		dataNodes:       dataNodes,
	}
}

func (c *StrawberryController) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		c.microservice,
		c.initUserAndUrlJob,
		c.initChytClusterJob,
		c.secret,
	)
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

func (c *StrawberryController) createInitChytClusterScript() string {
	script := []string{
		initJobPrologue,
		fmt.Sprintf("/usr/bin/chyt-controller --config-path %s init-cluster",
			path.Join(consts.ConfigMountPoint, ChytInitClusterJobConfigFileName)),
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
			Command: []string{
				"/usr/bin/chyt-controller",
				"--config-path",
				path.Join(consts.ConfigMountPoint, ControllerConfigFileName),
				"run",
			},
			Ports: []corev1.ContainerPort{
				{
					Name:          "http",
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

	if c.caBundle != nil {
		c.caBundle.AddVolume(&deployment.Spec.Template.Spec)
		c.caBundle.AddVolumeMount(&deployment.Spec.Template.Spec.Containers[0])
	}

	if c.busClientSecret != nil {
		c.busClientSecret.AddVolume(&deployment.Spec.Template.Spec)
		c.busClientSecret.AddVolumeMount(&deployment.Spec.Template.Spec.Containers[0])
	}

	if c.busServerSecret != nil {
		c.busServerSecret.AddVolume(&deployment.Spec.Template.Spec)
		c.busServerSecret.AddVolumeMount(&deployment.Spec.Template.Spec.Containers[0])
	}

	return c.microservice.Sync(ctx)
}

func (c *StrawberryController) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(c.ytsaurus.GetClusterState()) && c.microservice.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if c.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(c.ytsaurus, c) {
			if c.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
				if !dry {
					err = removePods(ctx, c.microservice, &c.localComponent)
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

	masterStatus, err := c.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, c.master.GetFullName()), err
	}

	schStatus, err := c.scheduler.Status(ctx)
	if err != nil {
		return schStatus, err
	}
	if !IsRunningStatus(schStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, c.scheduler.GetFullName()), err
	}

	for _, dataNode := range c.dataNodes {
		dndStatus, err := dataNode.Status(ctx)
		if err != nil {
			return dndStatus, err
		}
		if !IsRunningStatus(dndStatus.SyncStatus) {
			return WaitingStatus(SyncStatusBlocked, dataNode.GetFullName()), err
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

	if c.microservice.needSync() {
		if !dry {
			err = c.syncComponents(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (c *StrawberryController) Status(ctx context.Context) (ComponentStatus, error) {
	return c.doSync(ctx, true)
}

func (c *StrawberryController) Sync(ctx context.Context) error {
	_, err := c.doSync(ctx, false)
	return err
}
