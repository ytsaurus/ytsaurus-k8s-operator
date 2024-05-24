package components

import (
	"fmt"
	"strings"
)

type ConditionName string

type Condition struct {
	Name ConditionName
	Val  bool
}

func (c Condition) String() string {
	if c.Val {
		return string(c.Name)
	}
	return fmt.Sprintf("!%s", c.Name)
}

func not(condDep Condition) Condition {
	return Condition{
		Name: condDep.Name,
		Val:  !condDep.Val,
	}
}
func isTrue(cond ConditionName) Condition {
	// '^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$' for conditions.
	replaced := strings.ReplaceAll(string(cond), "-", "_")
	return Condition{Name: ConditionName(replaced), Val: true}
}

// buildFinished means that component was fully built initially.
func buildStarted(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sBuildStarted", compName)))
}
func buildFinished(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sBuildFinished", compName)))
}

func initializationStarted(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%snitializationStarted", compName)))
}
func initializationFinished(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%snitializationFinished", compName)))
}
func updateRequired(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sUpdateRequired", compName)))
}
func rebuildStarted(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sRebuildStarted", compName)))
}
func podsRemoved(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sPodsRemoved", compName)))
}
func podsCreated(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sPodsCreated", compName)))
}
func rebuildFinished(compName string) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sRebuildFinished", compName)))
}
