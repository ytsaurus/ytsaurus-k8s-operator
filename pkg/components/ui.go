package components

import (
	"context"
	"fmt"
	"path"
	"strings"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type UI struct {
	ComponentBase
	ytsaurus     *ytv1.Ytsaurus
	microservice *Microservice
	initJob      *InitJob
	master       Component
	secret       *resources.StringSecret
}

const UIConfigFileName = "clusters-config.json"

func NewUI(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, master Component) Component {
	resource := ytsaurus.GetResource()
	labeller := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelUI,
		ComponentName:  "UI",
	}

	microservice := NewMicroservice(
		&labeller,
		ytsaurus,
		resource.Spec.UIImage,
		resource.Spec.UI.InstanceCount,
		cfgen.GetWebUIConfig,
		UIConfigFileName,
		"ytsaurus-ui-deployment",
		"ytsaurus-ui")

	return &UI{
		ComponentBase: ComponentBase{
			labeller: &labeller,
			ytsaurus: ytsaurus,
			cfgen:    cfgen,
		},
		microservice: microservice,
		initJob: NewInitJob(
			&labeller,
			ytsaurus.APIProxy(),
			ytsaurus,
			resource.Spec.ImagePullSecrets,
			"default",
			consts.ClientConfigFileName,
			resource.Spec.CoreImage,
			cfgen.GetNativeClientConfig),
		secret: resources.NewStringSecret(
			labeller.GetSecretName(),
			&labeller,
			ytsaurus.APIProxy()),
		ytsaurus: resource,
		master:   master,
	}
}

func (u *UI) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		u.microservice,
		u.initJob,
		u.secret,
	})
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
	service := u.microservice.BuildService()
	service.Spec.Type = u.ytsaurus.Spec.UI.ServiceType

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      consts.ConfigVolumeName,
			MountPath: path.Join(consts.UIConfigMountPoint, UIConfigFileName),
			SubPath:   UIConfigFileName,
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
		{
			Name:  "YT_AUTH_CLUSTER_ID",
			Value: u.ytsaurus.Name,
		},
	}

	if u.ytsaurus.Spec.UI.UseMetrikaCounter {
		config := u.microservice.BuildConfig()
		config.Data[consts.MetrikaCounterFileName] = consts.MetrikaCounterScript

		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      consts.ConfigVolumeName,
				MountPath: path.Join("/opt/app/dist/server/configs/custom", consts.MetrikaCounterFileName),
				SubPath:   consts.MetrikaCounterFileName,
				ReadOnly:  true,
			})
		env = append(env, corev1.EnvVar{
			Name:  "APP_INSTALLATION",
			Value: "custom",
		})
	}

	if u.ytsaurus.Spec.UI.UseInsecureCookies {
		env = append(env, corev1.EnvVar{
			Name:  "YT_AUTH_ALLOW_INSECURE",
			Value: "1",
		})
	}

	secretsVolumeSize, _ := resource.ParseQuantity("1Mi")
	deployment := u.microservice.BuildDeployment()
	deployment.Spec.Template.Spec.InitContainers = []corev1.Container{
		corev1.Container{
			Image: u.microservice.image,
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
		corev1.Container{
			Image:        u.microservice.image,
			Name:         consts.UIContainerName,
			Env:          env,
			Command:      []string{"supervisord"},
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

	return u.microservice.Sync(ctx)
}

func (u *UI) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error

	if u.master.Status(ctx) != SyncStatusReady {
		return SyncStatusBlocked, err
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
		return SyncStatusPending, err
	}

	if !dry {
		u.initJob.SetInitScript(u.createInitScript())
	}
	status, err := u.initJob.Sync(ctx, dry)
	if err != nil || status != SyncStatusReady {
		return status, err
	}

	if !u.microservice.IsInSync() {
		if !dry {
			err = u.syncComponents(ctx)
		}
		return SyncStatusPending, err
	}

	return SyncStatusReady, err
}

func (u *UI) Status(ctx context.Context) SyncStatus {
	status, err := u.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (u *UI) Sync(ctx context.Context) error {
	_, err := u.doSync(ctx, false)
	return err
}
