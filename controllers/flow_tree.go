package controllers

import (
	"context"
	"fmt"

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
	if err = ytsaurus.SaveUpdateState(ctx, nextStep.updateState); err != nil {
		return false, err
	}
	return false, nil
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

func (f *flowTree) chainIfWithDeferredChain(cond bool, deferredChain ytv1.UpdateState, steps ...*flowStep) *flowTree {
	f.deferredChain = deferredChain

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
	ytv1.UpdateStateWaitingForSnapshots:                flowCheckStatusCondition(consts.ConditionSnaphotsSaved),
	ytv1.UpdateStateWaitingForTabletCellsRecovery:      flowCheckStatusCondition(consts.ConditionTabletCellsRecovered),
	ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if !componentManager.needSchedulerUpdate() || ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionNotNecessaryToUpdateOpArchive) {
			return stepResultMarkHappy
		}
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionOpArchivePreparedForUpdating) {
			return stepResultMarkUnhappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForOpArchiveUpdate: flowCheckStatusCondition(consts.ConditionOpArchiveUpdated),
	ytv1.UpdateStateWaitingForQTStateUpdatingPrepare: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if !componentManager.needQueryTrackerUpdate() {
			return stepResultMarkHappy
		}
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionQTStatePreparedForUpdating) {
			return stepResultMarkUnhappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForQTStateUpdate: flowCheckStatusCondition(consts.ConditionQTStateUpdated),
	ytv1.UpdateStateWaitingForYqlaUpdatingPrepare: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if !componentManager.needYqlAgentUpdate() {
			return stepResultMarkHappy
		}
		if ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionYqlaPreparedForUpdating) {
			return stepResultMarkUnhappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForYqlaUpdate:       flowCheckStatusCondition(consts.ConditionYqlaUpdated),
	ytv1.UpdateStateWaitingForSafeModeDisabled: flowCheckStatusCondition(consts.ConditionSafeModeDisabled),

	ytv1.UpdateStateWaitingForPodsRemoval: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if componentManager.arePodsRemoved() {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
	ytv1.UpdateStateWaitingForPodsCreation: func(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, componentManager *ComponentManager) stepResultMark {
		if componentManager.allReadyOrUpdating() {
			return stepResultMarkHappy
		}
		return stepResultMarkUnsatisfied
	},
}

func buildFlowTree(components []ytv1.Component) *flowTree {
	st := newSimpleStep
	head := st(ytv1.UpdateStateNone)
	tree := newFlowTree(head)

	updMasterOrTablet := hasMaster(components) || hasTablets(components)
	updMaster := hasMaster(components)
	updTablet := hasMaster(components)
	updStateless := hasStateless(components)

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
		st(ytv1.UpdateStateWaitingForSnapshots),
	).chain(
		st(ytv1.UpdateStateWaitingForPodsRemoval),
		st(ytv1.UpdateStateWaitingForPodsCreation),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForMasterExitReadOnly),
	).chainIf(
		updTablet,
		st(ytv1.UpdateStateWaitingForTabletCellsRecovery),
	).chainIfWithDeferredChain(
		// TODO: we need to check for the exact component type here
		updStateless,
		// This is ugly, I suppose component code can be changed to avoid such transitions.
		ytv1.UpdateStateWaitingForOpArchiveUpdate,
		newConditionalForkStep(
			ytv1.UpdateStateWaitingForOpArchiveUpdatingPrepare,
			// This is the unhappy path.
			// it will be chained with the next chained step via deferredChain argument
			st(ytv1.UpdateStateWaitingForOpArchiveUpdate),
			// Happy path will be chained automatically.
		),
	).chainIfWithDeferredChain(
		// TODO: we need to check for the exact component type here
		updStateless,
		// This is ugly, I suppose component code can be changed to avoid such transitions.
		ytv1.UpdateStateWaitingForQTStateUpdate,
		newConditionalForkStep(
			ytv1.UpdateStateWaitingForQTStateUpdatingPrepare,
			// This is the unhappy path.
			st(ytv1.UpdateStateWaitingForQTStateUpdate),
			// Happy path will be chained automatically.
		),
	).chainIfWithDeferredChain(
		// TODO: we need to check for the exact component type here
		updStateless,
		// This is ugly, I suppose component code can be changed to avoid such transitions.
		ytv1.UpdateStateWaitingForYqlaUpdate,
		newConditionalForkStep(
			ytv1.UpdateStateWaitingForYqlaUpdatingPrepare,
			// This is the unhappy path.
			st(ytv1.UpdateStateWaitingForYqlaUpdate),
			// Happy path will be chained automatically.
		),
	).chainIf(
		updMaster,
		st(ytv1.UpdateStateWaitingForSafeModeDisabled),
	)
	// what if not registered — we need to add with chain

	return tree
}
