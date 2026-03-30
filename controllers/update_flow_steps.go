package controllers

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type (
	stepResultMark string
)

const (
	stepResultMarkUnsatisfied stepResultMark = ""
	stepResultMarkHappy       stepResultMark = "happy"
	stepResultMarkUnhappy     stepResultMark = "unhappy"
)

var terminateTransitions = map[ytv1.UpdateState]ytv1.ClusterState{
	ytv1.UpdateStateImpossibleToStart: ytv1.ClusterStateUpdateCanceled,
}

type flowCondition func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark

func flowCheckStatusCondition(conditionName string) flowCondition {
	return func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if ytsaurus.IsUpdateStatusConditionTrue(conditionName) {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	}
}

func flowUpdateStateCondition(updateState ytv1.UpdateState) flowCondition {
	return func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if ytsaurus.IsUpdateStatusConditionTrue(ytsaurus.GetUpdateStateCompleteCondition(updateState)) {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	}
}

type flowStep struct {
	updateState ytv1.UpdateState
	// For most of the steps, there will be only one next step,
	// but for some outcome is based on condition result.
	nextSteps map[stepResultMark]*flowStep
}

func newSimpleStep(updateState ytv1.UpdateState) *flowStep {
	return &flowStep{
		updateState: updateState,
	}
}

func newConditionalForkStep(updateState ytv1.UpdateState, unhappyNext *flowStep) *flowStep {
	return &flowStep{
		updateState: updateState,
		nextSteps: map[stepResultMark]*flowStep{
			stepResultMarkUnhappy: unhappyNext,
		},
	}
}

func (s *flowStep) checkCondition(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
	condition := flowConditions[s.updateState]
	if condition == nil {
		log.FromContext(ctx).Info("Update flow state conditions are not defined", "updateState", s.updateState)
		return stepResultMarkUnsatisfied
	}
	return condition(ctx, ytsaurus, componentManager)
}

func (s *flowStep) next(mark stepResultMark) *flowStep {
	if mark == stepResultMarkUnsatisfied {
		return nil
	}
	return s.nextSteps[mark]
}

func (s *flowStep) addHappyPath(next *flowStep) *flowStep {
	if s.nextSteps == nil {
		s.nextSteps = make(map[stepResultMark]*flowStep)
	}
	s.nextSteps[stepResultMarkHappy] = next
	return next
}

func (s *flowStep) chain(steps ...*flowStep) *flowStep {
	current := s
	for _, step := range steps {
		current.addHappyPath(step)
		current = step
	}
	return current
}

// for the fast lookup of the step by the update state
// otherwise we can traverse the graph each time
type flowTree struct {
	index map[ytv1.UpdateState]*flowStep
	head  *flowStep
	tail  *flowStep
}

func newFlowTree(head *flowStep) *flowTree {
	return &flowTree{
		index: map[ytv1.UpdateState]*flowStep{
			head.updateState: head,
		},
		head: head,
		tail: head,
	}
}

/* Execute the flow tree starting from the current state of the Ytsaurus.
* Returns true if update state progressed to another state, false otherwise.
 */
func (f *flowTree) execute(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) (bool, error) {
	var err error
	currentState := ytsaurus.GetUpdateState()
	currentStep := f.index[currentState]
	if currentStep == nil {
		return false, fmt.Errorf("unexpected update state: %q", currentState)
	}

	// will execute one step at a time
	mark := currentStep.checkCondition(ctx, ytsaurus, componentManager)
	// condition is not met, wait for the next update
	if mark == stepResultMarkUnsatisfied {
		ytsaurus.LogUpdate(ctx, fmt.Sprintf("Update flow: condition not met for %s", currentState))
		return false, nil
	}

	nextStep := currentStep.next(mark)
	if nextStep == nil {
		// executed the last step in the flow — setting the cluster state
		clusterState := terminateTransitions[currentState]
		if clusterState == "" {
			clusterState = ytv1.ClusterStateUpdateFinished
		}
		ytsaurus.LogUpdate(
			ctx,
			fmt.Sprintf("Update flow finishing with cluster state: %s", clusterState),
		)
		err = ytsaurus.SaveClusterState(ctx, clusterState)
		if err != nil {
			return false, err
		}
		return true, nil
	}

	ytsaurus.LogUpdate(
		ctx,
		fmt.Sprintf("Update flow advance %s -> %s", currentState, nextStep.updateState),
	)

	err = ytsaurus.SaveUpdateState(ctx, nextStep.updateState)
	return false, err
}

