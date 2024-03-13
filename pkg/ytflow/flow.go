package ytflow

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
)

type FlowStatus string

const (
	FlowStatusDone     FlowStatus = "Done"
	FlowStatusUpdating FlowStatus = "Updating"
	FlowStatusBlocked  FlowStatus = "Blocked"
)

type stateManager interface {
	SetTrue(context.Context, Condition, string) error
	SetFalse(context.Context, Condition, string) error
	Set(context.Context, Condition, bool, string) error
	IsTrue(Condition) bool
	IsFalse(Condition) bool
	Get(Condition) bool

	// Don't really like mix of conditions and temporary data storage.

	SetClusterState(context.Context, ytv1.ClusterState) error
	SetTabletCellBundles(context.Context, []ytv1.TabletCellBundleInfo) error
	SetMasterMonitoringPaths(context.Context, []string) error
	GetClusterState() ytv1.ClusterState
	GetTabletCellBundles() []ytv1.TabletCellBundleInfo
	GetMasterMonitoringPaths() []string
}

func Advance(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, clusterDomain string, state stateManager) (FlowStatus, error) {
	comps := buildComponents(ytsaurus, clusterDomain)
	actionSteps := buildActionSteps(comps, state)
	return doAdvance(ctx, comps, actionSteps, state)
}

// maybe ok to have it public and move comps building in components.
// check about components names
func doAdvance(ctx context.Context, comps *componentRegistry, actions map[StepName]stepType, state stateManager) (FlowStatus, error) {
	log := log.FromContext(ctx)

	// fetch all the components and collect all the statuses
	statuses, err := observe(ctx, comps)
	if err != nil {
		return "", fmt.Errorf("failed to observe statuses: %w", err)
	}

	if err = updateClusterBasedConditions(ctx, state); err != nil {
		return "", fmt.Errorf("failed to update cluster based conditions: %w", err)
	}
	if err = updateComponentsBasedConditions(ctx, statuses, state); err != nil {
		return "", fmt.Errorf("failed to update components conditions: %w", err)
	}
	if err = updateDependenciesBasedConditions(ctx, conditionDependencies, state); err != nil {
		return "", err
	}

	if IsSatisfied(NothingToDo, state) {
		return FlowStatusDone, nil
	}

	steps := buildSteps(comps, actions)
	runnableSteps := collectRunnables(log, steps, state)
	// TODO: somehow differ all done with all blocked.
	// Need extra signal here with return after the conditions check.
	if len(runnableSteps) == 0 {
		return FlowStatusDone, nil
	}
	return FlowStatusUpdating, runSteps(ctx, runnableSteps)
}

func collectRunnables(log logr.Logger, steps *stepRegistry, conds stateManager) map[StepName]stepType {
	runnable := make(map[StepName]stepType)
	for name, step := range steps.steps {
		//log.Info(fmt.Sprintf("[ ] %s", name))
		// TODO: collect execution, configure logger in test
		fmt.Printf("STEP %s\n", name)
		stepDeps := stepDependencies[name]

		// If step has no dependencies no need to run.
		if len(stepDeps) == 0 {
			runnable[name] = step
			continue
		}

		// If any of the dependencies are not satisfied, no need to run.
		hasUnsatisfied := false
		for _, condDep := range stepDeps {
			isSatisfied := IsSatisfied(condDep, conds)

			symbol := " "
			if isSatisfied {
				symbol = "x"
			}
			notSymbol := ""
			if !condDep.val {
				notSymbol = "!"
			}
			fmt.Printf("     [%s] %s%s\n", symbol, notSymbol, condDep.name)

			if !isSatisfied {
				hasUnsatisfied = true
				break
			}
		}
		if hasUnsatisfied {
			continue
		}
		runnable[name] = step
	}
	return runnable
}

func runSteps(ctx context.Context, steps map[StepName]stepType) error {
	logger := log.FromContext(ctx)

	// Just for the test stability we execute steps in predictable order.
	var keys []string
	for key := range steps {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)

	logger.V(0).Info(fmt.Sprintf("going to run steps: %s", keys))
	for _, key := range keys {
		step := steps[StepName(key)]
		if err := step.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}
