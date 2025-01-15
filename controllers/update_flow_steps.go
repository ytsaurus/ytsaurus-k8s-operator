package controllers

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type stepCheck struct {
	condition func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool
	onSuccess func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error
}

type flow struct {
	steps map[ytv1.UpdateState][]*stepCheck
}

func buildAndExecuteFlow(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager, updatingComponents []ytv1.Component) (*ctrl.Result, error) {
	flow := buildFlow(updatingComponents)
	executed, err := flow.execute(ctx, ytsaurus, componentManager)
	if err != nil || executed {
		return &ctrl.Result{Requeue: true}, err
	}
	return nil, nil
}

// Executes the flow step for current update state. Returns true if any condition was satisfied.
func (f *flow) execute(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) (bool, error) {
	steps, ok := f.steps[ytsaurus.GetUpdateState()]
	if !ok {
		return false, nil
	}

	for _, step := range steps {
		if step.condition(componentManager, ytsaurus) {
			return true, step.onSuccess(ctx, ytsaurus)
		}
	}

	return false, nil
}

func buildFlow(components []ytv1.Component) *flow {
	flow := &flow{
		steps: make(map[ytv1.UpdateState][]*stepCheck),
	}
	addNoneStep(flow, components)
	addPossibilityCheckStep(flow, components)
	addImpossibleToStartStep(flow, components)
	addWaitingForSafeModeEnabledStep(flow, components)
	addWaitingForTabletCellsSavingStep(flow, components)
	addWaitingForTabletCellsRemovingStartStep(flow, components)
	addWaitingForTabletCellsRemovedStep(flow, components)
	addWaitingForSnapshotsStep(flow, components)
	addWaitingForPodsRemoval(flow, components)
	addWaitingForPodsCreation(flow, components)
	addWaitingForMasterExitReadOnly(flow, components)
	addWaitingForMasterExitReadOnly(flow, components)
	addWaitingForTabletCellsRecovery(flow, components)
	addWaitingForOpArchiveUpdatingPrepare(flow, components)
	addWaitingForOpArchiveUpdate(flow, components)
	addWaitingForQTStateUpdatingPrepare(flow, components)
	addWaitingForQTStateUpdate(flow, components)
	addWaitingForYqlaUpdatingPrepare(flow, components)
	addUpdateStateWaitingForYqlaUpdate(flow, components)
	addWaitingForSafeModeDisabled(flow, components)
	addFinishing(flow, components)
	return flow
}

func addFinishing(flow *flow, components []ytv1.Component) {
	flow.steps[ytv1.UpdateStateFinishing] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return componentManager.allReadyOrUpdating()
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Update finished")
				return ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateUpdateFinishing)
			},
		},
	}
}

func addWaitingForSafeModeDisabled(flow *flow, components []ytv1.Component) {
	if !hasMaster(components) {
		return
	}

	flow.steps[ytv1.UpdateStateWaitingForSafeModeDisabled] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSafeModeDisabled)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Safe mode disabled")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateFinishing)
			},
		},
	}
}

func addUpdateStateWaitingForYqlaUpdate(flow *flow, components []ytv1.Component) {
	if !hasStateless(components) {
		return
	}

	nextState := ytv1.UpdateStateFinishing
	logLine := "Finishing"
	if hasMaster(components) {
		nextState = ytv1.UpdateStateWaitingForSafeModeDisabled
		logLine = "Waiting for safe mode disabled"
	}

	flow.steps[ytv1.UpdateStateWaitingForYqlaUpdate] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionYqlaUpdated)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, logLine)
				return ytsaurus.SaveUpdateState(ctx, nextState)
			},
		},
	}
}

func addWaitingForYqlaUpdatingPrepare(flow *flow, components []ytv1.Component) {
	if !hasStateless(components) {
		return
	}

	nextState := ytv1.UpdateStateFinishing
	logLine := "Finishing"
	if hasMaster(components) {
		nextState = ytv1.UpdateStateWaitingForSafeModeDisabled
		logLine = "Waiting for safe mode disabled"
	}

	flow.steps[ytv1.UpdateStateWaitingForYqlaUpdatingPrepare] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return !componentManager.needYqlAgentUpdate()
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Yql agent update was skipped")
				ytsaurus.LogUpdate(ctx, logLine)
				return ytsaurus.SaveUpdateState(ctx, nextState)
			},
		},
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionYqlaPreparedForUpdating)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Waiting for yql agent env updating to finish")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForYqlaUpdate)
			},
		},
	}
}

