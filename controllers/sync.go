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

func canUpdateComponent(selectors []ytv1.ComponentUpdateSelector, component ytv1.Component) bool {
	for _, selector := range selectors {
		if selector.Class != consts.ComponentClassUnspecified {
			switch selector.Class {
			case consts.ComponentClassEverything:
				return true
			case consts.ComponentClassNothing:
				return false
			case consts.ComponentClassStateless:
				if component.Type != consts.DataNodeType && component.Type != consts.TabletNodeType && component.Type != consts.MasterType {
					return true
				}
			default:
				return false
			}
		}
		if selector.Component.Type == component.Type && (selector.Component.Name == "" || selector.Component.Name == component.Name) {
			return true
		}
	}
	return false
}

// Considers splits all the components in two groups: ones that can be updated and ones which update isblocked.
func chooseUpdatingComponents(spec ytv1.YtsaurusSpec, needUpdate []ytv1.Component, allComponents []ytv1.Component) (canUpdate []ytv1.Component, cannotUpdate []ytv1.Component) {
	configuredSelectors := getEffectiveSelectors(spec)

	for _, component := range needUpdate {
		upd := canUpdateComponent(configuredSelectors, component)
		if upd {
			canUpdate = append(canUpdate, component)
		} else {
			cannotUpdate = append(cannotUpdate, component)
		}
	}

	if len(canUpdate) == 0 {
		return nil, cannotUpdate
	}
	if hasEverythingSelector(configuredSelectors) && needFullUpdate(needUpdate) {
		// Here we update not only components that are not up-to-date, but all cluster.
		return allComponents, nil
	}
	return canUpdate, cannotUpdate
}

func hasEverythingSelector(selectors []ytv1.ComponentUpdateSelector) bool {
	for _, selector := range selectors {
		if selector.Class == consts.ComponentClassEverything {
			return true
		}
	}

	return false
}

func needFullUpdate(needUpdate []ytv1.Component) bool {
	statelessSelector := []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassStateless}}
	for _, component := range needUpdate {
		isStateless := canUpdateComponent(statelessSelector, component)
		if !isStateless {
			return true
		}
	}
	return false
}

func getEffectiveSelectors(spec ytv1.YtsaurusSpec) []ytv1.ComponentUpdateSelector {
	if spec.UpdatePlan != nil {
		return spec.UpdatePlan
	}

	if spec.UpdateSelector != ytv1.UpdateSelectorUnspecified {
		switch spec.UpdateSelector {
		case ytv1.UpdateSelectorNothing:
			return []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassNothing}}
		case ytv1.UpdateSelectorMasterOnly:
			return []ytv1.ComponentUpdateSelector{{
				Component: ytv1.Component{
					Type: consts.MasterType,
				},
			}}
		case ytv1.UpdateSelectorDataNodesOnly:
			return []ytv1.ComponentUpdateSelector{{
				Component: ytv1.Component{
					Type: consts.DataNodeType,
				},
			}}
		case ytv1.UpdateSelectorTabletNodesOnly:
			return []ytv1.ComponentUpdateSelector{{
				Component: ytv1.Component{
					Type: consts.TabletNodeType,
				},
			}}
		case ytv1.UpdateSelectorExecNodesOnly:
			return []ytv1.ComponentUpdateSelector{{
				Component: ytv1.Component{
					Type: consts.ExecNodeType,
				},
			}}
		case ytv1.UpdateSelectorStatelessOnly:
			return []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassStateless}}
		case ytv1.UpdateSelectorEverything:
			return []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassEverything}}
		}
	}

	if spec.EnableFullUpdate {
		return []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassEverything}}
	}

	return []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassStateless}}
}

func convertToComponent(components []components.Component) []ytv1.Component {
	var result []ytv1.Component
	for _, c := range components {
		result = append(result, ytv1.Component{
			Name: c.GetShortName(),
			Type: c.GetType(),
		})
	}
	return result
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
		needUpdate := componentManager.needUpdate()
		canUpdate, cannotUpdate := chooseUpdatingComponents(
			ytsaurus.GetResource().Spec,
			convertToComponent(needUpdate),
			convertToComponent(componentManager.allUpdatableComponents()),
		)

		// All status updates _must_ be in one transaction with observed generation and new cluster state.
		needStatusUpdate := ytsaurus.SyncObservedGeneration()

		// There may be the case when some components needed update, but spec was reverted
		// and Updating never happen — so blocked components column need to be always actualized.
		if ytsaurus.SetBlockedComponents(cannotUpdate) {
			needStatusUpdate = true
		}

		switch {
		case !componentManager.needSync():
			logger.Info("Ytsaurus is running and happy")

		case componentManager.needInit():
			logger.Info("Ytsaurus needs initialization of some components")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateReconfiguration)
			return ctrl.Result{Requeue: true}, err

		case len(needUpdate) == 0:
			logger.Info("All components are up-to-date")

		case len(canUpdate) != 0:
			logger.Info("Ytsaurus components needs update", "canUpdate", canUpdate, "cannotUpdate", cannotUpdate)
			// We do not update BlockedComponentsSummary here, it should be updated first thing in Running state.
			ytsaurus.SetUpdatingComponents(canUpdate)
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateUpdating)
			return ctrl.Result{Requeue: true}, err

		case len(cannotUpdate) != 0:
			logger.Info("Ytsaurus components update is blocked", "cannotUpdate", cannotUpdate)
		}

		// Have passed final check - save status update if needed.
		if needStatusUpdate {
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
			return ctrl.Result{Requeue: true}, err
		}

		// All done, nothing change - do not requeue reconcile.
		return ctrl.Result{}, nil

	case ytv1.ClusterStateUpdating:
		updatingComponents := ytsaurus.GetUpdatingComponents()
		progressed, err := buildAndExecuteFlow(ctx, ytsaurus, componentManager, updatingComponents)

		if err != nil {
			return ctrl.Result{}, err
		}

		if progressed {
			return ctrl.Result{Requeue: true}, err
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
		// Requeue once again to do final check and maybe update observed generation.
		return ctrl.Result{Requeue: true}, err

	case ytv1.ClusterStateReconfiguration:
		if !componentManager.needInit() {
			logger.Info("Ytsaurus has reconfigured and is running now")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
			// Requeue once again to do final check and maybe update observed generation.
			return ctrl.Result{Requeue: true}, err
		}
	}

	return componentManager.Sync(ctx)
}

func buildAndExecuteFlow(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager, updatingComponents []ytv1.Component) (bool, error) {
	tree := buildFlowTree(updatingComponents)
	ytsaurus.LogUpdate(ctx, fmt.Sprintf("Update flow starting with %s, updating components: %v", ytsaurus.GetUpdateState(), updatingComponents))
	return tree.execute(ctx, ytsaurus, componentManager)
}
