package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
)

func (r *YtsaurusReconciler) handleFullStrategy(
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
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForMasterExitReadOnly)
			return &ctrl.Result{RequeueAfter: time.Second * 7}, err
		}

	case ytv1.UpdateStateWaitingForMasterExitReadOnly:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionMasterExitedReadOnly) {
			ytsaurus.LogUpdate(ctx, "Masters exited read-only state")
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsRecovery)
			return &ctrl.Result{Requeue: true}, err
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

func (r *YtsaurusReconciler) handleStatelessStrategy(
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

func (r *YtsaurusReconciler) handleMasterOnlyStrategy(
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
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForMasterExitReadOnly)
			return &ctrl.Result{RequeueAfter: time.Second * 7}, err
		}

	case ytv1.UpdateStateWaitingForMasterExitReadOnly:
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionMasterExitedReadOnly) {
			ytsaurus.LogUpdate(ctx, "Masters exited read-only state")
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

func (r *YtsaurusReconciler) handleTabletNodesOnlyStrategy(
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
			err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsSaving)
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

type updateMeta struct {
	strategy       ytv1.UpdateStrategy
	componentNames []string
}

// chooseUpdateStrategy considers spec decides if operator should proceed with update or block.
// Block case is indicated with non-empty blockMsg.
// If update is not blocked, updateMeta containing a chosen strategy and the component names to update returned.
func chooseUpdateStrategy(spec ytv1.YtsaurusSpec, needUpdate []components.Component) (meta updateMeta, blockMsg string) {
	isFullUpdateEnabled := spec.EnableFullUpdate
	configuredStrategy := spec.UpdateStrategy

	masterNeedsUpdate := false
	tabletNodesNeedUpdate := false
	statelessNeedUpdate := false
	var masterNames []string
	var tabletNodeNames []string
	var statelessNames []string
	for _, comp := range needUpdate {
		if comp.GetType() == consts.MasterType {
			masterNeedsUpdate = true
			masterNames = append(masterNames, comp.GetName())
			continue
		}
		if comp.GetType() == consts.TabletNodeType {
			tabletNodesNeedUpdate = true
			tabletNodeNames = append(tabletNodeNames, comp.GetName())
			continue
		}
		statelessNames = append(statelessNames, comp.GetName())
		statelessNeedUpdate = true
	}
	statefulNeedUpdate := masterNeedsUpdate || tabletNodesNeedUpdate

	allNamesNeedingUpdate := getComponentNames(needUpdate)

	// Fallback to EnableFullUpdate field.
	if configuredStrategy == ytv1.UpdateStrategyNone {
		if statefulNeedUpdate {
			if isFullUpdateEnabled {
				return updateMeta{strategy: ytv1.UpdateStrategyFull, componentNames: nil}, ""
			} else {
				return updateMeta{strategy: "", componentNames: nil}, "Full update is not allowed by enableFullUpdate field, ignoring it"
			}
		}
		return updateMeta{strategy: ytv1.UpdateStrategyStatelessOnly, componentNames: allNamesNeedingUpdate}, ""
	}

	switch configuredStrategy {
	case ytv1.UpdateStrategyBlocked:
		return updateMeta{}, "All updates are blocked by updateStrategy field."
	case ytv1.UpdateStrategyFull:
		if statefulNeedUpdate {
			return updateMeta{
				strategy:       ytv1.UpdateStrategyFull,
				componentNames: nil,
			}, ""
		} else {
			return updateMeta{
				strategy:       ytv1.UpdateStrategyStatelessOnly,
				componentNames: allNamesNeedingUpdate,
			}, ""
		}
	case ytv1.UpdateStrategyMasterOnly:
		if !masterNeedsUpdate {
			return updateMeta{}, "Only Master update is allowed by updateStrategy, but it doesn't need update"
		}
		return updateMeta{
			strategy:       ytv1.UpdateStrategyMasterOnly,
			componentNames: masterNames,
		}, ""
	case ytv1.UpdateStrategyTabletNodesOnly:
		if !tabletNodesNeedUpdate {
			return updateMeta{}, "Only Tablet nodes update is allowed by updateStrategy, but they don't need update"
		}
		return updateMeta{
			strategy:       ytv1.UpdateStrategyTabletNodesOnly,
			componentNames: tabletNodeNames,
		}, ""
	case ytv1.UpdateStrategyStatelessOnly:
		if !statelessNeedUpdate {
			return updateMeta{}, "Only stateless components update is allowed by configuration, but they don't need update"
		}
		return updateMeta{
			strategy:       ytv1.UpdateStrategyStatelessOnly,
			componentNames: statelessNames,
		}, ""
	default:
		// TODO: just validate it in hook
		return updateMeta{}, fmt.Sprintf("Unexpected update strategy %s", configuredStrategy)
	}
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

		case componentManager.needInit():
			logger.Info("Ytsaurus needs initialization of some components")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateReconfiguration)
			return ctrl.Result{Requeue: true}, err

		case componentManager.needUpdate() != nil:
			meta, blockMsg := chooseUpdateStrategy(ytsaurus.GetResource().Spec, componentManager.needUpdate())
			if blockMsg != "" {
				logger.Info(blockMsg)
				return ctrl.Result{Requeue: true}, nil
			}
			logger.Info("Ytsaurus needs components update",
				"components", meta.componentNames,
				"strategy", meta.strategy,
			)
			err = ytsaurus.SaveUpdatingClusterState(ctx, meta.strategy, meta.componentNames)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

	case ytv1.ClusterStateUpdating:
		var result *ctrl.Result
		var err error

		switch ytsaurus.GetUpdateStrategy() {
		case ytv1.UpdateStrategyFull:
			result, err = r.handleFullStrategy(ctx, ytsaurus, componentManager)
		case ytv1.UpdateStrategyStatelessOnly:
			result, err = r.handleStatelessStrategy(ctx, ytsaurus, componentManager)
		case ytv1.UpdateStrategyMasterOnly:
			result, err = r.handleMasterOnlyStrategy(ctx, ytsaurus, componentManager)
		case ytv1.UpdateStrategyTabletNodesOnly:
			result, err = r.handleTabletNodesOnlyStrategy(ctx, ytsaurus, componentManager)
		}

		if result != nil {
			return *result, err
		}

	case ytv1.ClusterStateCancelUpdate:
		if err := ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateNone); err != nil {
			return ctrl.Result{Requeue: true}, err
		}

		if err := ytsaurus.ClearUpdateStatus(ctx); err != nil {
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
		if !componentManager.needInit() {
			logger.Info("Ytsaurus has reconfigured and is running now")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
			return ctrl.Result{}, err
		}
	}

	return componentManager.Sync(ctx)
}
