package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
)

func (r *YtsaurusReconciler) handleEverything(
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

func (r *YtsaurusReconciler) handleStateless(
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

func (r *YtsaurusReconciler) handleMasterOnly(
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

func (r *YtsaurusReconciler) handleTabletNodesOnly(
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
	flow ytv1.UpdateFlow
	// componentNames is a list of component names that will be updated. It is built according to the update selector.
	componentNames []string
}

// chooseUpdateFlow considers spec and decides if operator should proceed with update or block.
// Block case is indicated with non-empty blockMsg.
// If update is not blocked, updateMeta containing a chosen flow and the component names to update returned.
func chooseUpdateFlow(spec ytv1.YtsaurusSpec, needUpdate []components.Component) (meta updateMeta, blockMsg string) {
	isFullUpdateEnabled := spec.EnableFullUpdate
	configuredSelector := spec.UpdateSelector

	masterNeedsUpdate := false
	tabletNodesNeedUpdate := false
	execNodesNeedUpdate := false
	statelessNeedUpdate := false
	var masterNames []string
	var tabletNodeNames []string
	var execNodeNames []string
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
		if comp.GetType() == consts.ExecNodeType {
			execNodesNeedUpdate = true
			execNodeNames = append(execNodeNames, comp.GetName())
		}
		statelessNames = append(statelessNames, comp.GetName())
		statelessNeedUpdate = true
	}
	statefulNeedUpdate := masterNeedsUpdate || tabletNodesNeedUpdate

	allNamesNeedingUpdate := getComponentNames(needUpdate)

	// Fallback to EnableFullUpdate field.
	if configuredSelector == ytv1.UpdateSelectorUnspecified {
		if statefulNeedUpdate {
			if isFullUpdateEnabled {
				return updateMeta{flow: ytv1.UpdateFlowFull, componentNames: nil}, ""
			} else {
				return updateMeta{flow: "", componentNames: nil}, "Full update is not allowed by enableFullUpdate field, ignoring it"
			}
		}
		return updateMeta{flow: ytv1.UpdateFlowStateless, componentNames: allNamesNeedingUpdate}, ""
	}

	switch configuredSelector {
	case ytv1.UpdateSelectorNothing:
		return updateMeta{}, "All updates are blocked by updateSelector field."
	case ytv1.UpdateSelectorEverything:
		if statefulNeedUpdate {
			return updateMeta{
				flow:           ytv1.UpdateFlowFull,
				componentNames: nil,
			}, ""
		} else {
			return updateMeta{
				flow:           ytv1.UpdateFlowStateless,
				componentNames: allNamesNeedingUpdate,
			}, ""
		}
	case ytv1.UpdateSelectorMasterOnly:
		if !masterNeedsUpdate {
			return updateMeta{}, "Only Master update is allowed by updateSelector, but it doesn't need update"
		}
		return updateMeta{
			flow:           ytv1.UpdateFlowMaster,
			componentNames: masterNames,
		}, ""
	case ytv1.UpdateSelectorTabletNodesOnly:
		if !tabletNodesNeedUpdate {
			return updateMeta{}, "Only Tablet nodes update is allowed by updateSelector, but they don't need update"
		}
		return updateMeta{
			flow:           ytv1.UpdateFlowTabletNodes,
			componentNames: tabletNodeNames,
		}, ""
	case ytv1.UpdateSelectorExecNodesOnly:
		if !execNodesNeedUpdate {
			return updateMeta{}, "Only Exec nodes update is allowed by updateSelector, but they don't need update"
		}
		return updateMeta{
			flow:           ytv1.UpdateFlowStateless,
			componentNames: execNodeNames,
		}, ""
	case ytv1.UpdateSelectorStatelessOnly:
		if !statelessNeedUpdate {
			return updateMeta{}, "Only stateless components update is allowed by updateSelector, but they don't need update"
		}
		return updateMeta{
			flow:           ytv1.UpdateFlowStateless,
			componentNames: statelessNames,
		}, ""
	default:
		return updateMeta{}, fmt.Sprintf("Unexpected update selector %s", configuredSelector)
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
			if err != nil {
				return ctrl.Result{Requeue: true}, err
			}
			err = ytsaurus.SaveObserverGeneration(ctx, resource.ObjectMeta.Generation)
			return ctrl.Result{Requeue: true}, err
		}

	case ytv1.ClusterStateRunning:
		needUpdate := componentManager.needUpdate()
		switch {
		case !componentManager.needSync():
			logger.Info("Ytsaurus is running and happy")
			return ctrl.Result{}, nil

		case componentManager.needInit():
			logger.Info("Ytsaurus needs initialization of some components")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateReconfiguration)
			return ctrl.Result{Requeue: true}, err

		case needUpdate != nil:
			var needUpdateNames []string
			for _, c := range needUpdate {
				needUpdateNames = append(needUpdateNames, c.GetName())
			}
			logger = logger.WithValues("componentsForUpdateAll", needUpdateNames)
			meta, blockMsg := chooseUpdateFlow(ytsaurus.GetResource().Spec, needUpdate)
			if blockMsg != "" {
				logger.Info(blockMsg)
				return ctrl.Result{Requeue: true}, nil
			}
			logger.Info("Ytsaurus needs components update",
				"componentsForUpdateSelected", meta.componentNames,
				"flow", meta.flow,
			)
			err = ytsaurus.SaveUpdatingClusterState(ctx, meta.flow, meta.componentNames)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

	case ytv1.ClusterStateUpdating:
		var result *ctrl.Result
		var err error

		switch ytsaurus.GetUpdateFlow() {
		case ytv1.UpdateFlowFull:
			result, err = r.handleEverything(ctx, ytsaurus, componentManager)
		case ytv1.UpdateFlowStateless:
			result, err = r.handleStateless(ctx, ytsaurus, componentManager)
		case ytv1.UpdateFlowMaster:
			result, err = r.handleMasterOnly(ctx, ytsaurus, componentManager)
		case ytv1.UpdateFlowTabletNodes:
			result, err = r.handleTabletNodesOnly(ctx, ytsaurus, componentManager)
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
		// We don't update observed generation because the update was not really finished,
		// and it's still the old version running.
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
		if err != nil {
			return ctrl.Result{}, err
		}
		err = ytsaurus.SaveObserverGeneration(ctx, resource.ObjectMeta.Generation)
		return ctrl.Result{}, err

	case ytv1.ClusterStateReconfiguration:
		if !componentManager.needInit() {
			logger.Info("Ytsaurus has reconfigured and is running now")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = ytsaurus.SaveObserverGeneration(ctx, resource.ObjectMeta.Generation)
			return ctrl.Result{}, err
		}
	}

	return componentManager.Sync(ctx)
}
