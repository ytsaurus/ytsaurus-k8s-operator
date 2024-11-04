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

func getFlowFromComponent(component consts.ComponentType) ytv1.UpdateFlow {
	if component == consts.MasterType {
		return ytv1.UpdateFlowMaster
	}
	if component == consts.TabletNodeType {
		return ytv1.UpdateFlowTabletNodes
	}
	if component == consts.DataNodeType || component == consts.ExecNodeType {
		return ytv1.UpdateFlowFull
	}
	return ytv1.UpdateFlowStateless
}

func canUpdateComponent(selectors []ytv1.ComponentUpdateSelector, component ytv1.Component) (bool, error) {
	for _, selector := range selectors {
		if selector.ComponentType != "" {
			if selector.ComponentType == component.Type {
				return true, nil
			}
		} else if selector.ComponentName != "" {
			if selector.ComponentName == component.Name {
				return true, nil
			}
		} else {
			switch selector.ComponentGroup {
			case consts.ComponentGroupEverything:
				return true, nil
			case consts.ComponentGroupNothing:
				return false, nil
			case consts.ComponentGroupStateful:
				if component.Type == consts.DataNodeType || component.Type == consts.TabletNodeType {
					return true, nil
				}
			case consts.ComponentGroupStateless:
				if component.Type != consts.DataNodeType && component.Type != consts.TabletNodeType && component.Type != consts.MasterType {
					return true, nil
				}
			default:
				return false, fmt.Errorf("unexpected component group %s", selector.ComponentGroup)
			}
		}
	}
	return false, nil
}

// Considers spec and decides if operator should proceed with update or block.
// Block case is indicated with non-empty blockMsg.
// If update is not blocked, updateMeta containing a chosen flow and the component names to update returned.
func chooseUpdatingComponents(spec ytv1.YtsaurusSpec, needUpdate []components.Component, allComponents []components.Component) (components []ytv1.Component, blockMsg string) {
	configuredSelectors := spec.UpdateSelectors
	if len(configuredSelectors) == 0 && spec.EnableFullUpdate {
		configuredSelectors = []ytv1.ComponentUpdateSelector{{ComponentGroup: consts.ComponentGroupEverything}}
	}
	needFullUpdate := false

	var canUpdate []ytv1.Component
	var cannotUpdate []ytv1.Component

	for _, comp := range needUpdate {
		component := ytv1.Component{
			Name: comp.GetName(),
			Type: comp.GetType(),
		}
		upd, err := canUpdateComponent(configuredSelectors, component)
		if err != nil {
			return nil, err.Error()
		}
		if upd {
			canUpdate = append(canUpdate, component)
		} else {
			cannotUpdate = append(cannotUpdate, component)
		}
		statelessCheck, err := canUpdateComponent([]ytv1.ComponentUpdateSelector{{ComponentGroup: consts.ComponentGroupStateless}}, component)
		if !statelessCheck && component.Type != consts.DataNodeType {
			needFullUpdate = true
		}
	}

	if len(canUpdate) == 0 {
		if len(cannotUpdate) != 0 {
			return nil, fmt.Sprintf("All components allowed by updateSelector are uptodate, update of {%v} is not allowed", cannotUpdate)
		}
		return nil, "All components are uptodate"
	}

	if len(configuredSelectors) == 1 && configuredSelectors[0].ComponentGroup == consts.ComponentGroupEverything {
		if needFullUpdate {
			return convertToComponent(allComponents), ""
		} else {
			return canUpdate, ""
		}
	}
	return canUpdate, ""
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
