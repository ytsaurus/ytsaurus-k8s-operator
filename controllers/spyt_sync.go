package controllers

import (
	"context"
	"time"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *SpytReconciler) Sync(ctx context.Context, resource *ytv1.Spyt, ytsaurus *ytv1.Ytsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if r.ShouldIgnoreResource(ctx, resource) {
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	spyt := apiproxy.NewSpyt(resource, r.Client, r.Recorder, r.Scheme)

	cfgen := ytconfig.NewLocalNodeGenerator(ytsaurus, resource.Name, r.ClusterDomain)

	component := components.NewSpyt(cfgen, spyt, ytsaurus)

	err := component.Fetch(ctx)
	if err != nil {
		logger.Error(err, "failed to fetch SPYT status for controller")
		return ctrl.Result{Requeue: true}, err
	}

	if spyt.GetResource().Status.ReleaseStatus == ytv1.SpytReleaseStatusFinished {
		return ctrl.Result{}, nil
	}

	componentStatus := component.Status(ctx)

	if componentStatus.SyncStatus == components.SyncStatusBlocked {
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	if componentStatus.SyncStatus == components.SyncStatusReady {
		logger.Info("SPYT initialization finished")

		err := spyt.SaveReleaseStatus(ctx, ytv1.SpytReleaseStatusFinished)
		return ctrl.Result{Requeue: true}, err
	}

	if err := component.Sync(ctx); err != nil {
		logger.Error(err, "component sync failed", "component", "spyt")
		return ctrl.Result{Requeue: true}, err
	}

	if err := spyt.UpdateStatus(ctx); err != nil {
		logger.Error(err, "update spyt status failed")
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{Requeue: true}, nil
}
