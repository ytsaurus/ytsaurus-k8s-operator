package controllers

import (
	"context"
	"fmt"
	"time"

	"k8s.io/utils/ptr"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/validators"

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

// Considers splits all the components in two groups: ones that can be updated and ones which update is blocked.
func chooseUpdatingComponents(selectors []ytv1.ComponentUpdateSelector, needUpdate []ytv1.Component, allComponents []ytv1.Component) (canUpdate []ytv1.Component, cannotUpdate []ytv1.Component) {
	for _, component := range needUpdate {
		upd := canUpdateComponent(selectors, component)
		if upd {
			canUpdate = append(canUpdate, component)
		} else {
			cannotUpdate = append(cannotUpdate, component)
		}
	}

	if len(canUpdate) == 0 {
		return nil, cannotUpdate
	}
	if hasEverythingSelector(selectors) && needFullUpdate(needUpdate) {
		// if image wasn't changed, we don't need to run ImageHeater
		if !hasComponent(needUpdate, consts.ImageHeaterType) {
			filtered := make([]ytv1.Component, 0, len(allComponents))
			for _, component := range allComponents {
				if component.Type == consts.ImageHeaterType {
					continue
				}
				filtered = append(filtered, component)
			}
			return filtered, nil
		}
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
	// Update plan is defined in spec.
	if len(spec.UpdatePlan) != 0 {
		return spec.UpdatePlan
	}

	// Generate effective plan from legacy options.
	switch spec.UpdateSelector {
	case ytv1.UpdateSelectorNothing:
		return []ytv1.ComponentUpdateSelector{{
			Class: consts.ComponentClassNothing,
		}}
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
		return []ytv1.ComponentUpdateSelector{{
			Class: consts.ComponentClassStateless,
		}}
	case ytv1.UpdateSelectorEverything:
		return []ytv1.ComponentUpdateSelector{{
			Class: consts.ComponentClassEverything,
		}}
	case ytv1.UpdateSelectorUnspecified:
		if !ptr.Deref(spec.EnableFullUpdate, true) {
			return []ytv1.ComponentUpdateSelector{{
				Class: consts.ComponentClassStateless,
			}}
		}
		// Else: fail back to default plan below.
	default:
		// Safe default when seeing something unknown.
		return []ytv1.ComponentUpdateSelector{{
			Class: consts.ComponentClassNothing,
		}}
	}

	// This is default update plan.
	return []ytv1.ComponentUpdateSelector{{
		Class: consts.ComponentClassEverything,
	}}
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
	logger := log.FromContext(ctx, "ytsaurusState", resource.Status.State)
	ctx = log.IntoContext(ctx, logger)

	if !ptr.Deref(resource.Spec.IsManaged, true) || r.ShouldIgnoreResource(ctx, resource) {
		logger.Info("Ytsaurus cluster is not managed by controller, do nothing")
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	if err := validators.ValidateVersionConstraint(resource.Spec.RequiresOperatorVersion); err != nil {
		logger.Error(err, "Operator version does not satisfy spec version constraint")
		return ctrl.Result{}, err
	}

	ytsaurus := apiProxy.NewYtsaurus(resource, r.Client, r.Recorder, r.Scheme)
	componentManager, err := NewComponentManager(ctx, ytsaurus, r.ClusterDomain)
	if err != nil {
		logger.Error(err, "Cannot build component manager")
		return ctrl.Result{Requeue: true}, err
	}

	err = componentManager.FetchStatus(ctx)
	if err != nil {
		logger.Error(err, "Cannot fetch component manager status")
		return ctrl.Result{Requeue: true}, err
	}

	switch ytsaurus.GetClusterState() {
	case ytv1.ClusterStateCreated, "":
		logger.Info("Ytsaurus is just created and needs initialization")
		err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateInitializing)
		return ctrl.Result{Requeue: true}, err

	case ytv1.ClusterStateInitializing:
		// Ytsaurus has finished initializing, and is running now.
		if componentManager.status.allRunning {
			logger.Info("Ytsaurus has synced and is running now")
			err := ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateRunning)
			return ctrl.Result{Requeue: true}, err
		}

	case ytv1.ClusterStateRunning, ytv1.ClusterStateReconfiguration:
		spec := ytsaurus.GetResource().Spec
		needUpdate := componentManager.status.needUpdate
		selectors := getEffectiveSelectors(spec)
		canUpdate, cannotUpdate := chooseUpdatingComponents(
			selectors,
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
		case componentManager.status.allReady:
			logger.Info("Ytsaurus is running and happy")
			if ytsaurus.SetClusterState(ytv1.ClusterStateRunning) {
				needStatusUpdate = true
			}

		case !componentManager.status.allRunning:
			logger.Info("Ytsaurus needs initialization of some components")
			if ytsaurus.SetClusterState(ytv1.ClusterStateReconfiguration) {
				needStatusUpdate = true
			}

		case len(needUpdate) == 0:
			logger.Info("All components are up-to-date")
			if ytsaurus.SetClusterState(ytv1.ClusterStateRunning) {
				needStatusUpdate = true
			}

		case len(canUpdate) != 0:
			logger.Info("Ytsaurus components needs update", "canUpdate", canUpdate, "cannotUpdate", cannotUpdate)
			// We do not update BlockedComponentsSummary here, it should be updated first thing in Running state.
			ytsaurus.SetUpdatingComponents(canUpdate)
			ytsaurus.SetUpdateState(ytv1.UpdateStateNone)
			ytsaurus.SetClusterState(ytv1.ClusterStateUpdating)
			needStatusUpdate = true

		case len(cannotUpdate) != 0:
			logger.Info("Ytsaurus components update is blocked", "cannotUpdate", cannotUpdate)
			if ytsaurus.SetClusterState(ytv1.ClusterStateRunning) {
				needStatusUpdate = true
			}
		}

		// Have passed final check - save status update if needed.
		if needStatusUpdate {
			err := ytsaurus.UpdateStatus(ctx)
			return ctrl.Result{Requeue: true}, err
		}

		if ytsaurus.IsRunning() {
			// All done, nothing change - do not requeue reconcile.
			return ctrl.Result{}, nil
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

	default:
		return ctrl.Result{}, fmt.Errorf("unknown cluster state: %q", ytsaurus.GetClusterState())
	}

	return componentManager.Sync(ctx)
}

func buildAndExecuteFlow(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager, updatingComponents []ytv1.Component) (bool, error) {
	tree := buildFlowTree(updatingComponents)
	ytsaurus.LogUpdate(ctx, fmt.Sprintf("Update flow starting with %s, updating components: %v", ytsaurus.GetUpdateState(), updatingComponents))
	return tree.execute(ctx, ytsaurus, componentManager)
}
