package components

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const initJobPrologue = `
set -e
set -x
`

func initJobWithNativeDriverPrologue() string {
	commands := []string{
		initJobPrologue,
		fmt.Sprintf("export YT_DRIVER_CONFIG_PATH=%s", path.Join(consts.ConfigMountPoint, consts.ClientConfigFileName)),
	}
	return strings.Join(commands, "\n")
}

type InitJob struct {
	ComponentBase
	apiProxy *apiproxy.APIProxy

	initJob *resources.Job

	configHelper *ConfigHelper
	condition    string

	builtJob *batchv1.Job
}

func NewInitJob(
	labeller *labeller.Labeller,
	apiProxy *apiproxy.APIProxy,
	name, configFileName string,
	generator ytconfig.GeneratorFunc) *InitJob {
	return &InitJob{
		ComponentBase: ComponentBase{
			labeller: labeller,
		},
		apiProxy:  apiProxy,
		condition: fmt.Sprintf("%s%sInitJobCompleted", name, labeller.ComponentName),
		initJob: resources.NewJob(
			labeller.GetInitJobName(name),
			labeller,
			apiProxy),
		configHelper: NewConfigHelper(
			labeller,
			apiProxy,
			fmt.Sprintf(
				"%s-%s-init-job-config",
				strings.ToLower(name),
				labeller.ComponentLabel),
			configFileName,
			generator),
	}
}

func (j *InitJob) SetInitScript(script string) {
	cm := j.configHelper.Build()
	cm.BinaryData[consts.InitClusterScriptFileName] = []byte(script)
}

func (j *InitJob) Build() *batchv1.Job {
	if j.builtJob != nil {
		return j.builtJob
	}
	var defaultMode int32 = 0500
	job := j.initJob.Build()
	job.Spec.Template = corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			ImagePullSecrets: j.apiProxy.Ytsaurus().Spec.ImagePullSecrets,
			Containers: []corev1.Container{
				{
					Image:   j.labeller.Ytsaurus.Spec.CoreImage,
					Name:    "ytsaurus-init",
					Command: []string{"bash", "-c", path.Join(consts.ConfigMountPoint, consts.InitClusterScriptFileName)},
					VolumeMounts: []corev1.VolumeMount{
						createConfigVolumeMount(),
					},
				},
			},
			Volumes: []corev1.Volume{
				createConfigVolume(j.configHelper.GetConfigMapName(), &defaultMode),
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
		},
	}
	j.builtJob = job
	return job
}

func (j *InitJob) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		j.initJob,
		j.configHelper,
	})
}

func (j *InitJob) Sync(ctx context.Context, dry bool) (SyncStatus, error) {
	logger := log.FromContext(ctx)

	var err error
	if j.apiProxy.IsStatusConditionTrue(j.condition) {
		return SyncStatusReady, err
	}

	// Deal with init job.
	if !resources.Exists(j.initJob) {
		if !dry {
			_ = j.Build()
			resources.Sync(ctx, []resources.Syncable{
				j.configHelper,
				j.initJob,
			})
		}
		return SyncStatusPending, err
	}

	if !j.initJob.Completed() {
		logger.Info("Init job isn't completed for " + j.labeller.ComponentName)
		return SyncStatusBlocked, err
	}

	if !dry {
		err = j.apiProxy.SetStatusCondition(ctx, metav1.Condition{
			Type:    j.condition,
			Status:  metav1.ConditionTrue,
			Reason:  "InitJobCompleted",
			Message: "Init job successfully completed",
		})
	}

	return SyncStatusPending, err
}
