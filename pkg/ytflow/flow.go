package ytflow

import (
	"context"
	"fmt"

	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
)

type FlowStatus string

const (
	FlowStatusDone     FlowStatus = "Done"
	FlowStatusUpdating FlowStatus = "Updating"
	FlowStatusBlocked  FlowStatus = "Blocked"
)

func Advance(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, clusterDomain string) (FlowStatus, error) {
	comps := buildComponents(ytsaurus, clusterDomain)

	// fetch all the components and collect all the statuses
	statuses, err := observe(ctx, comps)
	if err != nil {
		return "", fmt.Errorf("failed to observe statuses: %w", err)
	}

	// set conditions based on statuses
	conds := newConditionManager(ytsaurus.APIProxy().Client(), ytsaurus.GetResource())
	if err = updateComponentsConditions(ctx, statuses, conds); err != nil {
		return "", fmt.Errorf("failed to update components conditions: %w", err)
	}

	// TODO:
	// set conditions based on other conditions (while true? with limit on cycles)

	steps := buildSteps(comps, conds)
	runnableSteps := collectRunnables(steps, conds)
	// TODO: somehow differ all done with all blocked.
	// Need extra signal here with return after the conditions check.
	if len(runnableSteps) == 0 {
		return FlowStatusDone, nil
	}
	return FlowStatusUpdating, runSteps(ctx, runnableSteps)
}

func collectRunnables(steps *stepRegistry, conds conditionManagerType) map[stepName]stepType {
	var runnable map[stepName]stepType
	for name, step := range steps.steps {
		stepDeps := dependencies[name]

		// If step has no dependencies.
		if len(stepDeps) == 0 {
			runnable[name] = step
			continue
		}

		// Or all dependencies are satisfied.
		for _, dep := range stepDeps {
			if conds.IsFalse(dep) {
				continue
			}
		}
		runnable[name] = step
	}
	return runnable
}

func runSteps(ctx context.Context, steps map[stepName]stepType) error {
	for _, step := range steps {
		if err := step.Run(ctx); err != nil {
			return err
		}
	}
	return nil
}
