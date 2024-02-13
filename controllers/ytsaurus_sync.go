package controllers

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
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

	if !resource.Spec.IsManaged {
		logger.Info("Ytsaurus cluster is not managed by controller, do nothing")
		return requeueLater, nil
	}

	ytsaurusProxy := apiProxy.NewYtsaurus(resource, r.Client, r.Recorder, r.Scheme)
	ytsaurusComponent, err := NewYtsaurus(ctx, ytsaurusProxy)
	if err != nil {
		return requeueASAP, err
	}

	status, err := ytsaurusComponent.Status(ctx)
	if err != nil {
		return requeueASAP, err
	}

	syncStatus := status.SyncStatus
	var newClusterState ytv1.ClusterState
	var requeue ctrl.Result

	switch syncStatus {
	case components.SyncStatusReady:
		logger.Info("YTsaurus is running and happy")
		newClusterState = ytv1.ClusterStateRunning
		requeue = requeueNot
	case components.SyncStatusBlocked:
		logger.Info("YTsaurus is not in sync, but update is blocked. Human is needed.")
		newClusterState = ytv1.ClusterStateCancelUpdate
		requeue = requeueLater
	case components.SyncStatusUpdating:
		logger.Info("YTsaurus is updating, but nothing to do currently.")
		newClusterState = ytv1.ClusterStateUpdating
		requeue = requeueSoon
	default:
		// currently here may be NeedFullUpdate or NeedLocalUpdate. At that level it is no difference.
		// also Pending is possible, but I'm not plan to use it.
		logger.Info("YTsaurus is not in sync. Synchronizing.", "status", status)
		err = ytsaurusComponent.Sync(ctx)
		if err != nil {
			logger.Error(err, "failed to sync YTsaurus", "status", status)
		}
		newClusterState = ytv1.ClusterStateUpdating
		requeue = requeueASAP
	}

	err = ytsaurusProxy.SaveClusterState(ctx, newClusterState)
	if err != nil {
		return requeueASAP, err
	}
	return requeue, nil
}
