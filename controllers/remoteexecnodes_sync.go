package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func (r *RemoteExecNodesReconciler) Sync(
	ctx context.Context,
	resource *ytv1.RemoteExecNodes,
	remoteYtsaurus *ytv1.RemoteYtsaurus,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("component", "remoteexecnodes")
	apiProxy := apiproxy.NewAPIProxy(resource, r.Client, r.Recorder, r.Scheme)

	cfgen := ytconfig.NewRemoteNodeGenerator(
		types.NamespacedName{Name: resource.Name, Namespace: resource.Namespace},
		getClusterDomain(r.Client),
		resource.Spec.CommonSpec,
		remoteYtsaurus.Spec.MasterConnectionSpec,
	)

	component := components.NewRemoteExecNodes(
		cfgen,
		resource,
		apiProxy,
		resource.Spec.ExecNodesSpec,
		resource.Spec.CommonSpec,
	)
	err := component.Fetch(ctx)
	if err != nil {
		logger.Error(err, "failed to fetch remote nodes")
		return ctrl.Result{Requeue: true}, err
	}

	status, err := component.Sync(ctx)
	if err != nil {
		logger.Error(err, "failed to sync remote nodes")
		return ctrl.Result{Requeue: true}, err
	}

	var requeue bool
	if status.SyncStatus != components.SyncStatusReady {
		resource.Status.ReleaseStatus = ytv1.RemoteExecNodeReleaseStatusPending
		requeue = true
	} else {
		resource.Status.ReleaseStatus = ytv1.RemoteExecNodeReleaseStatusRunning
		requeue = false
	}

	logger.Info("Setting status for remote exec nodes", "status", resource.Status.ReleaseStatus)
	err = r.Client.Status().Update(ctx, resource)
	if err != nil {
		logger.Error(err, "failed to update status for remote exec nodes")
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{Requeue: requeue}, nil
}
