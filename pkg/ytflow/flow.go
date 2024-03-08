package ytflow

import (
	"context"
	"fmt"
	"sort"

	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
)

type FlowStatus string

const (
	FlowStatusDone     FlowStatus = "Done"
	FlowStatusUpdating FlowStatus = "Updating"
	FlowStatusBlocked  FlowStatus = "Blocked"
)

type conditionManagerType interface {
	SetTrue(context.Context, conditionName, string) error
	SetFalse(context.Context, conditionName, string) error
	Set(context.Context, conditionName, bool, string) error
	IsTrue(conditionName) bool
	IsFalse(conditionName) bool
	Get(conditionName) bool
	IsSatisfied(conditionDependency) bool
}

func Advance(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, clusterDomain string, conds conditionManagerType) (FlowStatus, error) {
	comps := buildComponents(ytsaurus, clusterDomain)
	return doAdvance(ctx, comps, conds)
}

// maybe ok to have it public and move comps building in components.
// check about components names
func doAdvance(ctx context.Context, comps *componentRegistry, conds conditionManagerType) (FlowStatus, error) {
	// fetch all the components and collect all the statuses
	statuses, err := observe(ctx, comps)
	if err != nil {
		return "", fmt.Errorf("failed to observe statuses: %w", err)
	}

	// set conditions based on statuses
	if err = updateComponentsConditions(ctx, statuses, conds); err != nil {
		return "", fmt.Errorf("failed to update components conditions: %w", err)
	}

	// set conditions based on other conditions (while true? with limit on cycles)
	if err = updateConditionsByDependencies(ctx, conditionDependencies, conds); err != nil {
		return "", err
	}

	if conds.IsSatisfied(NothingToDo) {
		return FlowStatusDone, nil
	}

	steps := buildSteps(comps, conds)
	runnableSteps := collectRunnables(steps, conds)
	// TODO: somehow differ all done with all blocked.
	// Need extra signal here with return after the conditions check.
	if len(runnableSteps) == 0 {
		return FlowStatusDone, nil
	}
	return FlowStatusUpdating, runSteps(ctx, runnableSteps)
}

func collectRunnables(steps *stepRegistry, conds conditionManagerType) map[StepName]stepType {
	runnable := make(map[StepName]stepType)
	for name, step := range steps.steps {
		stepDeps := stepDependencies[name]

		// If step has no dependencies no need to run.
		if len(stepDeps) == 0 {
			runnable[name] = step
			continue
		}

		// If any of the dependencies are not satisfied, no need to run.
		hasUnsatisfied := false
		for _, condDep := range stepDeps {
			if !conds.IsSatisfied(condDep) {
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
	// Just for the test stability we execute steps in predictable order.
	var keys []string
	for key := range steps {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)

	for _, key := range keys {
		step := steps[StepName(key)]
		if err := step.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}
