package components

import (
	"context"
	"fmt"
	"path"
	"strings"

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
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
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
	imagePullSecrets  []corev1.LocalObjectReference

	initJob *resources.Job

	caBundle        *resources.CABundle
	busClientSecret *resources.TLSSecret

	configHelper           *ConfigHelper
	initCompletedCondition string

	image        string
	tolerations  []corev1.Toleration
	nodeSelector map[string]string

	builtJob  *batchv1.Job
	dnsConfig *corev1.PodDNSConfig
}

func NewInitJob(
	labeller *labeller.Labeller,
	apiProxy apiproxy.APIProxy,
	conditionsManager apiproxy.ConditionManager,
	imagePullSecrets []corev1.LocalObjectReference,
	name, configFileName, image string,
	generator ytconfig.YsonGeneratorFunc,
	tolerations []corev1.Toleration,
	nodeSelector map[string]string,
	dnsConfig *corev1.PodDNSConfig,
	commonSpec *ytv1.CommonSpec,
) *InitJob {
	var caBundle *resources.CABundle
	var busClientSecret *resources.TLSSecret

	if caBundleSpec := commonSpec.CABundle; caBundleSpec != nil {
		caBundle = resources.NewCABundle(*caBundleSpec, consts.CABundleVolumeName, consts.CABundleMountPoint)
	}

	if transportSpec := commonSpec.NativeTransport; transportSpec != nil {
		if transportSpec.TLSClientSecret != nil {
			busClientSecret = resources.NewTLSSecret(
				transportSpec.TLSClientSecret.Name,
				consts.BusClientSecretVolumeName,
				consts.BusClientSecretMountPoint)
		}
	}

	return &InitJob{
		baseComponent: baseComponent{
			labeller: labeller,
		},
		apiProxy:               apiProxy,
		conditionsManager:      conditionsManager,
		imagePullSecrets:       imagePullSecrets,
		initCompletedCondition: labeller.GetInitJobCompletedCondition(name),
		image:                  image,
		tolerations:            tolerations,
		nodeSelector:           nodeSelector,
		dnsConfig:              dnsConfig,
		initJob: resources.NewJob(
			labeller.GetInitJobName(name),
			labeller,
			apiProxy,
		),
		caBundle:        caBundle,
		busClientSecret: busClientSecret,
		configHelper: NewConfigHelper(
			labeller,
			apiProxy,
			labeller.GetInitJobConfigMapName(name),
			nil,
			map[string]ytconfig.GeneratorDescriptor{
				configFileName: {
					F:   generator,
					Fmt: ytconfig.ConfigFormatYson,
				},
			}),
	}
}

func (j *InitJob) IsCompleted() bool {
	return j.conditionsManager.IsStatusConditionTrue(j.initCompletedCondition)
}

func (j *InitJob) SetInitScript(script string) {
	cm := j.configHelper.Build()
	cm.Data[consts.InitClusterScriptFileName] = script
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
			ImagePullSecrets: j.imagePullSecrets,
			Containers: []corev1.Container{
				{
					Image:   j.image,
					Name:    "ytsaurus-init",
					Command: []string{"bash", "-c", path.Join(consts.ConfigMountPoint, consts.InitClusterScriptFileName)},
					Env:     getDefaultEnv(),
					VolumeMounts: []corev1.VolumeMount{
						createConfigVolumeMount(),
					},
				},
			},
			Volumes: []corev1.Volume{
				createConfigVolume(consts.ConfigVolumeName, j.configHelper.GetConfigMapName(), &defaultMode),
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Tolerations:   j.tolerations,
			NodeSelector:  j.nodeSelector,
			DNSConfig:     j.dnsConfig,
		},
	}

	if j.caBundle != nil {
		j.caBundle.AddVolume(&job.Spec.Template.Spec)
		j.caBundle.AddVolumeMount(&job.Spec.Template.Spec.Containers[0])
	}

	if j.busClientSecret != nil {
		j.busClientSecret.AddVolume(&job.Spec.Template.Spec)
		j.busClientSecret.AddVolumeMount(&job.Spec.Template.Spec.Containers[0])
	}

	j.builtJob = job
	return job
}

func (j *InitJob) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		j.initJob,
		j.configHelper,
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
				j.configHelper,
				j.initJob,
			)
		}

		return WaitingStatus(SyncStatusPending, fmt.Sprintf("%s creation", j.initJob.Name())), err
	}

	if !j.initJob.Completed() {
		logger.Info("Init job is not completed for " + j.labeller.GetFullComponentName())
		return WaitingStatus(SyncStatusBlocked, fmt.Sprintf("%s completion", j.initJob.Name())), err
	}

	if !dry {
		j.conditionsManager.SetStatusCondition(metav1.Condition{
			Type:    j.initCompletedCondition,
			Status:  metav1.ConditionTrue,
			Reason:  "InitJobCompleted",
			Message: "Init job successfully completed",
		})
	}

	return WaitingStatus(SyncStatusPending, fmt.Sprintf("setting %s condition", j.initCompletedCondition)), err
}

func (j *InitJob) prepareRestart(ctx context.Context, dry bool) error {
	if dry {
		return nil
	}

	if err := j.configHelper.RemoveIfExists(ctx); err != nil {
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
	configExists := j.configHelper.Exists()
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