func addWaitingForQTStateUpdate(flow *flow, components []ytv1.Component) {
	if !hasStateless(components) {
		return
	}

	flow.steps[ytv1.UpdateStateWaitingForQTStateUpdate] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionQTStateUpdated)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Waiting for yql agent env prepare for updating")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForYqlaUpdatingPrepare)
			},
		},
	}
}

func addWaitingForQTStateUpdatingPrepare(flow *flow, components []ytv1.Component) {
	if !hasStateless(components) {
		return
	}

	flow.steps[ytv1.UpdateStateWaitingForQTStateUpdatingPrepare] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return !componentManager.needQueryTrackerUpdate()
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Query tracker state update was skipped")
				ytsaurus.LogUpdate(ctx, "Waiting for yql agent env prepare for updating")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForYqlaUpdatingPrepare)
			},
		},
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionQTStatePreparedForUpdating)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Waiting for query tracker state updating to finish")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForQTStateUpdate)
			},
		},
	}
}

func addWaitingForOpArchiveUpdate(flow *flow, components []ytv1.Component) {
	if !hasStateless(components) {
		return
	}

	flow.steps[ytv1.UpdateStateWaitingForOpArchiveUpdate] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionOpArchiveUpdated)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Waiting for query tracker state prepare for updating")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForQTStateUpdatingPrepare)
			},
		},
	}
}

func addWaitingForOpArchiveUpdatingPrepare(flow *flow, components []ytv1.Component) {
	if !hasStateless(components) {
		return
	}

	flow.steps[ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return !componentManager.needSchedulerUpdate() || ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionNotNecessaryToUpdateOpArchive)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Operations archive update was skipped")
				ytsaurus.LogUpdate(ctx, "Waiting for query tracker state prepare for updating")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForQTStateUpdatingPrepare)
			},
		},
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionOpArchivePreparedForUpdating)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Waiting for operations archive updating to finish")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForOpArchiveUpdate)
			},
		},
	}
}

func addWaitingForTabletCellsRecovery(flow *flow, components []ytv1.Component) {
	if !hasTablets(components) {
		return
	}

	nextState := ytv1.UpdateStateFinishing
	logLine := "Finishing"
	if hasStateless(components) {
		nextState = ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare
		logLine = "Waiting for operations archive prepare for updating"
	} else if hasMaster(components) {
		nextState = ytv1.UpdateStateWaitingForSafeModeDisabled
		logLine = "Waiting for safe mode disabled"
	}

	flow.steps[ytv1.UpdateStateWaitingForTabletCellsRecovery] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRecovered)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, logLine)
				return ytsaurus.SaveUpdateState(ctx, nextState)
			},
		},
	}
}

func addWaitingForMasterExitReadOnly(flow *flow, components []ytv1.Component) {
	if !hasMaster(components) {
		return
	}

	nextState := ytv1.UpdateStateWaitingForSafeModeDisabled
	if hasTablets(components) {
		nextState = ytv1.UpdateStateWaitingForTabletCellsRecovery
	}

	flow.steps[ytv1.UpdateStateWaitingForMasterExitReadOnly] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionMasterExitedReadOnly)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Masters exited read-only state")
				return ytsaurus.SaveUpdateState(ctx, nextState)
			},
		},
	}
}

func addWaitingForPodsCreation(flow *flow, components []ytv1.Component) {
	nextState := ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare
	logLine := "Waiting for operations archive prepare for updating"
	if hasTablets(components) {
		nextState = ytv1.UpdateStateWaitingForTabletCellsRecovery
		logLine = "Waiting for tablet cells recovery"
	}
	if hasMaster(components) {
		nextState = ytv1.UpdateStateWaitingForMasterExitReadOnly
		logLine = "Masters exited read-only state"
	}

	flow.steps[ytv1.UpdateStateWaitingForPodsCreation] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return componentManager.allReadyOrUpdating()
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "All components were recreated")
				ytsaurus.LogUpdate(ctx, logLine)
				return ytsaurus.SaveUpdateState(ctx, nextState)
			},
		},
	}
}

func addWaitingForPodsRemoval(flow *flow, components []ytv1.Component) {
	flow.steps[ytv1.UpdateStateWaitingForPodsRemoval] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return componentManager.arePodsRemoved()
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Waiting for pods creation")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForPodsCreation)
			},
		},
	}
}

func addWaitingForSnapshotsStep(flow *flow, components []ytv1.Component) {
	if !hasMaster(components) {
		return
	}

	flow.steps[ytv1.UpdateStateWaitingForSnapshots] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSnaphotsSaved)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Waiting for pods removal")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForPodsRemoval)
			},
		},
	}
}

