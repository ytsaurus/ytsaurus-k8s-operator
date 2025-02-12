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
		if len(selector.Class) > 0 {
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
		if selector.Component.Type == component.Type {
			if len(selector.Component.Name) == 0 || selector.Component.Name == component.Name {
				return true
			}
		}
	}
	return false
}

// Considers spec and decides if operator should proceed with update or block.
// Block case is indicated with non-empty blockMsg.
// If update is not blocked, updateMeta containing a chosen flow and the component names to update returned.
func chooseUpdatingComponents(spec ytv1.YtsaurusSpec, needUpdate []components.Component, allComponents []components.Component) (components []ytv1.Component, blockMsg string) {
	configuredSelectors := getEffectiveSelectors(spec)
	var canUpdate []ytv1.Component
	var cannotUpdate []ytv1.Component

	for _, comp := range needUpdate {
		component := ytv1.Component{
			Name: comp.GetName(),
			Type: comp.GetType(),
		}
		upd := canUpdateComponent(configuredSelectors, component)
		if upd {
			canUpdate = append(canUpdate, component)
		} else {
			cannotUpdate = append(cannotUpdate, component)
		}
	}

	if len(canUpdate) == 0 {
		if len(cannotUpdate) != 0 {
			return nil, fmt.Sprintf("All components allowed by updateSelector are uptodate, update of {%v} is not allowed", cannotUpdate)
		}
		return nil, "All components are uptodate"
	}

	if len(configuredSelectors) == 1 && configuredSelectors[0].Class == consts.ComponentClassEverything {
		if needFullUpdate(needUpdate) {
			// Here we update not only components that are not up-to-date, but all cluster.
			return convertToComponent(allComponents), ""
		} else {
			return canUpdate, ""
		}
	}
	return canUpdate, ""
}

func needFullUpdate(needUpdate []components.Component) bool {
	statelessSelector := []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassStateless}}
	for _, comp := range needUpdate {
		component := ytv1.Component{
			Name: comp.GetName(),
			Type: comp.GetType(),
		}
		isStateless := canUpdateComponent(statelessSelector, component)
		if !isStateless && component.Type != consts.DataNodeType {
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
			Name: c.GetName(),
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
		switch {
		case !componentManager.needSync():
			logger.Info("Ytsaurus is running and happy")
			// Have passed final check - update observed generation.
			if ytsaurus.SyncObservedGeneration() {
				err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil

		case componentManager.needInit():
			logger.Info("Ytsaurus needs initialization of some components")
			ytsaurus.SyncObservedGeneration()
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateReconfiguration)
			return ctrl.Result{Requeue: true}, err

		case len(needUpdate) != 0:
			var needUpdateNames []string
			for _, c := range needUpdate {
				needUpdateNames = append(needUpdateNames, c.GetName())
			}
			logger = logger.WithValues("componentsForUpdateAll", needUpdateNames)
			updatingComponents, blockMsg := chooseUpdatingComponents(
				ytsaurus.GetResource().Spec, needUpdate, componentManager.allUpdatableComponents())
			if blockMsg != "" {
				logger.Info(blockMsg)
				return ctrl.Result{Requeue: true}, nil
			}
			logger.Info("Ytsaurus needs components update",
				"componentsForUpdateSelected", updatingComponents)
			ytsaurus.SyncObservedGeneration()
			err = ytsaurus.SaveUpdatingClusterState(ctx, updatingComponents)
			if err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}

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
