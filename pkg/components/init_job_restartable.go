package components

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
)

type InitJobRestartable struct {
	InitJobBase
}

func (j *InitJobRestartable) Status(ctx context.Context) (ComponentStatus, error) {
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
		return WaitingStatus(SyncStatusPending, fmt.Sprintf("%s creation", j.initJob.Name())), err
	}

	if !j.initJob.Completed() {
		logger.Info("Init job is not completed for " + j.labeller.ComponentName)
		return WaitingStatus(SyncStatusBlocked, fmt.Sprintf("%s completion", j.initJob.Name())), err
	}

	return WaitingStatus(SyncStatusPending, fmt.Sprintf("setting %s condition", j.initCompletedCondition)), err
}

func (j *InitJobRestartable) Sync(ctx context.Context) (ComponentStatus, error) {
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
		_ = j.Build()
		err = resources.Sync(ctx,
			j.configHelper,
			j.initJob,
		)

		return WaitingStatus(SyncStatusPending, fmt.Sprintf("%s creation", j.initJob.Name())), err
	}

	if !j.initJob.Completed() {
		logger.Info("Init job is not completed for " + j.labeller.ComponentName)
		return WaitingStatus(SyncStatusBlocked, fmt.Sprintf("%s completion", j.initJob.Name())), err
	}

	j.conditionsManager.SetStatusCondition(metav1.Condition{
		Type:    j.initCompletedCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "InitJobCompleted",
		Message: "Init job successfully completed",
	})

	return WaitingStatus(SyncStatusPending, fmt.Sprintf("setting %s condition", j.initCompletedCondition)), err
}

//func (j *InitJob) IsCompleted() bool {
//	return j.conditionsManager.IsStatusConditionTrue(j.initCompletedCondition)
//}

//func (j *InitJob) prepareRestart(ctx context.Context, dry bool) error {
//	if dry {
//		return nil
//	}
//	if err := j.removeIfExists(ctx); err != nil {
//		return err
//	}
//	j.conditionsManager.SetStatusCondition(metav1.Condition{
//		Type:    j.initCompletedCondition,
//		Status:  metav1.ConditionFalse,
//		Reason:  "InitJobNeedRestart",
//		Message: "Init job needs restart",
//	})
//	return nil
//}

//func (j *InitJob) isRestartPrepared() bool {
//	return !resources.Exists(j.initJob) && j.conditionsManager.IsStatusConditionFalse(j.initCompletedCondition)
//}

//func (j *InitJob) isRestartCompleted() bool {
//	return j.conditionsManager.IsStatusConditionTrue(j.initCompletedCondition)
//}
