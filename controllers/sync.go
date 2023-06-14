package controllers

import (
	"context"
	"fmt"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *YtsaurusReconciler) getComponents(ctx context.Context, ytsaurus *ytv1.Ytsaurus, apiProxy *apiProxy.APIProxy) []components.Component {
	cfgen := ytconfig.NewGenerator(ytsaurus, getClusterDomain(r.Client))

	d := components.NewDiscovery(cfgen, apiProxy)
	m := components.NewMaster(cfgen, apiProxy)
	var hps []components.Component
	for _, hpSpec := range apiProxy.Ytsaurus().Spec.HTTPProxies {
		hps = append(hps, components.NewHTTPProxy(cfgen, apiProxy, m, hpSpec))
	}
	yc := components.NewYtsaurusClient(cfgen, apiProxy, hps[0])

	dn := components.NewDataNode(cfgen, apiProxy, m)

	var en, tn, s components.Component

	result := []components.Component{
		d, m, dn, yc,
	}
	result = append(result, hps...)

	if ytsaurus.Spec.UI != nil {
		ui := components.NewUI(cfgen, apiProxy, m)
		result = append(result, ui)
	}

	if ytsaurus.Spec.RPCProxies != nil && len(ytsaurus.Spec.RPCProxies) > 0 {
		var rps []components.Component
		for _, rpSpec := range apiProxy.Ytsaurus().Spec.RPCProxies {
			rps = append(rps, components.NewRPCProxy(cfgen, apiProxy, m, rpSpec))
		}
		result = append(result, rps...)
	}

	if ytsaurus.Spec.ExecNodes != nil && len(ytsaurus.Spec.ExecNodes) > 0 {
		en = components.NewExecNode(cfgen, apiProxy, m)
		result = append(result, en)
	}

	if ytsaurus.Spec.TabletNodes != nil && len(ytsaurus.Spec.TabletNodes) > 0 {
		tn = components.NewTabletNode(cfgen, apiProxy, yc)
		result = append(result, tn)
	}

	if ytsaurus.Spec.Schedulers != nil {
		s = components.NewScheduler(cfgen, apiProxy, m, en, tn)
		result = append(result, s)
	}

	if ytsaurus.Spec.ControllerAgents != nil {
		ca := components.NewControllerAgent(cfgen, apiProxy, m)
		result = append(result, ca)
	}

	if ytsaurus.Spec.QueryTrackers != nil {
		q := components.NewQueryTracker(cfgen, apiProxy, m)
		result = append(result, q)
	}

	if ytsaurus.Spec.YQLAgents != nil {
		yqla := components.NewYQLAgent(cfgen, apiProxy, m)
		result = append(result, yqla)
	}

	if ytsaurus.Spec.Chyt != nil {
		chyt := components.NewChytController(cfgen, apiProxy, m, dn)
		result = append(result, chyt)
	}

	if ytsaurus.Spec.Spyt != nil && en != nil {
		spyt := components.NewSpyt(cfgen, apiProxy, m, en, s, dn)
		result = append(result, spyt)
	}

	return result
}

func (r *YtsaurusReconciler) saveClusterState(ctx context.Context, ytsaurus *ytv1.Ytsaurus, clusterState ytv1.ClusterState) error {
	logger := log.FromContext(ctx)
	ytsaurus.Status.State = clusterState
	if err := r.Status().Update(ctx, ytsaurus); err != nil {
		logger.Error(err, "unable to update YT cluster status")
		return err
	}

	return nil
}

func (r *YtsaurusReconciler) saveUpdateState(ctx context.Context, ytsaurus *ytv1.Ytsaurus, updateState ytv1.UpdateState) error {
	logger := log.FromContext(ctx)
	ytsaurus.Status.UpdateStatus.State = updateState
	if err := r.Status().Update(ctx, ytsaurus); err != nil {
		logger.Error(err, "unable to update YTsaurus update state")
		return err
	}
	return nil
}

func (r *YtsaurusReconciler) logUpdate(ctx context.Context, proxy *apiProxy.APIProxy, message string) {
	logger := log.FromContext(ctx)
	proxy.RecordNormal("Update", message)
	logger.Info(fmt.Sprintf("Ytsaurus update: %s", message))
}

