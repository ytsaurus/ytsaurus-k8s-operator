package controllers

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

func (r *RemoteDataNodesReconciler) Sync(
	ctx context.Context,
	resource *ytv1.RemoteDataNodes,
	remoteYtsaurus *ytv1.RemoteYtsaurus,
) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("component", "remoteDatanodes")
	apiProxy := apiproxy.NewAPIProxy(resource, r.Client, r.Recorder, r.Scheme)

	cfgen := ytconfig.NewRemoteNodeGenerator(remoteYtsaurus, resource.GetName(), getClusterDomain(r.Client), &resource.Spec.CommonSpec)

	component := components.NewRemoteDataNodes(
		cfgen,
		resource,
		apiProxy,
		resource.Spec.DataNodesSpec,
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
		resource.Status.ReleaseStatus = ytv1.RemoteNodeReleaseStatusPending
		requeue = true
	} else {
		resource.Status.ReleaseStatus = ytv1.RemoteNodeReleaseStatusRunning
		requeue = false
	}
	resource.Status.ObservedGeneration = resource.Generation

	logger.Info("Setting status for remote data nodes", "status", resource.Status.ReleaseStatus)
	err = r.Client.Status().Update(ctx, resource)
	if err != nil {
		logger.Error(err, "failed to update status for remote data nodes")
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{Requeue: requeue}, nil
}
