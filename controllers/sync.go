package controllers

import (
	"context"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"time"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *YtsaurusReconciler) handleUpdatingStateFullMode(
	ctx context.Context,
	ytsaurus *apiProxy.Ytsaurus,
	componentManager *ComponentManager,
) (*ctrl.Result, error) {
	resource := ytsaurus.GetResource()

	switch resource.Status.UpdateStatus.State {
	case ytv1.UpdateStateNone:
		ytsaurus.LogUpdate(ctx, "Checking the possibility of updating")
		err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStatePossibilityCheck)
		return &ctrl.Result{Requeue: true}, err

	case ytv1.UpdateStatePossibilityCheck:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionHasPossibility) {
			ytsaurus.LogUpdate(ctx, "Waiting for safe mode enabled")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForSafeModeEnabled)
			return &ctrl.Result{Requeue: true}, err
		} else if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionNoPossibility) {
			ytsaurus.LogUpdate(ctx, "Update is impossible, need to apply previous images")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateImpossibleToStart)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateImpossibleToStart:
		if !componentManager.needSync() || !ytsaurus.GetResource().Spec.EnableFullUpdate {
			ytsaurus.LogUpdate(ctx, "Spec changed back or full update isn't enabled, update is canceling")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateCancelUpdate)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForSafeModeEnabled:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSafeModeEnabled) {
			ytsaurus.LogUpdate(ctx, "Waiting for tablet cells saving")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsSaving)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsSaving:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsSaved) {
			ytsaurus.LogUpdate(ctx, "Waiting for tablet cells removing to start")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsRemovingStart)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemovingStart:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemovingStarted) {
			ytsaurus.LogUpdate(ctx, "Waiting for tablet cells removing to finish")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsRemoved)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemoved:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemoved) {
			ytsaurus.LogUpdate(ctx, "Waiting for snapshots")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForSnapshots)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForSnapshots:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSnaphotsSaved) {
			ytsaurus.LogUpdate(ctx, "Waiting for pods removal")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForPodsRemoval)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForPodsRemoval:
		if componentManager.arePodsRemoved() {
			ytsaurus.LogUpdate(ctx, "Waiting for pods creation")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForPodsCreation)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForPodsCreation:
		if componentManager.allReadyOrUpdating() {
			ytsaurus.LogUpdate(ctx, "All components were recreated")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsRecovery)
			return &ctrl.Result{RequeueAfter: time.Second * 7}, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRecovery:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRecovered) {
			ytsaurus.LogUpdate(ctx, "Waiting for operations archive prepare for updating")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare:
		if !componentManager.needSchedulerUpdate() ||
			ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionNotNecessaryToUpdateOpArchive) {
			ytsaurus.LogUpdate(ctx, "Operations archive update was skipped")
			ytsaurus.LogUpdate(ctx, "Waiting for query tracker state prepare for updating")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForQTStateUpdatingPrepare)
			return &ctrl.Result{Requeue: true}, err
		}
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionOpArchivePreparedForUpdating) {
			ytsaurus.LogUpdate(ctx, "Waiting for operations archive updating to finish")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForOpArchiveUpdate)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForOpArchiveUpdate:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionOpArchiveUpdated) {
			ytsaurus.LogUpdate(ctx, "Waiting for query tracker state prepare for updating")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForQTStateUpdatingPrepare)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForQTStateUpdatingPrepare:
		if !componentManager.needQueryTrackerUpdate() {
			ytsaurus.LogUpdate(ctx, "Query tracker state update was skipped")
			ytsaurus.LogUpdate(ctx, "Waiting for safe mode disabled")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForSafeModeDisabled)
			return &ctrl.Result{Requeue: true}, err
		}
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionQTStatePreparedForUpdating) {
			ytsaurus.LogUpdate(ctx, "Waiting for query tracker state updating to finish")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForQTStateUpdate)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForQTStateUpdate:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionQTStateUpdated) {
			ytsaurus.LogUpdate(ctx, "Waiting for safe mode disabled")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForSafeModeDisabled)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForSafeModeDisabled:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSafeModeDisabled) {
			ytsaurus.LogUpdate(ctx, "Finishing")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateUpdateFinishing)
			return &ctrl.Result{Requeue: true}, err
		}
	}

	return nil, nil
}