func addWaitingForTabletCellsRemovedStep(flow *flow, components []ytv1.Component) {
	if !hasTablets(components) {
		return
	}

	nextState := ytv1.UpdateStateWaitingForSnapshots
	logLine := "Waiting for snapshots"
	if !hasMaster(components) {
		nextState = ytv1.UpdateStateWaitingForPodsRemoval
		logLine = "Waiting for pods removal"
	}

	flow.steps[ytv1.UpdateStateWaitingForTabletCellsRemoved] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemoved)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, logLine)
				return ytsaurus.SaveUpdateState(ctx, nextState)
			},
		},
	}
}

func addWaitingForTabletCellsRemovingStartStep(flow *flow, components []ytv1.Component) {
	if !hasTablets(components) {
		return
	}

	flow.steps[ytv1.UpdateStateWaitingForTabletCellsRemovingStart] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemovingStarted)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Waiting for tablet cells removed")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsRemoved)
			},
		},
	}
}

func addWaitingForTabletCellsSavingStep(flow *flow, components []ytv1.Component) {
	if !hasTablets(components) {
		return
	}

	flow.steps[ytv1.UpdateStateWaitingForTabletCellsSaving] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsSaved)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Waiting for tablet cells removing to start")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateWaitingForTabletCellsRemovingStart)
			},
		},
	}
}

func addWaitingForSafeModeEnabledStep(flow *flow, components []ytv1.Component) {
	if !hasTablets(components) && !hasMaster(components) {
		return
	}
	nextState := ytv1.UpdateStateWaitingForSnapshots
	logLine := "Waiting for snapshots"
	if hasTablets(components) {
		nextState = ytv1.UpdateStateWaitingForTabletCellsSaving
		logLine = "Waiting for tablet cells removing"
	}

	flow.steps[ytv1.UpdateStateWaitingForSafeModeEnabled] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSafeModeEnabled)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, logLine)
				return ytsaurus.SaveUpdateState(ctx, nextState)
			},
		},
	}
}

func addImpossibleToStartStep(flow *flow, components []ytv1.Component) {
	flow.steps[ytv1.UpdateStateImpossibleToStart] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return !componentManager.needSync() || !ytsaurus.GetResource().Spec.EnableFullUpdate
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Spec changed back or full update isn't enabled, update is canceling")
				return ytsaurus.SaveClusterState(ctx, ytv1.ClusterStateCancelUpdate)
			},
		},
	}
}

func addPossibilityCheckStep(flow *flow, components []ytv1.Component) {
	if isStateless(components) {
		return
	}
	nextState := ytv1.UpdateStateWaitingForSafeModeEnabled
	logLine := "Waiting for safe mode enabled"
	if !hasMaster(components) {
		nextState = ytv1.UpdateStateWaitingForTabletCellsSaving
		logLine = "Waiting for tablet cells saving"
	}

	flow.steps[ytv1.UpdateStatePossibilityCheck] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionHasPossibility)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, logLine)
				return ytsaurus.SaveUpdateState(ctx, nextState)
			},
		},
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionNoPossibility)
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, "Update is impossible, need to apply previous images")
				return ytsaurus.SaveUpdateState(ctx, ytv1.UpdateStateImpossibleToStart)
			},
		},
	}
}

func addNoneStep(flow *flow, components []ytv1.Component) {
	nextState := ytv1.UpdateStateWaitingForPodsRemoval
	logLine := "Waiting for pods removal"
	if !isStateless(components) {
		nextState = ytv1.UpdateStatePossibilityCheck
		logLine = "Checking the possibility of updating"
	}

	flow.steps[ytv1.UpdateStateNone] = []*stepCheck{
		{
			condition: func(componentManager *ComponentManager, ytsaurus *apiProxy.Ytsaurus) bool {
				return true
			},
			onSuccess: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus) error {
				ytsaurus.LogUpdate(ctx, logLine)
				return ytsaurus.SaveUpdateState(ctx, nextState)
			},
		},
	}
}

func hasMaster(components []ytv1.Component) bool {
	if components == nil {
		return true
	}
	for _, component := range components {
		if component.ComponentType == consts.MasterType {
			return true
		}
	}

	return false
}

func hasTablets(components []ytv1.Component) bool {
	if components == nil {
		return true
	}

	for _, component := range components {
		if component.ComponentType == consts.TabletNodeType {
			return true
		}
	}

	return false
}

func hasStateless(components []ytv1.Component) bool {
	if components == nil {
		return true
	}

	for _, component := range components {
		if component.ComponentType != consts.MasterType && component.ComponentType != consts.TabletNodeType {
			return true
		}
	}
	return false
}

func isStateless(components []ytv1.Component) bool {
	return !hasMaster(components) && !hasTablets(components)
}
