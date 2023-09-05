package controllers

import (
	"context"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

func (r *ChytReconciler) Sync(ctx context.Context, resource *ytv1.Chyt, ytsaurus *ytv1.Ytsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	chyt := apiproxy.NewChyt(resource, r.Client, r.Recorder, r.Scheme)

	cfgen := ytconfig.NewGenerator(ytsaurus, getClusterDomain(chyt.APIProxy().Client()))

	component := components.NewChyt(cfgen, chyt, ytsaurus)

	err := component.Fetch(ctx)
	if err != nil {
		logger.Error(err, "failed to fetch CHYT status for controller")
		return ctrl.Result{Requeue: true}, err
	}

	status := component.Status(ctx)
	if status.SyncStatus == components.SyncStatusBlocked {
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	if status.SyncStatus == components.SyncStatusReady {
		logger.Info("CHYT initialization finished")

		return ctrl.Result{}, err
	}

	if err := component.Sync(ctx); err != nil {
		logger.Error(err, "component sync failed", "component", "chyt")
		return ctrl.Result{Requeue: true}, err
	}

	if err := chyt.APIProxy().UpdateStatus(ctx); err != nil {
		logger.Error(err, "update chyt status failed")
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{Requeue: true}, nil
}
