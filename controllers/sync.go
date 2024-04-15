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

		case componentManager.needLocalUpdate() != nil:
			configuredStrategy := ytsaurus.GetResource().Spec.UpdateStrategy
			isFullUpdateEnabled := ytsaurus.GetResource().Spec.EnableFullUpdate

			masterNeedsUpdate := false
			tabletNodesNeedUpdate := false
			statelessNeedUpdate := false
			for _, comp := range componentManager.needLocalUpdate() {
				if comp.GetType() == consts.MasterType {
					masterNeedsUpdate = true
					continue
				}
				if comp.GetType() == consts.TabletNodeType {
					tabletNodesNeedUpdate = true
					continue
				}
				statelessNeedUpdate = true
			}
			statefullNeedUpdate := masterNeedsUpdate || tabletNodesNeedUpdate

			componentNames := getComponentNames(componentManager.needLocalUpdate())
			log := logger.WithValues("needUpdate", componentNames)
			// `strategy` and `componentsToUpdate` can be one of:
			//   - UpdateStrategyFull + nil == full update — all the pods will be recreated simultaneously
			//   - UpdateStrategyMasterOnly + [Master] — only master will be updated (with snapshots)
			//   - UpdateStrategyTabletNodesOnly + [TabletNode-a, TabletNode-b, ...] — only tablet nodes will be updated
			//     (with tablet cell bundles delete)
			//   - UpdateStrategyStatelessOnly + [non-empty list of components which doesn't include Master and TabletNodes]
			//     — some of the components will be updated with "local update" flow.
			// We persist that strategy and components' slice in the ytsaurus resource status until the end of update.

			// если strategy="" то как раньше
			// если нужно обновить мастер/тн то просто пишем в логи
			// и кластер не переходит в апдейтинг стейт
			// если strategy=block то ничего не делаем
			// если full или isFullUpdate=true и нужно обновить мастер или таблетноды — стратегия фулл
			// если мастеронли и мастер нужно обновить — то стратегия мастер
			// если таблетнод онли и тн нужно обновить — то стратегия таблетнод
			// если стейтлесс онли то выкидываем мастер и таблетноды из списка — стратегия стейтлесс

			// TODO: describe exact options in log message.
			chooseStrategy := func() (strategy ytv1.UpdateStrategy, components []string, blockUpdate bool) {
				// Backward compatible with EnableFullUpdateSetting if new setting is not used.
				if configuredStrategy == "" {
					if isFullUpdateEnabled && statefullNeedUpdate {
						strategy = ytv1.UpdateStrategyFull
						components = nil
					}
					if isFullUpdateEnabled && !statefullNeedUpdate {
						strategy = ytv1.UpdateStrategyStateless
						components = componentNames
					}
					if !isFullUpdateEnabled && statefullNeedUpdate {
						log.Info("Full update isn't allowed, ignore it")
						blockUpdate = true
					}
					if !isFullUpdateEnabled && !statefullNeedUpdate {
						strategy = ytv1.UpdateStrategyStateless
						components = componentNames
					}
					return
				}

				// New setting is used.
				switch configuredStrategy {
				case ytv1.UpdateStrategyBlocked:
					log.Info("All updates are blocked by ytsaurus spec configuration.")
					blockUpdate = true
				case ytv1.UpdateStrategyFull:
					if statefullNeedUpdate {
						strategy = ytv1.UpdateStrategyFull
						components = nil
					} else {
						strategy = ytv1.UpdateStrategyStateless
						components = componentNames
					}
				case ytv1.UpdateStrategyMasterOnly:
					if masterNeedsUpdate {
						strategy = ytv1.UpdateStrategyMasterOnly
						components = []string{"Master"} // Do we have MasterName?
					} else {
						log.Info("Only Master update is allowed by configuration, but it doesn't need update")
					}
				case ytv1.UpdateStrategyTabletNodesOnly:
					if tabletNodesNeedUpdate {
						strategy = ytv1.UpdateStrategyTabletNodesOnly
						for _, comp := range componentManager.needLocalUpdate() {
							if comp.GetType() == consts.TabletNodeType {
								components = append(components, comp.GetName())
							}
						}
					} else {
						log.Info("Only Tablet nodes update is allowed by configuration, but they don't need update")
					}
				case ytv1.UpdateStrategyStateless:
					if !statelessNeedUpdate {
						log.Info("Only stateless components update is allowed by configuration, but they don't need update")
					}
					for _, comp := range componentManager.needLocalUpdate() {
						if comp.GetType() == consts.MasterType {
							continue
						}
						if comp.GetType() == consts.TabletNodeType {
							continue
						}
						components = append(components, comp.GetName())
					}
				default:
					// TODO: just validate it in hook
					log.Info(fmt.Sprintf("Unexpected update strategy %s", configuredStrategy))
					blockUpdate = true
				}
				return
			}

			chosenStrategy, componentsToUpdate, blockUpdate := chooseStrategy()
			if blockUpdate {
				return ctrl.Result{}, nil
			}

			logger.Info("Ytsaurus will be updated",
				"componentsToUpdate", componentsToUpdate,
				"strategy", chosenStrategy,
			)
			err := ytsaurus.SaveUpdatingClusterState(ctx, chosenStrategy, componentsToUpdate)
			return ctrl.Result{Requeue: true}, err
		}

	case ytv1.ClusterStateUpdating:
		var result *ctrl.Result
		var err error

		switch ytsaurus.GetUpdateStrategy() {
		case ytv1.UpdateStrategyFull:
			result, err = r.handleFullStrategy(ctx, ytsaurus, componentManager)
		case ytv1.UpdateStrategyStateless:
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
