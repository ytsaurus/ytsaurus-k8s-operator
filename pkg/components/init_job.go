package components

import (
	"context"
	"fmt"
	"path"
	"strings"

	"k8s.io/utils/ptr"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
)

const initJobPrologue = `#!/bin/bash
set -eux -o pipefail
`

func initJobWithNativeDriverPrologue() string {
	commands := []string{
		initJobPrologue,
		fmt.Sprintf("export YT_DRIVER_CONFIG_PATH=%s", path.Join(consts.ConfigMountPoint, consts.ClientConfigFileName)),
		`export YTSAURUS_VERSION="$(/usr/bin/ytserver-all --version | head -c4)"`,
	}
	return strings.Join(commands, "\n")
}

type InitJob struct {
	component

	commonSpec    *ytv1.CommonSpec
	commonPodSpec *ytv1.PodSpec
	instanceSpec  *ytv1.InstanceSpec

	initJob *resources.Job
	configs *ConfigMapBuilder
	script  func() ([]string, error)

	caRootBundle    *resources.CABundle
	caBundle        *resources.CABundle
	busClientSecret *resources.TLSSecret

	condition string
	reason    string

	builtJob *batchv1.Job
}

func NewInitJob(
	labeller *labeller.Labeller,
	apiProxy apiproxy.APIProxy,
	name string,
	configFileName string,
	generator ConfigGeneratorFunc,
	commonSpec *ytv1.CommonSpec,
	commonPodSpec *ytv1.PodSpec,
	instanceSpec *ytv1.InstanceSpec,
) *InitJob {
	var busClientSecret *resources.TLSSecret

	if transportSpec := commonSpec.NativeTransport; transportSpec != nil {
		if transportSpec.TLSClientSecret != nil {
			busClientSecret = resources.NewTLSSecret(
				transportSpec.TLSClientSecret.Name,
				consts.BusClientSecretVolumeName,
				consts.BusClientSecretMountPoint)
		}
	}

	initJob := &InitJob{
		component: component{
			labeller: labeller,
			owner:    apiProxy,
		},
		commonSpec:    commonSpec,
		commonPodSpec: commonPodSpec,
		instanceSpec:  instanceSpec,
		condition:     labeller.GetInitJobCompletedCondition(name),
		reason:        "InitJobCompleted",
		initJob: resources.NewJob(
			labeller.GetInitJobName(name),
			labeller,
			apiProxy,
		),
		caRootBundle:    resources.NewCARootBundle(commonSpec.CARootBundle),
		caBundle:        resources.NewCABundle(commonSpec.CABundle),
		busClientSecret: busClientSecret,
		configs:         NewConfigMapBuilder(labeller, apiProxy, labeller.GetInitJobConfigMapName(name), nil),
		script:          func() ([]string, error) { return nil, fmt.Errorf("script undefined") },
	}

	initJob.configs.AddGenerator(configFileName, ConfigFormatYson, generator)
	initJob.configs.AddGenerator(
		consts.InitJobScriptFilename,
		ConfigFormatText,
		func() ([]byte, error) {
			if lines, err := initJob.script(); err != nil {
				return nil, err
			} else {
				return []byte(strings.Join(lines, "\n")), nil
			}
		},
	)

	return initJob
}

func NewInitJobForYtsaurus(
	labeller *labeller.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	name, configFileName string,
	generator ConfigGeneratorFunc,
	instanceSpec *ytv1.InstanceSpec,
) *InitJob {
	return NewInitJob(
		labeller,
		ytsaurus,
		name,
		configFileName,
		generator,
		ytsaurus.GetCommonSpec(),
		ytsaurus.GetCommonPodSpec(),
		instanceSpec,
	)
}

func (j *InitJob) IsCompleted() bool {
	condition := j.owner.GetStatusCondition(j.condition)
	return condition != nil && condition.Status == metav1.ConditionTrue && condition.Reason == j.reason
}

func (j *InitJob) SetInitScript(script string) {
	j.script = func() ([]string, error) {
		return []string{script}, nil
	}
}

func (j *InitJob) RunScript(
	ctx context.Context,
	dry bool,
	reason string,
	script func() ([]string, error),
	complete func(),
) (ComponentStatus, error) {
	j.reason = reason
	j.script = script
	if j.IsCompleted() {
		complete()
		return ComponentStatusReadyAfter("Job %v %v completed", j.initJob.Name(), reason), nil
	}
	return j.Sync(ctx, dry)
}

