package components

import (
	"context"
	"fmt"
	"path"
	"strings"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
)

const initJobPrologue = `
set -e
set -x
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
	baseComponent
	apiProxy          apiproxy.APIProxy
	conditionsManager apiproxy.ConditionManager

	commonSpec    *ytv1.CommonSpec
	commonPodSpec *ytv1.PodSpec
	instanceSpec  *ytv1.InstanceSpec

	initJob *resources.Job
	configs *ConfigMapBuilder

	caRootBundle    *resources.CABundle
	caBundle        *resources.CABundle
	busClientSecret *resources.TLSSecret

	initCompletedCondition string

	builtJob *batchv1.Job
}

func NewInitJob(
	labeller *labeller.Labeller,
	apiProxy apiproxy.APIProxy,
	conditionsManager apiproxy.ConditionManager,
	name, configFileName string,
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

	configs := NewConfigMapBuilder(labeller, apiProxy, labeller.GetInitJobConfigMapName(name), nil)
	configs.AddGenerator(configFileName, ConfigFormatYson, generator)

	return &InitJob{
		baseComponent: baseComponent{
			labeller: labeller,
		},
		apiProxy:               apiProxy,
		conditionsManager:      conditionsManager,
		commonSpec:             commonSpec,
		commonPodSpec:          commonPodSpec,
		instanceSpec:           instanceSpec,
		initCompletedCondition: labeller.GetInitJobCompletedCondition(name),
		initJob: resources.NewJob(
			labeller.GetInitJobName(name),
			labeller,
			apiProxy,
		),
		caRootBundle:    resources.NewCARootBundle(commonSpec.CARootBundle),
		caBundle:        resources.NewCABundle(commonSpec.CABundle),
		busClientSecret: busClientSecret,
		configs:         configs,
	}
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
		ytsaurus.APIProxy(),
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
	return j.conditionsManager.IsStatusConditionTrue(j.initCompletedCondition)
}

func (j *InitJob) SetInitScript(script string) {
	if cm, err := j.configs.Build(); err == nil {
		cm.Data[consts.InitClusterScriptFileName] = script
	}
}

func (j *InitJob) Build() *batchv1.Job {
	if j.builtJob != nil {
		return j.builtJob
	}
	var defaultMode int32 = 0o500
	job := j.initJob.Build()
	job.Spec.Template = corev1.PodTemplateSpec{
		ObjectMeta: j.baseComponent.labeller.GetInitJobObjectMeta(),
		Spec: corev1.PodSpec{
			ImagePullSecrets: j.commonSpec.ImagePullSecrets,
			Containers: []corev1.Container{
				{
					Image:   ptr.Deref(j.instanceSpec.Image, j.commonSpec.CoreImage),
					Name:    "ytsaurus-init",
					Command: []string{"bash", "-c", path.Join(consts.ConfigMountPoint, consts.InitClusterScriptFileName)},
					Env:     getDefaultEnv(),
					VolumeMounts: []corev1.VolumeMount{
						createConfigVolumeMount(),
					},
					TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
				},
			},
			Volumes: []corev1.Volume{
				createConfigVolume(consts.ConfigVolumeName, j.configs.GetConfigMapName(), &defaultMode),
			},
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
	return resources.Fetch(ctx,
		j.initJob,
		j.configs,
	)
}

func (j *InitJob) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	logger := log.FromContext(ctx)
	var err error

	if j.conditionsManager.IsStatusConditionTrue(j.initCompletedCondition) {
		return ComponentStatus{
			SyncStatusReady,
			fmt.Sprintf("%s completed", j.initJob.Name()),
		}, err
	}

	// Deal with init job.
	if !resources.Exists(j.initJob) {
		if !dry {
			_ = j.Build()
			err = resources.Sync(ctx,
				j.configs,
				j.initJob,
			)
		}

		return ComponentStatusWaitingFor(fmt.Sprintf("%s creation", j.initJob.Name())), err
	}

	if !j.initJob.Completed() {
		logger.Info("Init job is not completed for " + j.labeller.GetFullComponentName())
		return ComponentStatusBlockedBy(fmt.Sprintf("%s completion", j.initJob.Name())), err
	}

	if !dry {
		j.conditionsManager.SetStatusCondition(metav1.Condition{
			Type:    j.initCompletedCondition,
			Status:  metav1.ConditionTrue,
			Reason:  "InitJobCompleted",
			Message: "Init job successfully completed",
		})
	}

	return ComponentStatusWaitingFor(fmt.Sprintf("setting %s condition", j.initCompletedCondition)), err
}

func (j *InitJob) prepareRestart(ctx context.Context, dry bool) error {
	if dry {
		return nil
	}

	if err := j.configs.RemoveIfExists(ctx); err != nil {
		return err
	}

	if err := j.removeIfExists(ctx); err != nil {
		return err
	}
	j.conditionsManager.SetStatusCondition(metav1.Condition{
		Type:    j.initCompletedCondition,
		Status:  metav1.ConditionFalse,
		Reason:  "InitJobNeedRestart",
		Message: "Init job needs restart",
	})
	return nil
}

func (j *InitJob) isRestartPrepared() bool {
	jobExists := resources.Exists(j.initJob)
	configExists := j.configs.Exists()
	conditionIsFalse := j.conditionsManager.IsStatusConditionFalse(j.initCompletedCondition)
	return !jobExists && !configExists && conditionIsFalse
}

func (j *InitJob) isRestartCompleted() bool {
	return j.conditionsManager.IsStatusConditionTrue(j.initCompletedCondition)
}

func (j *InitJob) removeIfExists(ctx context.Context) error {
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