func (f *flowTree) chain(steps ...*flowStep) *flowTree {
	for _, step := range steps {
		f.index[step.updateState] = step
		// Also index any unhappy path steps
		if step.nextSteps != nil {
			if unhappyStep := step.nextSteps[stepResultMarkUnhappy]; unhappyStep != nil {
				f.index[unhappyStep.updateState] = unhappyStep
			}
		}
		f.tail.chain(step)
		f.tail = step
	}
	return f
}

func (f *flowTree) chainIf(cond bool, steps ...*flowStep) *flowTree {
	if cond {
		return f.chain(steps...)
	}
	return f
}

var flowConditions = map[ytv1.UpdateState]flowCondition{
	ytv1.UpdateStateNone: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		return stepResultMarkHappy
	},
	ytv1.UpdateStateWaitingForImageHeater: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if componentManager.allUpdatingImagesAreHeated() {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStatePossibilityCheck: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionHasPossibility) {
			return stepResultMarkHappy
		} else if ytsaurus.IsUpdateStatusConditionFalse(consts.ConditionHasPossibility) {
			return stepResultMarkUnhappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateImpossibleToStart: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if componentManager.status.allReady {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForSafeModeEnabled:             flowCheckStatusCondition(consts.ConditionSafeModeEnabled),
	ytv1.UpdateStateWaitingForTabletCellsSaving:           flowCheckStatusCondition(consts.ConditionTabletCellsSaved),
	ytv1.UpdateStateWaitingForTabletCellsRemovingStart:    flowCheckStatusCondition(consts.ConditionTabletCellsRemovingStarted),
	ytv1.UpdateStateWaitingForTabletCellsRemoved:          flowCheckStatusCondition(consts.ConditionTabletCellsRemoved),
	ytv1.UpdateStateWaitingForImaginaryChunksAbsence:      flowCheckStatusCondition(consts.ConditionDataNodesWithImaginaryChunksAbsent),
	ytv1.UpdateStateWaitingForSnapshots:                   flowCheckStatusCondition(consts.ConditionSnaphotsSaved),
	ytv1.UpdateStateWaitingForTabletCellsRecovery:         flowCheckStatusCondition(consts.ConditionTabletCellsRecovered),
	ytv1.UpdateStateWaitingForOpArchiveUpdate:             flowUpdateStateCondition(ytv1.UpdateStateWaitingForOpArchiveUpdate),
	ytv1.UpdateStateWaitingForSidecarsInitializingPrepare: nil, // TODO: Remove, it holds alignment.
	ytv1.UpdateStateWaitingForSidecarsInitialize:          flowUpdateStateCondition(ytv1.UpdateStateWaitingForSidecarsInitialize),
	ytv1.UpdateStateWaitingForQTStateUpdatingPrepare:      flowCheckStatusCondition(consts.ConditionQTStatePreparedForUpdating),
	ytv1.UpdateStateWaitingForQTStateUpdate:               flowCheckStatusCondition(consts.ConditionQTStateUpdated),
	ytv1.UpdateStateWaitingForYqlaUpdate:                  flowUpdateStateCondition(ytv1.UpdateStateWaitingForYqlaUpdate),
	ytv1.UpdateStateWaitingForQAStateUpdatingPrepare:      flowCheckStatusCondition(consts.ConditionQAStatePreparedForUpdating),
	ytv1.UpdateStateWaitingForQAStateUpdate:               flowCheckStatusCondition(consts.ConditionQAStateUpdated),
	ytv1.UpdateStateWaitingForSafeModeDisabled:            flowCheckStatusCondition(consts.ConditionSafeModeDisabled),
	ytv1.UpdateStateWaitingForMasterReady: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if componentManager.status.masterReady {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForMasterEnterReadOnly:      flowUpdateStateCondition(ytv1.UpdateStateWaitingForMasterEnterReadOnly),
	ytv1.UpdateStateWaitingForMasterExitReadOnly:       flowUpdateStateCondition(ytv1.UpdateStateWaitingForMasterExitReadOnly),
	ytv1.UpdateStateWaitingForMasterCellsPreparation:   flowUpdateStateCondition(ytv1.UpdateStateWaitingForMasterCellsPreparation),
	ytv1.UpdateStateWaitingForMasterCellsEnterReadOnly: flowUpdateStateCondition(ytv1.UpdateStateWaitingForMasterCellsEnterReadOnly),
	ytv1.UpdateStateWaitingForMasterCellsExitReadOnly:  flowUpdateStateCondition(ytv1.UpdateStateWaitingForMasterCellsExitReadOnly),
	ytv1.UpdateStateWaitingForMasterCellsRegistration:  flowUpdateStateCondition(ytv1.UpdateStateWaitingForMasterCellsRegistration),
	ytv1.UpdateStateWaitingForMasterCellsSettlement:    flowUpdateStateCondition(ytv1.UpdateStateWaitingForMasterCellsSettlement),
	ytv1.UpdateStateWaitingForMasterCellsCompletion:    flowUpdateStateCondition(ytv1.UpdateStateWaitingForMasterCellsCompletion),
	ytv1.UpdateStateWaitingForCypressPatch:             flowCheckStatusCondition(consts.ConditionCypressPatchApplied),
	ytv1.UpdateStateWaitingForTimbertruckPrepared: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if ytsaurus.GetResource().Spec.PrimaryMasters.Timbertruck == nil || ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTimbertruckPrepared) {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForPodsRemoval: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if componentManager.arePodsRemoved() {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForPodsCreation: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if componentManager.status.allStarted {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
}

// TODO: Must be part of ComponentManager.
func buildFlowTree(componentManager *ComponentManager) *flowTree {
	updatingComponents := componentManager.status.nowUpdating
	st := newSimpleStep
	head := st(ytv1.UpdateStateNone)
	tree := newFlowTree(head)

	updMaster := hasComponent(updatingComponents, consts.MasterType)
	updTablet := hasComponent(updatingComponents, consts.TabletNodeType)
	updDataNodes := hasComponent(updatingComponents, consts.DataNodeType)
	updScheduler := hasComponent(updatingComponents, consts.SchedulerType)
	updQueryTracker := hasComponent(updatingComponents, consts.QueryTrackerType)
	updYqlAgent := hasComponent(updatingComponents, consts.YqlAgentType)
	updQueueAgent := hasComponent(updatingComponents, consts.QueueAgentType)

	// TODO: if validation conditions can be not mentioned here or needed
	tree.chainIf(
		componentManager.getHeaterStatus != nil,
		st(ytv1.UpdateStateWaitingForImageHeater),
	).chainIf(
		(updMaster || updTablet) && !componentManager.status.mastersMaintenance,
		newConditionalForkStep(
			ytv1.UpdateStatePossibilityCheck,
			// This is the unhappy path.
			st(ytv1.UpdateStateImpossibleToStart),
			// Happy path will be chained automatically.
		),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForSafeModeEnabled),
	).chainIf(
		updTablet && !componentManager.status.shutdownTablets,
		st(ytv1.UpdateStateWaitingForTabletCellsSaving),
		st(ytv1.UpdateStateWaitingForTabletCellsRemovingStart),
		st(ytv1.UpdateStateWaitingForTabletCellsRemoved),
	).chainIf(
		updDataNodes || updMaster,
		st(ytv1.UpdateStateWaitingForImaginaryChunksAbsence),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForSnapshots),
	).chain(
		st(ytv1.UpdateStateWaitingForPodsRemoval),
		st(ytv1.UpdateStateWaitingForPodsCreation),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForMasterReady),
		st(ytv1.UpdateStateWaitingForMasterExitReadOnly),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForSidecarsInitialize),
	).chain(
		st(ytv1.UpdateStateWaitingForCypressPatch),
	).chainIf(
		updTablet && !componentManager.status.shutdownTablets,
		st(ytv1.UpdateStateWaitingForTabletCellsRecovery),
	).chainIf(
		updScheduler && !componentManager.status.clusterMaintenance,
		st(ytv1.UpdateStateWaitingForOpArchiveUpdate),
	).chainIf(
		updQueryTracker && !componentManager.status.clusterMaintenance,
		st(ytv1.UpdateStateWaitingForQTStateUpdatingPrepare),
		st(ytv1.UpdateStateWaitingForQTStateUpdate),
	).chainIf(
		updYqlAgent && !componentManager.status.clusterMaintenance,
		st(ytv1.UpdateStateWaitingForYqlaUpdate),
	).chainIf(
		updQueueAgent && !componentManager.status.clusterMaintenance,
		st(ytv1.UpdateStateWaitingForQAStateUpdatingPrepare),
		st(ytv1.UpdateStateWaitingForQAStateUpdate),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForSafeModeDisabled),
	).chain(
		st(ytv1.UpdateStateWaitingForTimbertruckPrepared),
	)

	return tree
}

