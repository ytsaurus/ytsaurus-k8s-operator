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
	ytsaurusSteps, err := NewYtsaurusSteps(ytsaurusProxy)
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
	switch state {
	case ytv1.ClusterStateRunning:
		logger.Info("YTsaurus is running and happy")
		requeue = requeueNot
	case ytv1.ClusterStateCancelUpdate:
		logger.Info("YTsaurus is not in sync, but update is blocked. Human is needed.")
		requeue = requeueLater
	case ytv1.ClusterStateUpdating:
		logger.Info("YTsaurus is updating, but nothing to do currently.")
		requeue = requeueSoon
	default:
		return requeueLater, fmt.Errorf("unexpected ytsaurus cluster state: %s", state)
	}

	err = ytsaurusProxy.SaveClusterState(ctx, state)
	if err != nil {
		logger.Error(err, "failed to save cluster state", "state", state)
		return requeueASAP, err
	}
	return requeue, nil
}
