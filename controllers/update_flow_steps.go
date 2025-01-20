package controllers

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
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

func buildAndExecuteFlow(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager, updatingComponents []ytv1.Component) (*ctrl.Result, error) {
	tree := buildFlowTree(updatingComponents, componentManager.allComponents)
	_, err := tree.execute(ctx, ytsaurus, componentManager)
	return &ctrl.Result{Requeue: true}, err

}

var terminateTransitions = map[ytv1.UpdateState]ytv1.ClusterState{
	ytv1.UpdateStateImpossibleToStart: ytv1.ClusterStateCancelUpdate,
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
	return condition(ctx, ytsaurus, componentManager)
}

func (s *flowStep) next(mark stepResultMark) *flowStep {
	if mark == stepResultMarkUnsatisfied {
		return nil
	}
	return s.nextSteps[mark]
}

func (s *flowStep) addHappyPath(next *flowStep) *flowStep {
	s.nextSteps = map[stepResultMark]*flowStep{
		stepResultMarkHappy: next,
	}
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

	deferredChain ytv1.UpdateState
}

func newFlowTree(head *flowStep) *flowTree {
	return &flowTree{
		index: make(map[ytv1.UpdateState]*flowStep),
		head:  head,
		tail:  head,
	}
}

func (f *flowTree) execute(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) (bool, error) {
	var err error
	currentState := ytsaurus.GetUpdateState()
	currentStep := f.index[currentState]

	// will execute one step at a time
	mark := currentStep.checkCondition(ctx, ytsaurus, componentManager)
	// condition is not met, wait for the next update
	if mark == stepResultMarkUnsatisfied {
		return false, nil
	}

	nextStep := currentStep.next(mark)
	if nextStep == nil {
		// executed the last step in the flow — setting the cluster state
		clusterState := terminateTransitions[currentState]
		if clusterState == "" {
			clusterState = ytv1.ClusterStateUpdateFinishing
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
	if f.deferredChain != "" {
		if len(steps) != 0 {
			f.index[f.deferredChain].chain(steps[0])
			f.deferredChain = ""
		}
	}
	for _, step := range steps {
		f.index[step.updateState] = step
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
	ytv1.UpdateStatePossibilityCheck: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionHasPossibility) {
			return stepResultMarkHappy
		} else if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionNoPossibility) {
			return stepResultMarkUnhappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateImpossibleToStart: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if !componentManager.needSync() || !ytsaurus.GetResource().Spec.EnableFullUpdate {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForSafeModeEnabled:          flowCheckStatusCondition(consts.ConditionSafeModeEnabled),
	ytv1.UpdateStateWaitingForTabletCellsSaving:        flowCheckStatusCondition(consts.ConditionTabletCellsSaved),
	ytv1.UpdateStateWaitingForTabletCellsRemovingStart: flowCheckStatusCondition(consts.ConditionTabletCellsRemovingStarted),
	ytv1.UpdateStateWaitingForTabletCellsRemoved:       flowCheckStatusCondition(consts.ConditionTabletCellsRemoved),
	ytv1.UpdateStateWaitingForEnableRealChunkLocations: flowCheckStatusCondition(consts.ConditionRealChunkLocationsEnabled),
	ytv1.UpdateStateWaitingForSnapshots:                flowCheckStatusCondition(consts.ConditionSnaphotsSaved),
	ytv1.UpdateStateWaitingForTabletCellsRecovery:      flowCheckStatusCondition(consts.ConditionTabletCellsRecovered),
	ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare: flowCheckStatusCondition(consts.ConditionOpArchivePreparedForUpdating),
	ytv1.UpdateStateWaitingForOpArchiveUpdate:          flowCheckStatusCondition(consts.ConditionOpArchiveUpdated),
	ytv1.UpdateStateWaitingForQTStateUpdatingPrepare:   flowCheckStatusCondition(consts.ConditionQTStatePreparedForUpdating),
	ytv1.UpdateStateWaitingForQTStateUpdate:            flowCheckStatusCondition(consts.ConditionQTStateUpdated),
	ytv1.UpdateStateWaitingForYqlaUpdatingPrepare:      flowCheckStatusCondition(consts.ConditionYqlaPreparedForUpdating),
	ytv1.UpdateStateWaitingForYqlaUpdate:               flowCheckStatusCondition(consts.ConditionYqlaUpdated),
	ytv1.UpdateStateWaitingForSafeModeDisabled:         flowCheckStatusCondition(consts.ConditionSafeModeDisabled),
	ytv1.UpdateStateWaitingForPodsRemoval: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if componentManager.areNonMasterPodsRemoved() {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForPodsCreation: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if componentManager.nonMasterReadyOrUpdating() {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForMasterPodsRemoval: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if componentManager.areMasterPodsRemoved() {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForMasterPodsCreation: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if componentManager.masterReadyOrUpdating() {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
}

func buildFlowTree(updatingComponents []ytv1.Component, allComponents []components.Component) *flowTree {
	st := newSimpleStep
	head := st(ytv1.UpdateStateNone)
	tree := newFlowTree(head)

	updMaster := hasComponent(updatingComponents, allComponents, consts.MasterType)
	updTablet := hasComponent(updatingComponents, allComponents, consts.TabletNodeType)
	updMasterOrTablet := updMaster || updTablet
	updScheduler := hasComponent(updatingComponents, allComponents, consts.SchedulerType)
	updQueryTracker := hasComponent(updatingComponents, allComponents, consts.QueryTrackerType)
	updYqlAgent := hasComponent(updatingComponents, allComponents, consts.YqlAgentType)

	// TODO: if validation conditions can be not mentioned here or needed
	tree.chain(
		newConditionalForkStep(
			ytv1.UpdateStatePossibilityCheck,
			// This is the unhappy path.
			st(ytv1.UpdateStateImpossibleToStart),
			// Happy path will be chained automatically.
		),
	).chainIf(
		updMasterOrTablet,
		st(ytv1.UpdateStateWaitingForSafeModeEnabled),
	).chainIf(
		updTablet,
		st(ytv1.UpdateStateWaitingForTabletCellsSaving),
		st(ytv1.UpdateStateWaitingForTabletCellsRemovingStart),
		st(ytv1.UpdateStateWaitingForTabletCellsRemoved),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForEnableRealChunkLocations),
	).chain(
		st(ytv1.UpdateStateWaitingForPodsRemoval),
		st(ytv1.UpdateStateWaitingForPodsCreation),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForSnapshots),
		st(ytv1.UpdateStateWaitingForMasterPodsRemoval),
		st(ytv1.UpdateStateWaitingForMasterPodsCreation),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForMasterExitReadOnly),
	).chainIf(
		updTablet,
		st(ytv1.UpdateStateWaitingForTabletCellsRecovery),
	).chainIf(
		updScheduler,
		st(ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare),
		st(ytv1.UpdateStateWaitingForOpArchiveUpdate),
	).chainIf(
		updQueryTracker,
		st(ytv1.UpdateStateWaitingForQTStateUpdatingPrepare),
		st(ytv1.UpdateStateWaitingForQTStateUpdate),
	).chainIf(
		updYqlAgent,
		st(ytv1.UpdateStateWaitingForYqlaUpdatingPrepare),
		st(ytv1.UpdateStateWaitingForYqlaUpdate),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForSafeModeDisabled),
	)

	return tree
}

func hasComponent(updatingComponents []ytv1.Component, allComponents []components.Component, componentType consts.ComponentType) bool {
	if updatingComponents == nil {
		for _, component := range allComponents {
			if component.GetType() == componentType {
				return true
			}
		}
		return false
	}

	for _, component := range updatingComponents {
		if component.ComponentType == componentType {
			return true
		}
	}

	return false
}
