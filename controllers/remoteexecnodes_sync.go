package controllers

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

func (r *RemoteExecNodesReconciler) Sync(ctx context.Context, resource *ytv1.RemoteExecNodes, ytsaurus *ytv1.RemoteYtsaurus) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