func (r *YtsaurusReconciler) handleUpdatingStateLocalMode(
	ctx context.Context,
	ytsaurus *apiProxy.Ytsaurus,
	componentManager *ComponentManager,
) (*ctrl.Result, error) {
	resource := ytsaurus.GetResource()

	switch resource.Status.UpdateStatus.State {
	case ytv1.UpdateStateNone:
		ytsaurus.LogUpdate(ctx, "Waiting for pods removal")
		err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForPodsRemoval)
		return &ctrl.Result{Requeue: true}, err

	case ytv1.UpdateStateWaitingForPodsRemoval:
		if componentManager.arePodsRemoved() {
			ytsaurus.LogUpdate(ctx, "Waiting for pods creation")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForPodsCreation)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForPodsCreation:
		if componentManager.allReadyOrUpdating() {
			ytsaurus.LogUpdate(ctx, "All components were recreated")
			ytsaurus.LogUpdate(ctx, "Waiting for operations archive prepare for updating")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare:
		if !componentManager.needSchedulerUpdate() || ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionNotNecessaryToUpdateOpArchive) {
			ytsaurus.LogUpdate(ctx, "Operations archive update was skipped")
			ytsaurus.LogUpdate(ctx, "Waiting for query tracker state prepare for updating")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForQTStateUpdatingPrepare)
			return &ctrl.Result{Requeue: true}, err
		}
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionOpArchivePreparedForUpdating) {
			ytsaurus.LogUpdate(ctx, "Waiting for operations archive updating to finish")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForOpArchiveUpdate)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForOpArchiveUpdate:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionOpArchiveUpdated) {
			ytsaurus.LogUpdate(ctx, "Waiting for query tracker state prepare for updating")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForQTStateUpdatingPrepare)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForQTStateUpdatingPrepare:
		if !componentManager.needQueryTrackerUpdate() {
			ytsaurus.LogUpdate(ctx, "Query tracker state update was skipped")
			ytsaurus.LogUpdate(ctx, "Finishing")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateUpdateFinishing)
			return &ctrl.Result{Requeue: true}, err
		}
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionQTStatePreparedForUpdating) {
			ytsaurus.LogUpdate(ctx, "Waiting for query tracker state updating to finish")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForQTStateUpdate)
			return &ctrl.Result{Requeue: true}, err
		}

	case ytv1.UpdateStateWaitingForQTStateUpdate:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionQTStateUpdated) {
			ytsaurus.LogUpdate(ctx, "Finishing")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateUpdateFinishing)
			return &ctrl.Result{Requeue: true}, err
		}
	}

	return nil, nil
}

func getComponentNames(components []components.Component) []string {
	if components == nil {
		return nil
	}
	names := make([]string, 0)
	for _, cmp := range components {
		names = append(names, cmp.GetName())
	}
	return names
}

func (r *YtsaurusReconciler) Sync(ctx context.Context, resource *ytv1.Ytsaurus) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !resource.Spec.IsManaged {
		logger.Info("Ytsaurus cluster is not managed by controller, do nothing")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	ytsaurus := apiProxy.NewYtsaurus(resource, r.Client, r.Recorder, r.Scheme)
	componentManager, err := NewComponentManager(ctx, ytsaurus)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	switch resource.Status.State {
	case ytv1.ClusterStateCreated:
		logger.Info("Ytsaurus is just created and needs initialization")
		err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateInitializing)
		return ctrl.Result{Requeue: true}, err

	case ytv1.ClusterStateInitializing:
		// Ytsaurus has finished initializing, and is running now.
		if !componentManager.needSync() {
			logger.Info("Ytsaurus has synced and is running now")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
			return ctrl.Result{Requeue: true}, err
		}

	case ytv1.ClusterStateRunning:
		switch {
		case !componentManager.needSync():
			logger.Info("Ytsaurus is running and happy")
			return ctrl.Result{}, nil

		case componentManager.needFullUpdate():
			logger.Info("Ytsaurus needs full update")
			if !ytsaurus.GetResource().Spec.EnableFullUpdate {
				logger.Info("Full update isn't allowed, ignore it")
				return ctrl.Result{}, nil
			}
			err := ytsaurus.SaveUpdatingClusterState(ctx, nil)
			return ctrl.Result{Requeue: true}, err

		case componentManager.needLocalUpdate() != nil:
			componentNames := getComponentNames(componentManager.needLocalUpdate())
			logger.Info("Ytsaurus needs local components update", "components", componentNames)
			err := ytsaurus.SaveUpdatingClusterState(ctx, componentNames)
			return ctrl.Result{Requeue: true}, err

		case componentManager.needSync():
			logger.Info("Ytsaurus needs reconfiguration")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateReconfiguration)
			return ctrl.Result{Requeue: true}, err
		}

	case ytv1.ClusterStateUpdating:
		var result *ctrl.Result
		var err error
		if ytsaurus.GetLocalUpdatingComponents() != nil {
			result, err = r.handleUpdatingStateLocalMode(ctx, ytsaurus, componentManager)
		} else {
			result, err = r.handleUpdatingStateFullMode(ctx, ytsaurus, componentManager)
		}

		if result != nil {
			return *result, err
		}

	case ytv1.ClusterStateCancelUpdate:
		if err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateNone); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		logger.Info("Ytsaurus update was canceled, ytsaurus is running now")
		err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
		return ctrl.Result{}, err

	case ytv1.ClusterStateUpdateFinishing:
		if err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateNone); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		if err := ytsaurus.ClearUpdateStatus(ctx); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		logger.Info("Ytsaurus update was finished and Ytsaurus is running now")
		err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
		return ctrl.Result{}, err

	case ytv1.ClusterStateReconfiguration:
		if !componentManager.needSync() {
			logger.Info("Ytsaurus has reconfigured and is running now")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
			return ctrl.Result{}, err
		}
	}

	return componentManager.Sync(ctx)
}
