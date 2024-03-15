package components

import (
	"context"
	"fmt"
	"path"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
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
	InitJobBase
	conditionsManager      apiproxy.ConditionManager
	initCompletedCondition string
}

func NewInitJob(
	labeller *labeller.Labeller,
	apiProxy apiproxy.APIProxy,
	conditionsManager apiproxy.ConditionManager,
	imagePullSecrets []corev1.LocalObjectReference,
	name, configFileName, image string,
	generator ytconfig.YsonGeneratorFunc) *InitJob {
	return &InitJob{
		conditionsManager:      conditionsManager,
		initCompletedCondition: fmt.Sprintf("%s%sInitJobCompleted", name, labeller.ComponentName),
		InitJobBase: InitJobBase{
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
				},
			),
		},
	}
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
		logger.Info("Init job is not completed for " + j.labeller.ComponentName)
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
