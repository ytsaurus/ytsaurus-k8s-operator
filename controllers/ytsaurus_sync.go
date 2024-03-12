package controllers

import (
	"context"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/conditions"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytflow"
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

func (r *YtsaurusReconciler) Sync(ctx context.Context, resource *ytv1.Ytsaurus) (ctrl.Result, error) {
	var err error
	logger := log.FromContext(ctx)

	logger.Info(">>> Start reconciliation loop")
	defer logger.Info("<<< Finish reconciliation loop")

	if !resource.Spec.IsManaged {
		logger.Info("Ytsaurus cluster is not managed by controller, do nothing")
		return requeueLater, nil
	}

	ytsaurus := apiProxy.NewYtsaurus(resource, r.Client, r.Recorder, r.Scheme)
	clusterDomain := getClusterDomain(r.Client)
	conds := conditions.NewConditionManager(r.Client, resource)
	flowStatus, err := ytflow.Advance(ctx, ytsaurus, clusterDomain, conds)

	if err != nil {
		logger.Error(err, "failed to advance run ytsaurus flow")
		// This is most likely non-recoverable configuration error.
		return requeueLater, err
	}

	var requeue ctrl.Result
	var clusterState ytv1.ClusterState
	switch flowStatus {
	case ytflow.FlowStatusDone:
		logger.Info("YTsaurus is running and happy.")
		requeue = requeueNot
		clusterState = ytv1.ClusterStateRunning
	case ytflow.FlowStatusUpdating:
		logger.Info("YTsaurus is updating.")
		requeue = requeueSoon
		clusterState = ytv1.ClusterStateUpdating
	case ytflow.FlowStatusBlocked:
		logger.Info("YTsaurus is not in sync, but update is blocked. Human is needed.")
		requeue = requeueLater
		clusterState = ytv1.ClusterStateCancelUpdate
	default:
		return requeueLater, fmt.Errorf("unexpected ytsaurus flow status: %s", flowStatus)
	}

	// TODO: Make cond manager support more fields to update.
	err = conds.UpdateStatusRetryOnConflict(
		ctx,
		func(ytsaurusResource *ytv1.Ytsaurus) {
			ytsaurusResource.Status.State = clusterState
		},
	)
	if err != nil {
		logger.Error(err, "failed to save cluster state", "state", clusterState)
		return requeueASAP, err
	}
	return requeue, nil
}
