package ytflow

import (
	"context"
	"fmt"
)

func not(condDep Condition) Condition {
	return Condition{
		Name: condDep.Name,
		Val:  !condDep.Val,
	}
}
func isTrue(cond ConditionName) Condition {
	return Condition{Name: cond, Val: true}
}

func updateDependenciesBasedConditions(ctx context.Context, condDeps map[ConditionName][]Condition, state stateManager) error {
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		somethingChanged := false

		for condName, deps := range condDeps {

			newValue := true
			for _, dep := range deps {
				if !IsSatisfied(dep, state) {
					newValue = false
				}
			}

			currentValue := state.Get(condName)
			if currentValue != newValue {
				somethingChanged = true
			}
			if err := state.Set(ctx, condName, newValue, "satisfied by deps"); err != nil {
				return fmt.Errorf("failed to set value %t for %s", newValue, condName)
			}

		}

		if !somethingChanged {
			return nil
		}
	}
	return fmt.Errorf("couldn't resolve dependencies in %d iterations", maxIterations)
}

func IsSatisfied(condDep Condition, conds stateManager) bool {
	realValue := conds.Get(condDep.Name)
	return realValue == condDep.Val
}