func (j *InitJob) Build() *batchv1.Job {
	if j.builtJob != nil {
		return j.builtJob
	}
	job := j.initJob.Build()
	job.Spec.Template = corev1.PodTemplateSpec{
		ObjectMeta: j.component.labeller.GetInitJobObjectMeta(),
		Spec: corev1.PodSpec{
			ImagePullSecrets: j.commonSpec.ImagePullSecrets,
			Containers: []corev1.Container{
				{
					Image: ptr.Deref(j.instanceSpec.Image, j.commonSpec.CoreImage),
					Name:  "ytsaurus-init",
					Command: []string{
						"/bin/bash",
						"-eux",
						path.Join(consts.ConfigMountPoint, consts.InitJobScriptFilename),
					},
					Env: getDefaultEnv(),
					VolumeMounts: []corev1.VolumeMount{{
						Name:      consts.InitScriptVolumeName,
						MountPath: consts.ConfigMountPoint,
						ReadOnly:  true,
					}},
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				},
			},
			Volumes: []corev1.Volume{{
				Name: consts.InitScriptVolumeName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: j.configs.GetConfigMapName(),
						},
						DefaultMode: ptr.To(int32(0o500)),
					},
				},
			}},
			RestartPolicy:     corev1.RestartPolicyOnFailure,
			Tolerations:       getTolerationsWithDefault(j.instanceSpec.Tolerations, j.commonPodSpec.Tolerations),
			NodeSelector:      getNodeSelectorWithDefault(j.instanceSpec.NodeSelector, j.commonPodSpec.NodeSelector),
			DNSConfig:         ptrDefault(j.instanceSpec.DNSConfig, j.commonPodSpec.DNSConfig),
			SetHostnameAsFQDN: ptr.To(ptr.Deref(j.instanceSpec.SetHostnameAsFQDN, ptr.Deref(j.commonPodSpec.SetHostnameAsFQDN, true))),
		},
	}
	podSpec := &job.Spec.Template.Spec

	if ptr.Deref(j.instanceSpec.HostNetwork, ptr.Deref(j.commonPodSpec.HostNetwork, false)) {
		podSpec.HostNetwork = true
		// https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-policy
		podSpec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}
	podSpec.DNSPolicy = ptr.Deref(j.instanceSpec.DNSPolicy, ptr.Deref(j.commonPodSpec.DNSPolicy, podSpec.DNSPolicy))

	j.caRootBundle.AddVolume(&job.Spec.Template.Spec)
	j.caRootBundle.AddVolumeMount(&job.Spec.Template.Spec.Containers[0])
	j.caRootBundle.AddContainerEnv(&job.Spec.Template.Spec.Containers[0])

	j.caBundle.AddVolume(&job.Spec.Template.Spec)
	j.caBundle.AddVolumeMount(&job.Spec.Template.Spec.Containers[0])

	j.busClientSecret.AddVolume(&job.Spec.Template.Spec)
	j.busClientSecret.AddVolumeMount(&job.Spec.Template.Spec.Containers[0])

	j.builtJob = job
	return job
}

func (j *InitJob) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, j.initJob, j.configs)
}

func (j *InitJob) Exists() bool {
	return resources.Exists(j.initJob, j.configs)
}

func (j *InitJob) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	condition := j.owner.GetStatusCondition(j.condition)
	if condition != nil && condition.Status == metav1.ConditionTrue && condition.Reason == j.reason {
		return ComponentStatusReadyAfter("%s completed", j.initJob.Name()), err
	}

	if !j.Exists() || condition == nil || condition.Reason != j.reason {
		if !dry {
			for j.initJob.Exists() {
				err := j.initJob.DeleteForeground(ctx)
				return ComponentStatusWaitingFor("job %s deleting", j.initJob.Name()), err
			}
			if _, err = j.configs.Build(); err != nil {
				return ComponentStatusWaitingFor("job %s start", j.initJob.Name()), err
			}
			_ = j.Build()
			if err := resources.Sync(ctx, j.configs, j.initJob); err != nil {
				return ComponentStatusWaitingFor("job %s start", j.initJob.Name()), err
			}
			j.owner.SetStatusCondition(metav1.Condition{
				Type:    j.condition,
				Status:  metav1.ConditionFalse,
				Reason:  j.reason,
				Message: "Init job created",
			})
		}
		return ComponentStatusWaitingFor("job %s start", j.initJob.Name()), nil
	}

	if status := j.initJob.Status(); status.Succeeded <= 0 {
		status := ComponentStatusWaitingFor("job %s completion, active: %v, failed: %v", j.initJob.Name(), status.Active, status.Failed)
		if !dry {
			j.owner.SetStatusCondition(metav1.Condition{
				Type:    j.condition,
				Status:  metav1.ConditionFalse,
				Reason:  j.reason,
				Message: status.Message,
			})
		}
		return status, err
	}

	if !dry {
		j.owner.SetStatusCondition(metav1.Condition{
			Type:    j.condition,
			Status:  metav1.ConditionTrue,
			Reason:  j.reason,
			Message: "Init job successfully completed",
		})
	}

	return ComponentStatusWaitingFor("setting %s condition", j.condition), err
}

func (j *InitJob) prepareRestart(ctx context.Context, dry bool) error {
	if !dry {
		j.owner.SetStatusCondition(metav1.Condition{
			Type:    j.condition,
			Status:  metav1.ConditionFalse,
			Reason:  "InitJobNeedRestart",
			Message: "Init job needs restart",
		})
	}
	return nil
}

func (j *InitJob) isRestartPrepared() bool {
	return j.owner.IsStatusConditionFalse(j.condition)
}

func (j *InitJob) isRestartCompleted() bool {
	return j.IsCompleted()
}
