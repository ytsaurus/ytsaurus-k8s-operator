package components

import (
	"context"
	"fmt"
	"path"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type JobStateless struct {
	baseComponent
	apiProxy          apiproxy.APIProxy
	conditionsManager apiproxy.ConditionManager
	imagePullSecrets  []corev1.LocalObjectReference

	initJob *resources.Job

	configHelper           *ConfigHelper
	initCompletedCondition string

	image string

	builtJob *batchv1.Job
}

func NewJobStateless(
	labeller *labeller.Labeller,
	apiProxy apiproxy.APIProxy,
	imagePullSecrets []corev1.LocalObjectReference,
	name, configFileName, image string,
	generator ytconfig.YsonGeneratorFunc) *JobStateless {
	return &JobStateless{
		baseComponent: baseComponent{
			labeller: labeller,
		},
		apiProxy:         apiProxy,
		imagePullSecrets: imagePullSecrets,
		image:            image,
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
			nil,
			map[string]ytconfig.GeneratorDescriptor{
				configFileName: {
					F:   generator,
					Fmt: ytconfig.ConfigFormatYson,
				},
			}),
	}
}

func (j *JobStateless) SetInitScript(script string) {
	cm := j.configHelper.Build()
	cm.Data[consts.InitClusterScriptFileName] = script
}

func (j *JobStateless) Build() *batchv1.Job {
	if j.builtJob != nil {
		return j.builtJob
	}
	var defaultMode int32 = 0500
	job := j.initJob.Build()
	job.Spec.Template = corev1.PodTemplateSpec{
		ObjectMeta: j.baseComponent.labeller.GetInitJobObjectMeta(),
		Spec: corev1.PodSpec{
			ImagePullSecrets: j.imagePullSecrets,
			Containers: []corev1.Container{
				{
					Image:   j.image,
					Name:    "ytsaurus-job",
					Command: []string{"bash", "-c", path.Join(consts.ConfigMountPoint, consts.InitClusterScriptFileName)},
					VolumeMounts: []corev1.VolumeMount{
						createConfigVolumeMount(),
					},
				},
			},
			Volumes: []corev1.Volume{
				createConfigVolume(consts.ConfigVolumeName, j.configHelper.GetConfigMapName(), &defaultMode),
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
		},
	}
	j.builtJob = job
	return job
}

func (j *JobStateless) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		j.initJob,
		j.configHelper,
	)
}

func (j *JobStateless) IsCompleted() bool {
	return j.initJob.Completed()
}

func (j *JobStateless) removeIfExists(ctx context.Context) error {
	if !resources.Exists(j.initJob) {
		return nil
	}
	propagation := metav1.DeletePropagationForeground
	return j.apiProxy.DeleteObject(
		ctx,
		j.initJob.OldObject(),
		&client.DeleteOptions{PropagationPolicy: &propagation},
	)
}

func (j *JobStateless) Prepare(ctx context.Context) error {
	return j.removeIfExists(ctx)
}

func (j *JobStateless) IsPrepared() bool {
	return !resources.Exists(j.initJob)
}

func (j *JobStateless) Sync(ctx context.Context) error {
	_ = j.Build()
	return resources.Sync(ctx,
		j.configHelper,
		j.initJob,
	)
}
