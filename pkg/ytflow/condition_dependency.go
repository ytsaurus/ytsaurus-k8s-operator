package ytflow

import (
	"context"
	"fmt"
)

type conditionDependency struct {
	name Condition
	val  bool
}

func not(condDep conditionDependency) conditionDependency {
	return conditionDependency{
		name: condDep.name,
		val:  !condDep.val,
	}
}
func isTrue(cond Condition) conditionDependency {
	return conditionDependency{name: cond, val: true}
}

//func isFalse(cond Condition) conditionDependency {
//	return conditionDependency{name: cond, val: false}
//}

// Components' statuses.
func isBuilt(compName ComponentName) conditionDependency {
	return isTrue(Condition(fmt.Sprintf("%sBuilt", compName)))
}
func isReady(compName ComponentName) conditionDependency {
	return isTrue(isReadyCondName(compName))
}

func isBuiltCondName(compName ComponentName) Condition {
	return Condition(fmt.Sprintf("%sBuilt", compName))
}
func isReadyCondName(compName ComponentName) Condition {
	return Condition(fmt.Sprintf("%sReady", compName))
}

// Actions statuses.
//func isBlocked(name StepName) conditionDependency {
//	return isTrue(condition(fmt.Sprintf("%sBlocked", name)))
//}
//
//func isRun(name StepName) conditionDependency {
//	return isTrue(condition(fmt.Sprintf("%sRun", name)))
//}
//
//func isUpdating(name StepName) conditionDependency {
//	return isTrue(condition(fmt.Sprintf("%sUpdating", name)))
//}
//
//func isDone(name StepName) conditionDependency {
//	return cond(fmt.Sprintf("%sDone", name))
//}

func updateDependenciesBasedConditions(ctx context.Context, condDeps map[Condition][]conditionDependency, conds stateManager) error {
	maxIterations := 10
	for i := 0; i < maxIterations; i++ {
		somethingChanged := false

		for condName, deps := range condDeps {

			newValue := true
			for _, dep := range deps {
				if !IsSatisfied(dep, conds) {
					newValue = false
				}
			}

			currentValue := conds.Get(condName)
			if currentValue != newValue {
				somethingChanged = true
			}
			if err := conds.Set(ctx, condName, newValue, "satisfied by deps"); err != nil {
				return fmt.Errorf("failed to set value %t for %s", newValue, condName)
			}

		}

		if !somethingChanged {
			return nil
		}
	}
	return fmt.Errorf("couldn't resolve dependencies in %d iterations", maxIterations)
}

func IsSatisfied(condDep conditionDependency, conds stateManager) bool {
	realValue := conds.Get(condDep.name)
	return realValue == condDep.val
}