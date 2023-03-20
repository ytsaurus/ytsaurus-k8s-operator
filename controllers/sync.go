package controllers

import (
	"context"
	"fmt"
	ytv1 "github.com/YTsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/components"
	"github.com/YTsaurus/yt-k8s-operator/pkg/ytconfig"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

func (r *YtsaurusReconciler) getComponents(ctx context.Context, ytsaurus *ytv1.Ytsaurus) []components.Component {
	logger := log.FromContext(ctx)

	cfgen := ytconfig.NewGenerator(ytsaurus, getClusterDomain(r.Client))
	proxy := apiProxy.NewApiProxy(ytsaurus, r.Client, r.Recorder, r.Scheme)

	d := components.NewDiscovery(cfgen, proxy)
	m := components.NewMaster(cfgen, proxy)
	ui := components.NewUi(cfgen, proxy, m)
	hp := components.NewHttpProxy(cfgen, proxy, m)
	dn := components.NewDataNode(cfgen, proxy, m)

	var en, tn components.Component

	result := []components.Component{
		d, m, ui, hp, dn,
	}

	if ytsaurus.Spec.RpcProxies != nil {
		rp := components.NewRpcProxy(cfgen, proxy, m)
		result = append(result, rp)
	}

	if ytsaurus.Spec.ExecNodes != nil {
		en = components.NewExecNode(cfgen, proxy, m)
		result = append(result, en)
	}

	if ytsaurus.Spec.TabletNodes != nil {
		tn = components.NewTabletNode(cfgen, proxy, m)
		result = append(result, tn)
	}

	if ytsaurus.Spec.Schedulers != nil {
		s := components.NewScheduler(cfgen, proxy, m, en, tn)
		result = append(result, s)
	}

	if ytsaurus.Spec.ControllerAgents != nil {
		ca := components.NewControllerAgent(cfgen, proxy, m)
		result = append(result, ca)
	}

	if ytsaurus.Spec.QueryTrackers != nil {
		q := components.NewQueryTracker(cfgen, proxy, m)
		result = append(result, q)
	}

	if ytsaurus.Spec.YqlAgents != nil {
		yqla := components.NewYqlAgent(cfgen, proxy, m)
		result = append(result, yqla)
	}

	if ytsaurus.Spec.Chyt != nil {
		chyt := components.NewChytController(cfgen, proxy, m, dn)
		result = append(result, chyt)
	}

	if ytsaurus.Spec.Spyt != nil && en != nil {
		logger.Info("getComponents - SPYT will be run")
		spyt := components.NewSpyt(cfgen, proxy, m, en)
		result = append(result, spyt)
	} else {
		logger.Info("getComponents - no SPYT")
	}

	return result
}

func (r *YtsaurusReconciler) updateClusterState(ctx context.Context, ytsaurus *ytv1.Ytsaurus, clusterState ytv1.ClusterState) error {
	ytsaurus.Status.State = clusterState
	if err := r.Status().Update(ctx, ytsaurus); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "unable to update YT cluster status")
		return err
	}
	return nil
}

func (r *YtsaurusReconciler) Sync(ctx context.Context, ytsaurus *ytv1.Ytsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	cmps := r.getComponents(ctx, ytsaurus)
	needSync := false
	for _, c := range cmps {
		err := c.Fetch(ctx)
		if err != nil {
			logger.Error(err, "failed to fetch status for controller", "component", fmt.Sprintf("%T", c))
			return ctrl.Result{Requeue: true}, err
		}

		status := c.Status(ctx)
		if status != components.SyncStatusReady {
			logger.Info("component is not ready", "component", fmt.Sprintf("%T", c), "syncStatus", status)
			needSync = needSync || true
		}
	}

	if ytsaurus.Status.State == ytv1.ClusterStateRunning && !needSync {
		logger.V(1).Info("Ytsaurus is running and happy")
		return ctrl.Result{}, nil
	}

	if ytsaurus.Status.State == ytv1.ClusterStateRunning && needSync {
		logger.V(1).Info("Ytsaurus needs reconfiguration")
		err := r.updateClusterState(ctx, ytsaurus, ytv1.ClusterStateReconfiguration)
		return ctrl.Result{Requeue: true}, err
	}

	if ytsaurus.Status.State == ytv1.ClusterStateCreated {
		logger.V(1).Info("Ytsaurus is just crated and needs initialization")
		err := r.updateClusterState(ctx, ytsaurus, ytv1.ClusterStateInitializing)
		return ctrl.Result{Requeue: true}, err
	}

	// Ytsaurus has finished initializing or reconfiguring, and is now running.
	if !needSync {
		logger.V(1).Info("Ytsaurus has synced and is now running")
		err := r.updateClusterState(ctx, ytsaurus, ytv1.ClusterStateRunning)
		return ctrl.Result{}, err
	}

	hasPending := false
	for _, c := range cmps {
		if c.Status(ctx) == components.SyncStatusPending {
			hasPending = true
			if err := c.Sync(ctx); err != nil {
				return ctrl.Result{Requeue: true}, err
			}
		}
	}

	if !hasPending {
		// All components are blocked.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{Requeue: true}, nil
}
