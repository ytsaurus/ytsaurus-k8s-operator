package controllers

import (
	"context"
	"fmt"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

var (
	requeueNot   = ctrl.Result{Requeue: false}
	requeueASAP  = ctrl.Result{Requeue: true}
	requeueSoon  = ctrl.Result{RequeueAfter: 1 * time.Second}
	requeueLater = ctrl.Result{RequeueAfter: 1 * time.Minute}
)

func (r *YtsaurusReconciler) Sync(ctx context.Context, resource *ytv1.Ytsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !resource.Spec.IsManaged {
		logger.Info("Ytsaurus cluster is not managed by controller, do nothing")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	ytsaurus := apiProxy.NewYtsaurus(resource, r.Client, r.Recorder, r.Scheme)
	compRegistry := buildComponentRegistry(ytsaurus)
	st, err := syncComponents(ctx, compRegistry, ytsaurus.GetResource())
	if err != nil {
		return requeueASAP, fmt.Errorf("failed to sync components: %w", err)
	}

	var requeue ctrl.Result
	var clusterState ytv1.ClusterState

	switch st.SyncStatus {
	case components.SyncStatusReady:
		logger.Info("YTsaurus running and happy")
		requeue = requeueNot
		clusterState = ytv1.ClusterStateRunning
	case components.SyncStatusBlocked:
		logger.Info("Components update is blocked. Human is needed. %s", st.Message)
		requeue = requeueLater
		clusterState = ytv1.ClusterStateCancelUpdate
	default:
		requeue = requeueSoon
		clusterState = ytv1.ClusterStateUpdating
	}

	err = ytsaurus.SaveClusterState(ctx, clusterState)
	if err != nil {
		return requeueASAP, fmt.Errorf("failed to save cluster state to %s", clusterState)
	}
	return requeue, nil
}
