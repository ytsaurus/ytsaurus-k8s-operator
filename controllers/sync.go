package controllers

import (
	"context"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"time"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ComponentStatus struct {
	needSync           bool
	needUpdate         bool
	allReadyOrUpdating bool
}

func (r *YtsaurusReconciler) handleUpdatingState(
	ctx context.Context,
	proxy *apiProxy.APIProxy,
	componentManager *ComponentManager,
) (*ctrl.Result, error) {
	ytsaurus := proxy.Ytsaurus()

	switch ytsaurus.Status.UpdateStatus.State {
	case ytv1.UpdateStateNone:
		proxy.LogUpdate(ctx, "Checking the possibility of updating")
		err := proxy.SaveUpdateState(ctx, ytv1.UpdateStatePossibilityCheck)
		return &ctrl.Result{Requeue: true}, err

	case ytv1.UpdateStatePossibilityCheck:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionHasPossibility) {
			proxy.LogUpdate(ctx, "Waiting for safe mode enabled")
			err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForSafeModeEnabled)
			return &ctrl.Result{Requeue: true}, err
		} else if proxy.IsUpdateStatusConditionTrue(consts.ConditionNoPossibility) {
			proxy.LogUpdate(ctx, "Update is impossible, need to apply previous core image")
			err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateImpossibleToStart)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateImpossibleToStart:
		if componentManager.masterComponent.IsImageEqualTo(proxy.Ytsaurus().Spec.CoreImage) {
			proxy.LogUpdate(ctx, "Core image was changed back, update is canceling")
			err := proxy.SaveClusterState(ctx, ytv1.ClusterStateCancelUpdate)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForSafeModeEnabled:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionSafeModeEnabled) {
			proxy.LogUpdate(ctx, "Waiting for tablet cells saving")
			err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsSaving)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsSaving:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsSaved) {
			proxy.LogUpdate(ctx, "Waiting for tablet cells removing to start")
			err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsRemovingStart)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemovingStart:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemovingStarted) {
			proxy.LogUpdate(ctx, "Waiting for tablet cells removing to finish")
			err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsRemoved)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemoved:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemoved) {
			proxy.LogUpdate(ctx, "Waiting for snapshots")
			err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForSnapshots)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForSnapshots:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionSnaphotsSaved) {
			proxy.LogUpdate(ctx, "Waiting for pods removal")
			err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForPodsRemoval)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForPodsRemoval:
		if componentManager.areServerPodsRemoved(proxy) {
			proxy.LogUpdate(ctx, "Waiting for pods creation")
			err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForPodsCreation)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForPodsCreation:
		if componentManager.allReadyOrUpdating() {
			proxy.LogUpdate(ctx, "All components were recreated")
			err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsRecovery)
			return &ctrl.Result{RequeueAfter: time.Second * 7}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRecovery:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRecovered) {
			proxy.LogUpdate(ctx, "Waiting for safe mode disabled")
			err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForSafeModeDisabled)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForSafeModeDisabled:
		if proxy.IsUpdateStatusConditionTrue(consts.ConditionSafeModeDisabled) {
			proxy.LogUpdate(ctx, "Finishing")
			err := proxy.SaveClusterState(ctx, ytv1.ClusterStateUpdateFinishing)
			return &ctrl.Result{Requeue: true}, err
		}
	}

	return nil, nil
}

func (r *YtsaurusReconciler) Sync(ctx context.Context, ytsaurus *ytv1.Ytsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !ytsaurus.Spec.IsManaged {
		logger.Info("Ytsaurus cluster is not managed by controller, do nothing")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	proxy := apiProxy.NewAPIProxy(ytsaurus, r.Client, r.Recorder, r.Scheme)
	componentManager, err := NewComponentManager(ctx, proxy)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	switch ytsaurus.Status.State {
	case ytv1.ClusterStateCreated:
		logger.Info("Ytsaurus is just created and needs initialization")
		err := proxy.SaveClusterState(ctx, ytv1.ClusterStateInitializing)
		return ctrl.Result{Requeue: true}, err

	case ytv1.ClusterStateInitializing:
		// Ytsaurus has finished initializing, and is running now.
		if !componentManager.needSync() {
			logger.Info("Ytsaurus has synced and is running now")
			err := proxy.SaveClusterState(ctx, ytv1.ClusterStateRunning)
			return ctrl.Result{}, err
		}

	case ytv1.ClusterStateRunning:
		switch {
		case !componentManager.needSync():
			logger.Info("Ytsaurus is running and happy")
			return ctrl.Result{}, nil

		case componentManager.needUpdate():
			logger.Info("Ytsaurus needs update")
			err := proxy.SaveClusterState(ctx, ytv1.ClusterStateUpdating)
			return ctrl.Result{Requeue: true}, err

		case componentManager.needSync():
			logger.Info("Ytsaurus needs reconfiguration")
			err := proxy.SaveClusterState(ctx, ytv1.ClusterStateReconfiguration)
			return ctrl.Result{Requeue: true}, err
		}

	case ytv1.ClusterStateUpdating:
		result, err := r.handleUpdatingState(ctx, proxy, componentManager)
		if result != nil {
			return *result, err
		}

	case ytv1.ClusterStateCancelUpdate:
		if err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateNone); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		logger.Info("Ytsaurus update was canceled, ytsaurus is running now")
		err := proxy.SaveClusterState(ctx, ytv1.ClusterStateRunning)
		return ctrl.Result{}, err

	case ytv1.ClusterStateUpdateFinishing:
		if err := proxy.SaveUpdateState(ctx, ytv1.UpdateStateNone); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		if err := proxy.ClearUpdateStatus(ctx); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		logger.Info("Ytsaurus update was finished and Ytsaurus is running now")
		err := proxy.SaveClusterState(ctx, ytv1.ClusterStateRunning)
		return ctrl.Result{}, err

	case ytv1.ClusterStateReconfiguration:
		if !componentManager.needSync() {
			logger.Info("Ytsaurus has reconfigured and is running now")
			err := proxy.SaveClusterState(ctx, ytv1.ClusterStateRunning)
			return ctrl.Result{}, err
		}
	}

	return componentManager.Sync(ctx)
}