func (r *YtsaurusReconciler) arePodsRemoved(proxy *apiProxy.APIProxy, cmps []components.Component) bool {
	for _, cmp := range cmps {
		if scmp, ok := cmp.(components.ServerComponent); ok {
			if !proxy.IsUpdateStatusConditionTrue(scmp.GetPodsRemovedCondition()) {
				return false
			}
		}
	}
	return true
}

func (r *YtsaurusReconciler) handleUpdatingState(
	ctx context.Context,
	proxy *apiProxy.APIProxy,
	ytsaurus *ytv1.Ytsaurus,
	cmps []components.Component,
	allReadyOrUpdating bool,
) (*ctrl.Result, error) {
	switch ytsaurus.Status.UpdateStatus.State {
	case ytv1.UpdateStateNone:
		r.logUpdate(ctx, proxy, "Waiting for safe mode enabled")
		err := r.saveUpdateState(ctx, ytsaurus, ytv1.UpdateStateWaitingForSafeModeEnabled)
		return &ctrl.Result{Requeue: true}, err

	case ytv1.UpdateStateWaitingForSafeModeEnabled:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionSafeModeEnabled) {
			r.logUpdate(ctx, proxy, "Waiting for tablet cells saving")
			err := r.saveUpdateState(ctx, ytsaurus, ytv1.UpdateStateWaitingForTabletCellsSaving)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsSaving:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsSaved) {
			r.logUpdate(ctx, proxy, "Waiting for tablet cells removing to start")
			err := r.saveUpdateState(ctx, ytsaurus, ytv1.UpdateStateWaitingForTabletCellsRemovingStart)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemovingStart:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemovingStarted) {
			r.logUpdate(ctx, proxy, "Waiting for tablet cells removing to finish")
			err := r.saveUpdateState(ctx, ytsaurus, ytv1.UpdateStateWaitingForTabletCellsRemoved)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemoved:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemoved) {
			r.logUpdate(ctx, proxy, "Waiting for snapshots")
			err := r.saveUpdateState(ctx, ytsaurus, ytv1.UpdateStateWaitingForSnapshots)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForSnapshots:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionSnaphotsSaved) {
			r.logUpdate(ctx, proxy, "Waiting for pods removal")
			err := r.saveUpdateState(ctx, ytsaurus, ytv1.UpdateStateWaitingForPodsRemoval)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForPodsRemoval:
		if r.arePodsRemoved(proxy, cmps) {
			r.logUpdate(ctx, proxy, "Waiting for pods creation")
			err := r.saveUpdateState(ctx, ytsaurus, ytv1.UpdateStateWaitingForPodsCreation)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForPodsCreation:
		if allReadyOrUpdating {
			r.logUpdate(ctx, proxy, "All components were recreated")
			err := r.saveUpdateState(ctx, ytsaurus, ytv1.UpdateStateWaitingForTabletCellsRecovery)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRecovery:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRecovered) {
			r.logUpdate(ctx, proxy, "Waiting for safe move disabled")
			err := r.saveUpdateState(ctx, ytsaurus, ytv1.UpdateStateWaitingForSafeModeDisabled)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForSafeModeDisabled:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionSafeModeDisabled) {
			r.logUpdate(ctx, proxy, "Finishing")
			err := r.saveClusterState(ctx, ytsaurus, ytv1.ClusterStateUpdateFinishing)
			return &ctrl.Result{Requeue: true}, err
		}
	}

	return nil, nil
}

func (r *YtsaurusReconciler) ClearUpdateStatus(ctx context.Context, proxy *apiProxy.APIProxy, ytsaurus *ytv1.Ytsaurus) error {
	ytsaurus.Status.UpdateStatus.Conditions = make([]metav1.Condition, 0)
	ytsaurus.Status.UpdateStatus.TabletCellBundles = make([]ytv1.TabletCellBundleInfo, 0)
	ytsaurus.Status.UpdateStatus.MasterMonitoringPaths = make([]string, 0)
	return proxy.UpdateStatus(ctx)
}

func (r *YtsaurusReconciler) Sync(ctx context.Context, ytsaurus *ytv1.Ytsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !ytsaurus.Spec.IsManaged {
		logger.Info("Ytsaurus cluster isn't managed by controller, do nothing")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	var readyComponents []string
	var notReadyComponents []string

	proxy := apiProxy.NewAPIProxy(ytsaurus, r.Client, r.Recorder, r.Scheme)
	cmps := r.getComponents(ctx, ytsaurus, proxy)
	needSync := false
	needUpdate := false
	allReadyOrUpdating := true

	for _, c := range cmps {
		err := c.Fetch(ctx)
		if err != nil {
			logger.Error(err, "failed to fetch status for controller", "component", c.GetName())
			return ctrl.Result{Requeue: true}, err
		}

		status := c.Status(ctx)

		if status == components.SyncStatusNeedUpdate {
			needUpdate = true
		}

		if status != components.SyncStatusReady && status != components.SyncStatusUpdating {
			allReadyOrUpdating = false
		}

		if status != components.SyncStatusReady {
			logger.Info("component is not ready", "component", c.GetName(), "syncStatus", status)
			notReadyComponents = append(notReadyComponents, c.GetName())
			needSync = true
		} else {
			readyComponents = append(readyComponents, c.GetName())
		}
	}

	logger.Info("Ytsaurus sync status",
		"notReadyComponents", notReadyComponents,
		"readyComponents", readyComponents,
		"updateState", ytsaurus.Status.UpdateStatus.State,
		"clusterState", ytsaurus.Status.State)

	switch ytsaurus.Status.State {
	case ytv1.ClusterStateCreated:
		logger.Info("Ytsaurus is just created and needs initialization")
		err := r.saveClusterState(ctx, ytsaurus, ytv1.ClusterStateInitializing)
		return ctrl.Result{Requeue: true}, err

	// Ytsaurus has finished initializing, and is running now.
	case ytv1.ClusterStateInitializing:
		if !needSync {
			logger.Info("Ytsaurus has synced and is running now")
			err := r.saveClusterState(ctx, ytsaurus, ytv1.ClusterStateRunning)
			return ctrl.Result{}, err
		}

	case ytv1.ClusterStateRunning:
		switch {
		case needSync:
			logger.Info("Ytsaurus is running and happy")
			return ctrl.Result{}, nil

		case !needSync:
			logger.Info("Ytsaurus needs reconfiguration")
			err := r.saveClusterState(ctx, ytsaurus, ytv1.ClusterStateReconfiguration)
			return ctrl.Result{Requeue: true}, err

		case needUpdate:
			logger.Info("Ytsaurus needs update")
			err := r.saveClusterState(ctx, ytsaurus, ytv1.ClusterStateUpdating)
			return ctrl.Result{Requeue: true}, err
		}

	case ytv1.ClusterStateUpdating:
		result, err := r.handleUpdateStatus(ctx, proxy, ytsaurus, cmps, allReadyOrUpdating)
		if result != nil {
			return *result, err
		}

	case ytv1.ClusterStateUpdateFinishing:
		if err := r.saveUpdateState(ctx, ytsaurus, ytv1.UpdateStateNone); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		if err := r.ClearUpdateStatus(ctx, proxy, ytsaurus); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		logger.Info("Ytsaurus update was finished and Ytsaurus is running now")
		err := r.saveClusterState(ctx, ytsaurus, ytv1.ClusterStateRunning)
		return ctrl.Result{}, err

	case ytv1.ClusterStateReconfiguration:
		if !needSync {
			logger.Info("Ytsaurus has reconfigured and is running now")
			err := r.saveClusterState(ctx, ytsaurus, ytv1.ClusterStateRunning)
			return ctrl.Result{}, err
		}
	}

	hasPending := false
	for _, c := range cmps {
		status := c.Status(ctx)

		if status == components.SyncStatusPending || status == components.SyncStatusUpdating {
			hasPending = true
			if err := c.Sync(ctx); err != nil {
				logger.Error(err, "component sync failed", "component", c.GetName())
				return ctrl.Result{Requeue: true}, err
			}
		}
	}

	if !hasPending {
		// All components are blocked.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return ctrl.Result{RequeueAfter: time.Second}, nil
}
