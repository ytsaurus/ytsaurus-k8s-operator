package components

import (
	"context"
	"fmt"
	"path"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type UI struct {
	localComponent
	cfgen        *ytconfig.Generator
	microservice microservice
	initJob      *InitJob
	master       Component
	secret       *resources.StringSecret
	caBundle     *resources.CABundle
}

const UIClustersConfigFileName = "clusters-config.json"
const UICustomConfigFileName = "common.js"

func NewUI(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, master Component) *UI {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:    &resource.ObjectMeta,
		APIProxy:      ytsaurus.APIProxy(),
		ComponentType: consts.UIType,
		Annotations:   resource.Spec.ExtraPodAnnotations,
	}
	image := resource.Spec.UIImage
	if resource.Spec.UI.Image != nil {
		image = *resource.Spec.UI.Image
	}

	var caBundle *resources.CABundle
	if caBundleSpec := resource.Spec.CABundle; caBundleSpec != nil {
		caBundle = resources.NewCABundle(caBundleSpec.Name, consts.CABundleVolumeName, consts.CABundleMountPoint)
	}

	microservice := newMicroservice(
		&l,
		ytsaurus,
		image,
		resource.Spec.UI.InstanceCount,
		map[string]ytconfig.GeneratorDescriptor{
			UIClustersConfigFileName: {
				F:   cfgen.GetUIClustersConfig,
				Fmt: ytconfig.ConfigFormatJson,
			},
			UICustomConfigFileName: {
				F:   cfgen.GetUICustomConfig,
				Fmt: ytconfig.ConfigFormatJsonWithJsPrologue,
			},
		},
		"ytsaurus-ui-deployment",
		"ytsaurus-ui",
		resource.Spec.UI.Tolerations,
		resource.Spec.UI.NodeSelector,
	)

	microservice.getHttpService().SetHttpNodePort(resource.Spec.UI.HttpNodePort)

	return &UI{
		localComponent: newLocalComponent(&l, ytsaurus),
		cfgen:          cfgen,
		microservice:   microservice,
		initJob: NewInitJob(
			&l,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"default",
			consts.ClientConfigFileName,
			getImageWithDefault(resource.Spec.UI.Image, resource.Spec.CoreImage),
			cfgen.GetNativeClientConfig,
			getTolerationsWithDefault(resource.Spec.UI.Tolerations, resource.Spec.Tolerations),
			getNodeSelectorWithDefault(resource.Spec.UI.NodeSelector, resource.Spec.NodeSelector),
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			&l,
			ytsaurus.APIProxy()),
		caBundle: caBundle,
		master:   master,
	}
}

func (u *UI) IsUpdatable() bool {
	return true
}

func (u *UI) GetType() consts.ComponentType { return consts.UIType }

func (u *UI) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		u.microservice,
		u.initJob,
		u.secret,
	)
}

func (u *UI) initUser() string {
	token, _ := u.secret.GetValue(consts.TokenSecretKey)
	commands := createUserCommand(consts.UIUserName, "", token, false)
	return strings.Join(commands, "\n")
}

func (u *UI) createInitScript() string {
	script := []string{
		initJobWithNativeDriverPrologue(),
		u.initUser(),
	}

	return strings.Join(script, "\n")
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

	if u.caBundle != nil {
		env = append(env, corev1.EnvVar{
			Name:  "NODE_EXTRA_CA_CERTS",
			Value: fmt.Sprintf("%s/ca.crt", u.caBundle.MountPath),
		})
	}

	env = append(env, ytsaurusResource.Spec.UI.ExtraEnvVariables...)

	secretsVolumeSize, _ := resource.ParseQuantity("1Mi")
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
					Name:          "http",
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

	if u.caBundle != nil {
		u.caBundle.AddVolume(&deployment.Spec.Template.Spec)
		for i := range deployment.Spec.Template.Spec.Containers {
			u.caBundle.AddVolumeMount(&deployment.Spec.Template.Spec.Containers[i])
		}
	}

	return u.microservice.Sync(ctx)
}

func (u *UI) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if u.ytsaurus.GetClusterState() == ytv1.ClusterStateRunning && u.microservice.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if u.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(u.ytsaurus, u) {
			if u.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
				if !dry {
					err = removePods(ctx, u.microservice, &u.localComponent)
				}
				return WaitingStatus(SyncStatusUpdating, "pods removal"), err
			}

			if u.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation {
				return NewComponentStatus(SyncStatusReady, "Nothing to do now"), err
			}
		} else {
			return NewComponentStatus(SyncStatusReady, "Not updating component"), err
		}
	}

	masterStatus, err := u.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, u.master.GetName()), err
	}

	if u.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			token := ytconfig.RandString(30)
			s := u.secret.Build()
			s.StringData = map[string]string{
				consts.UISecretFileName: fmt.Sprintf("{\"oauthToken\" : \"%s\"}", token),
				consts.TokenSecretKey:   token,
			}
			err = u.secret.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, u.secret.Name()), err
	}

	if !dry {
		u.initJob.SetInitScript(u.createInitScript())
	}
	status, err := u.initJob.Sync(ctx, dry)
	if err != nil || status.SyncStatus != SyncStatusReady {
		return status, err
	}

	if u.microservice.needSync() {
		if !dry {
			err = u.syncComponents(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !u.microservice.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusPending, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (u *UI) Status(ctx context.Context) (ComponentStatus, error) {
	return u.doSync(ctx, true)
}

func (u *UI) Sync(ctx context.Context) error {
	_, err := u.doSync(ctx, false)
	return err
}