func masterMaintenanceFlow(componentManager *ComponentManager) *flowTree {
	updatingComponents := componentManager.status.nowUpdating
	updMaster := hasComponent(updatingComponents, consts.MasterType)

	st := newSimpleStep
	tree := newFlowTree(st(ytv1.UpdateStateNone))

	// Master new master cells are added in three passes:
	// - Reconfiguration: new secondary master instances are started by cluster reconfiguration
	// - Registration: old masters are updated to register new secondary master cells without roles
	// - Settlement: all masters are updated to build common snapshot for all cells and assign roles
	// Workflow depends on status update conditions which reflects cluster status at begin of update.
	// Link: https://ytsaurus.tech/docs/en/admin-guide/cell-addition

	tree.chainIf(
		// Prepare for adding new master cell:
		// - disable chunk refresh
		// - allow secondary master cells without roles
		updMaster && componentManager.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionMasterCellsRegistration),
		st(ytv1.UpdateStateWaitingForMasterCellsPreparation),
	).chainIf(
		// Build snapshot, update configs and restart masters:
		// - old masters for cell registration (dynamic propagation)
		// - all masters for cell settlement (static propagation)
		updMaster,
		st(ytv1.UpdateStateWaitingForMasterCellsEnterReadOnly),
	).chain(
		st(ytv1.UpdateStateWaitingForPodsRemoval),
		st(ytv1.UpdateStateWaitingForPodsCreation),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForMasterCellsExitReadOnly),
	).chainIf(
		// Waiting for registration of new master cells.
		// Initial master cells settlement pass.
		updMaster && componentManager.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionMasterCellsRegistration),
		st(ytv1.UpdateStateWaitingForMasterCellsRegistration),
	).chainIf(
		// Checks that there is no dynamically propagated master cells then do master cells settlement:
		// - assigns roles to new master cells
		// - mark new master cells as settled in cluster status conditions
		updMaster && componentManager.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionMasterCellsSettlement),
		st(ytv1.UpdateStateWaitingForMasterCellsSettlement),
	).chainIf(
		updMaster,
		// Finish master cells reconfiguration:
		// - enable chunk refresh
		st(ytv1.UpdateStateWaitingForMasterCellsCompletion),
	)

	return tree
}

func hasComponent(updatingComponents []ytv1.Component, componentType consts.ComponentType) bool {
	for _, component := range updatingComponents {
		if component.Type == componentType {
			return true
		}
	}

	return false
}
