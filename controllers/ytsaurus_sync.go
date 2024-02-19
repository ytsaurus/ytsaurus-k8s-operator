package controllers

import (
	"context"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
)

var (
	requeueNot = ctrl.Result{Requeue: false}
	// I'm actually not sure that this means now
	// TODO: validate
	requeueASAP     = ctrl.Result{Requeue: true}
	requeueSoon     = ctrl.Result{RequeueAfter: 1 * time.Second}
	requeueBitLater = ctrl.Result{RequeueAfter: 10 * time.Second}
	requeueLater    = ctrl.Result{RequeueAfter: 1 * time.Minute}
)

// SyncNew is responsible for
//   - building main ytsaurus component
//   - checking its status
//   - asking ytsaurus component to sync if it is not ready
//   - requeue reconciliation with correct delay if necessary
//   - updating ytsaurus k8s object main state
//   - logging for humans to understand what is going on
func (r *YtsaurusReconciler) SyncNew(ctx context.Context, resource *ytv1.Ytsaurus) (ctrl.Result, error) {
	var err error
	logger := log.FromContext(ctx)

	logger.Info(">>> Start reconciliation loop")
	defer logger.Info("<<< Finish reconciliation loop")

	if !resource.Spec.IsManaged {
		logger.Info("Ytsaurus cluster is not managed by controller, do nothing")
		return requeueLater, nil
	}

	ytsaurusProxy := apiProxy.NewYtsaurus(resource, r.Client, r.Recorder, r.Scheme)
	cm, err := NewComponentManager(ytsaurusProxy)
	if err != nil {
		return requeueASAP, err
	}

	ytsaurusSteps, err := NewYtsaurusSteps(cm.allComponents, &resource.Status, ytsaurusProxy)
	if err != nil {
		logger.Error(err, "failed to create ytsaurus steps")
		return requeueASAP, err
	}
	state, err := ytsaurusSteps.Sync(ctx)
	if err != nil {
		logger.Error(err, "failed to sync ytsaurus steps")
		return requeueASAP, err
	}

	var requeue ctrl.Result
	var clusterState ytv1.ClusterState
	switch state {
	case StepSyncStatusDone:
		logger.Info("YTsaurus is running and happy.")
		requeue = requeueNot
		clusterState = ytv1.ClusterStateRunning
	case StepSyncStatusUpdating:
		logger.Info("YTsaurus is updating.")
		requeue = requeueSoon
		clusterState = ytv1.ClusterStateUpdating
	case StepSyncStatusBlocked:
		logger.Info("YTsaurus is not in sync, but update is blocked. Human is needed.")
		requeue = requeueLater
		clusterState = ytv1.ClusterStateCancelUpdate
	default:
		return requeueLater, fmt.Errorf("unexpected ytsaurus cluster state: %s", state)
	}

	err = ytsaurusProxy.SaveClusterState(ctx, clusterState)
	if err != nil {
		logger.Error(err, "failed to save cluster state", "state", state)
		return requeueASAP, err
	}
	return requeue, nil
}
