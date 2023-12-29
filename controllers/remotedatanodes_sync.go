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

type remoteMaster struct {
	name string
}

func NewRemoteMaster(name string) *remoteMaster {
	return &remoteMaster{name: name}
}

func (m *remoteMaster) GetName() string {
	return m.name
}
func (m *remoteMaster) Status(_ context.Context) components.ComponentStatus {
	return components.ComponentStatus{
		SyncStatus: components.SyncStatusReady,
		Message:    "Remote master status assumed ready",
	}
}

func (r *RemoteDataNodesReconciler) Sync(ctx context.Context, resource *ytv1.RemoteDataNodes, remoteYtsaurus *ytv1.RemoteYtsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	nodes := apiproxy.NewRemoteDataNodes(resource, r.Client, r.Recorder, r.Scheme)

	_, err := ytconfig.NewNodeGenerator(
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

	//component := components.NewDataNodeConfigured(
	//	cfgen,
	//	components.NewRemoteYtsaurusStateManager(),
	//	&nodes.GetResource().ObjectMeta,
	//	nodes.APIProxy(),
	//	&resource.Spec.ConfigurationSpec,
	//	NewRemoteMaster(remoteYtsaurus.Name+" remote master"),
	//	resource.Spec.DataNodesSpec,
	//)
	//
	//if nodes.GetResource().Status.DeployStatus == ytv1.RemoteDataNodesStatusStarted {
	//	return ctrl.Result{}, nil
	//}
	//
	//err = component.Fetch(ctx)
	//if err != nil {
	//	logger.Error(err, "failed to fetch remote data nodes status for controller")
	//	return ctrl.Result{Requeue: true}, err
	//}
	//
	//status := component.Status(ctx)
	//if status.SyncStatus == components.SyncStatusBlocked {
	//	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	//}
	//if status.SyncStatus == components.SyncStatusReady {
	//	logger.Info("Data nodes initialization finished")
	//
	//	err := nodes.SaveDeployStatus(ctx, ytv1.RemoteDataNodesStatusStarted)
	//	return ctrl.Result{Requeue: true}, err
	//}
	//if err := component.Sync(ctx); err != nil {
	//	logger.Error(err, "component sync failed", "component", "datanodes")
	//	return ctrl.Result{Requeue: true}, err
	//}
	//if err := nodes.APIProxy().UpdateStatus(ctx); err != nil {
	//	logger.Error(err, "update chyt status failed")
	//	return ctrl.Result{Requeue: true}, err
	//}

	return ctrl.Result{Requeue: true}, nil
}
