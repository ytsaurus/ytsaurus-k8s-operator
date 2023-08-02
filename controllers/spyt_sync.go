package controllers

import (
	"context"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *SpytReconciler) Sync(ctx context.Context, resource *ytv1.Spyt, ytsaurus *ytv1.Ytsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	spyt := apiproxy.NewSpyt(resource, r.Client, r.Recorder, r.Scheme)

	cfgen := ytconfig.NewGenerator(ytsaurus, getClusterDomain(spyt.APIProxy().Client()))

	component := components.NewSpyt(cfgen, spyt, ytsaurus)

	err := component.Fetch(ctx)
	if err != nil {
		logger.Error(err, "failed to fetch SPYT status for controller")
		return ctrl.Result{Requeue: true}, err
	}

	if status := component.Status(ctx); status != components.SyncStatusReady {
		err := component.Sync(ctx)
		return ctrl.Result{Requeue: true}, err
	}

	logger.Info("SPYT initialization finished")

	return ctrl.Result{}, err
}
