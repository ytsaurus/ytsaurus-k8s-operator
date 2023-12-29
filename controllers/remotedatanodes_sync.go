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

func (r *RemoteDataNodesReconciler) Sync(ctx context.Context, resource *ytv1.RemoteDataNodes, remoteYtsaurus *ytv1.RemoteYtsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("component", "remotedatanodes")

	nodes := apiproxy.NewRemoteDataNodes(resource, r.Client, r.Recorder, r.Scheme)

	cfgen, err := ytconfig.NewNodeGenerator(
		types.NamespacedName{Name: resource.Name, Namespace: resource.Namespace},
		getClusterDomain(nodes.APIProxy().Client()),
		&resource.Spec.ConfigurationSpec,
		&remoteYtsaurus.Spec.MasterConnectionSpec,
		0,
	)
	if err != nil {
		logger.Error(err, "failed to create config generator")
		return ctrl.Result{}, err
	}

	component := components.NewDataNodeRemote(
		cfgen,
		nodes,
		resource.Spec.DataNodesSpec,
	)

	err = component.Fetch(ctx)
	if err != nil {
		logger.Error(err, "failed to fetch remote data nodes status for controller")
		return ctrl.Result{Requeue: true}, err
	}

	status := component.Status(ctx)
	if status.SyncStatus == components.SyncStatusReady {
		logger.Info("component ready")
		return ctrl.Result{}, nil
	}

	logger.Info("component sync")
	if err = component.Sync(ctx); err != nil {
		logger.Error(err, "component sync failed")
		return ctrl.Result{Requeue: true}, err
	}

	// TODO: what status is this?
	// TODO: CRD status is needed?
	if err = nodes.APIProxy().UpdateStatus(ctx); err != nil {
		logger.Error(err, "update status failed")
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{Requeue: true}, nil
}
