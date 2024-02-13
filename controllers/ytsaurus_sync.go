package controllers

import (
	"context"
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
	if !status.IsReady() {
		err = ytsaurusProxy.SaveClusterState(ctx, ytv1.ClusterStateUpdating)
		if err != nil {
			return requeueASAP, err
		}
		// TODO: maybe we need a different behaviour for different statuses
		// TODO: maybe we need a different requeue time for different statuses
		err = ytsaurusComponent.Sync(ctx)
		if err != nil {
			logger.Error(err, "failed to sync ytsaurus")
		}
		return requeueSoon, err
	}

	err = ytsaurusProxy.SaveClusterState(ctx, ytv1.ClusterStateRunning)
	if err != nil {
		return requeueASAP, err
	}
	logger.Info("Ytsaurus is running and happy")
	return requeueNot, nil
}
