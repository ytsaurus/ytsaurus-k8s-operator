package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
)

func canUpdateComponent(selector ytv1.UpdateSelector, component consts.ComponentType) bool {
	switch selector {
	case ytv1.UpdateSelectorNothing:
		return false
	case ytv1.UpdateSelectorMasterOnly:
		return component == consts.MasterType
	case ytv1.UpdateSelectorDataNodesOnly:
		return component == consts.DataNodeType
	case ytv1.UpdateSelectorTabletNodesOnly:
		return component == consts.TabletNodeType
	case ytv1.UpdateSelectorExecNodesOnly:
		return component == consts.ExecNodeType
	case ytv1.UpdateSelectorStatelessOnly:
		switch component {
		case consts.MasterType:
			return false
		case consts.DataNodeType:
			return false
		case consts.TabletNodeType:
			return false
		}
		return true
	case ytv1.UpdateSelectorEverything:
		return true
	default:
		return false
	}
}

// Considers spec and decides if operator should proceed with update or block.
// Block case is indicated with non-empty blockMsg.
// If update is not blocked, updateMeta containing a chosen flow and the component names to update returned.
func chooseUpdatingComponents(spec ytv1.YtsaurusSpec, needUpdate []components.Component) (components []ytv1.Component, blockMsg string) {
	configuredSelector := spec.UpdateSelector
	if configuredSelector == ytv1.UpdateSelectorUnspecified {
		if spec.EnableFullUpdate {
			configuredSelector = ytv1.UpdateSelectorEverything
		} else {
			configuredSelector = ytv1.UpdateSelectorStatelessOnly
		}
	}

	var canUpdate []ytv1.Component
	var cannotUpdate []string
	needFullUpdate := false

	for _, comp := range needUpdate {
		component := ytv1.Component{
			Name: comp.GetName(),
			Type: comp.GetType(),
		}
		if canUpdateComponent(configuredSelector, component.Type) {
			canUpdate = append(canUpdate, component)
		} else {
			cannotUpdate = append(cannotUpdate, component.Name)
		}
		if !canUpdateComponent(ytv1.UpdateSelectorStatelessOnly, component.Type) && component.Type != consts.DataNodeType {
			needFullUpdate = true
		}
	}

	if len(canUpdate) == 0 {
		if len(cannotUpdate) != 0 {
			return nil, fmt.Sprintf("All components allowed by updateSelector are uptodate, update of {%s} is not allowed", strings.Join(cannotUpdate, ", "))
		}
		return nil, "All components are uptodate"
	}

	switch configuredSelector {
	case ytv1.UpdateSelectorEverything:
		if needFullUpdate {
			return nil, ""
		} else {
			return canUpdate, ""
		}
	case ytv1.UpdateSelectorMasterOnly:
		return canUpdate, ""
	case ytv1.UpdateSelectorTabletNodesOnly:
		return canUpdate, ""
	case ytv1.UpdateSelectorDataNodesOnly, ytv1.UpdateSelectorExecNodesOnly, ytv1.UpdateSelectorStatelessOnly:
		return canUpdate, ""
	default:
		return nil, fmt.Sprintf("Unexpected update selector %s", configuredSelector)
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
			updatingComponents, blockMsg := chooseUpdatingComponents(ytsaurus.GetResource().Spec, needUpdate)
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
	allComponents := convertToYtComponents(componentManager.allComponents)
	tree := buildFlowTree(updatingComponents, allComponents)
	ytsaurus.LogUpdate(ctx, fmt.Sprintf("Update flow starting with %s, updating components: %v, all components: %v", ytsaurus.GetUpdateState(), updatingComponents, allComponents))
	return tree.execute(ctx, ytsaurus, componentManager)
}

func convertToYtComponents(components []components.Component) []ytv1.Component {
	result := make([]ytv1.Component, len(components))
	for i, c := range components {
		result[i] = ytv1.Component{
			Type: c.GetType(),
		}
	}
	return result
}
