package components

import (
	"context"
	"fmt"
	"path"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type UI struct {
	microserviceComponent

	cfgen        *ytconfig.Generator
	client       *YtsaurusClient
	secret       *resources.StringSecret
	caRootBundle *resources.CABundle
	caBundle     *resources.CABundle
}

const UIClustersConfigFileName = "clusters-config.json"
const UICustomConfigFileName = "common.js"

func NewUI(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	client *YtsaurusClient,
) *UI {
	l := cfgen.GetComponentLabeller(consts.UIType, "")

	resource := ytsaurus.GetResource()
	image := resource.Spec.UIImage
	if resource.Spec.UI.Image != nil {
		image = *resource.Spec.UI.Image
	}

	microservice := newMicroservice(
		l,
		ytsaurus,
		image,
		resource.Spec.UI.InstanceCount,
		map[string]ConfigGenerator{
			UIClustersConfigFileName: {
				Generator: cfgen.GetUIClustersConfig,
				Format:    ConfigFormatJson,
			},
			UICustomConfigFileName: {
				Generator: cfgen.GetUICustomConfig,
				Format:    ConfigFormatJsonWithJsPrologue,
			},
		},
		"ytsaurus-ui-deployment",
		"ytsaurus-ui",
		resource.Spec.UI.Tolerations,
		resource.Spec.UI.NodeSelector,
	)

	microservice.getHttpService().SetHttpNodePort(resource.Spec.UI.HttpNodePort)

	return &UI{
		microserviceComponent: microserviceComponent{
			component:    newComponent(l, ytsaurus),
			microservice: microservice,
		},

		cfgen: cfgen,
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus),
		caRootBundle: resources.NewCARootBundle(resource.Spec.CARootBundle),
		caBundle:     resources.NewCABundle(resource.Spec.CABundle),
		client:       client,
	}
}

func (u *UI) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		u.microservice,
		u.secret,
	)
}

func (u *UI) Exists() bool {
	return resources.Exists(u.microservice, u.secret)
}

func (u *UI) syncComponents(ctx context.Context) (err error) {
	ytsaurusResource := u.ytsaurus.GetResource()
	service := u.microservice.buildService()
	service.Spec.Type = ytsaurusResource.Spec.UI.ServiceType

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      consts.ConfigVolumeName,
			MountPath: path.Join(consts.UIClustersConfigMountPoint, UIClustersConfigFileName),
			SubPath:   UIClustersConfigFileName,
			ReadOnly:  true,
		},
		{
			Name:      consts.ConfigVolumeName,
			MountPath: path.Join(consts.UICustomConfigMountPoint, UICustomConfigFileName),
			SubPath:   UICustomConfigFileName,
			ReadOnly:  true,
		},
		{
			Name:      consts.UIVaultVolumeName,
			MountPath: consts.UIVaultMountPoint,
			ReadOnly:  true,
		},
		{
			Name:      consts.UISecretsVolumeName,
			MountPath: consts.UISecretsMountPoint,
			ReadOnly:  false,
		},
	}

	env := []corev1.EnvVar{
		// Deprecated since v 17.0.0
		{
			Name:  "YT_AUTH_CLUSTER_ID",
			Value: ytsaurusResource.Name,
		},
		{
			Name:  "ALLOW_PASSWORD_AUTH",
			Value: "1",
		},
		{
			Name:  "APP_INSTALLATION",
			Value: "custom",
		},
	}

	if ytsaurusResource.Spec.UI.UseInsecureCookies {
		env = append(env, corev1.EnvVar{
			Name:  "YT_AUTH_ALLOW_INSECURE",
			Value: "1",
		})
	}

	// FIXME(khlebnikov): UI node should have no use for native transport CA certificate.
	// For compatibility add CA bundle unless have CA root bundle.
	caBundle := u.caRootBundle
	if caBundle == nil {
		caBundle = u.caBundle
	}
	if caBundle != nil {
		env = append(env, corev1.EnvVar{
			Name:  "NODE_EXTRA_CA_CERTS",
			Value: path.Join(caBundle.MountPath, caBundle.FileName),
		})
	}

	env = append(env, ytsaurusResource.Spec.UI.ExtraEnvVariables...)
	env = append(env, getDefaultEnv()...)

	secretsVolumeSize := resource.MustParse("1Mi")
	deployment := u.microservice.buildDeployment()
	deployment.Spec.Template.Spec.InitContainers = []corev1.Container{
		{
			Image: u.microservice.getImage(),
			Name:  consts.PrepareSecretContainerName,
			Command: []string{
				"bash",
				"-c",
				fmt.Sprintf("cp %s %s",
					path.Join(consts.UIVaultMountPoint, consts.UISecretFileName),
					consts.UISecretsMountPoint),
			},
			Env:          getDefaultEnv(),
			VolumeMounts: volumeMounts,
		},
	}

	deployment.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Image:   u.microservice.getImage(),
			Name:    consts.UIContainerName,
			Env:     env,
			Command: []string{"supervisord"},
			Ports: []corev1.ContainerPort{
				{
					Name:          consts.HTTPPortName,
					ContainerPort: consts.UIHTTPPort,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path: "/readyz", // there's not such a URL but responses with 302 Found
						Port: intstr.FromInt32(consts.UIHTTPPort),
					},
				},
			},
			VolumeMounts: volumeMounts,
		},
	}

	deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: consts.ConfigVolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: u.labeller.GetMainConfigMapName(),
					},
				},
			},
		},
		{
			Name: consts.UIVaultVolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: u.secret.Name(),
				},
			},
		},
		{
			Name: consts.UISecretsVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: &secretsVolumeSize,
				},
			},
		},
	}

	caBundle.AddVolume(&deployment.Spec.Template.Spec)
	for i := range deployment.Spec.Template.Spec.Containers {
		ct := &deployment.Spec.Template.Spec.Containers[i]
		caBundle.AddVolumeMount(ct)
		// NOTE: Add standard environment variables only in case CA root bundle.
		u.caRootBundle.AddContainerEnv(ct)
	}

	return u.microservice.Sync(ctx)
}

func (u *UI) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if u.ytsaurus.IsReadyToUpdate() && u.NeedUpdate() {
		return SimpleStatus(SyncStatusNeedUpdate), err
	}

	if u.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(u.ytsaurus, u) {
			if u.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
				if !dry {
					err = removePods(ctx, u.microservice, &u.component)
				}
				return ComponentStatusUpdateStep("pods removal"), err
			}

			if u.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation {
				return ComponentStatusReady(), err
			}
		} else {
			return ComponentStatusReadyAfter("Not updating component"), nil
		}
	}

	if status, err := syncUserToken(ctx, u.client, u.secret, consts.UIUserName, "", dry); !status.IsRunning() {
		return status, err
	}

	if u.NeedSync() {
		if !dry {
			err = u.syncComponents(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if !u.microservice.arePodsReady(ctx) {
		return ComponentStatusWaitingFor("pods"), err
	}

	return ComponentStatusReady(), err
}

func (u *UI) Status(ctx context.Context) (ComponentStatus, error) {
	return u.doSync(ctx, true)
}

func (u *UI) Sync(ctx context.Context) error {
	_, err := u.doSync(ctx, false)
	return err
}
