package ytflow

import (
	"fmt"
	"sort"
	"strings"
)

func diffConditions(beforeConds []Condition, afterConds []Condition) string {
	beforeMap := make(map[ConditionName]bool)
	afterMap := make(map[ConditionName]bool)
	allNamesMap := make(map[ConditionName]struct{})
	var allNames []string

	for _, cond := range beforeConds {
		name := cond.Name
		beforeMap[name] = cond.Val
		allNamesMap[name] = struct{}{}
	}
	for _, cond := range afterConds {
		name := cond.Name
		afterMap[name] = cond.Val
		allNamesMap[name] = struct{}{}
	}
	for name := range allNamesMap {
		allNames = append(allNames, string(name))
	}
	sort.Strings(allNames)

	var lines []string
	for _, name := range allNames {
		b := beforeMap[ConditionName(name)]
		a := afterMap[ConditionName(name)]
		if a == b {
			continue
		}
		prefix := "[ ]"
		if a {
			prefix = "[v]"
		}
		line := fmt.Sprintf("%s %s", prefix, name)
		lines = append(lines, line)
	}
	sort.Slice(lines, func(i, j int) bool {
		return lines[i][len("[ ]"):] < lines[j][len("[ ]"):]
	})

	return strings.Join(lines, "\n")
}

func reportSteps(steps *stepRegistry, runnable map[StepName]stepType, state stateManager) string {
	nonRunnable := make(map[StepName]stepType)
	for name, step := range steps.steps {
		if _, ok := runnable[name]; !ok {
			nonRunnable[name] = step
		}
	}

	stringifyConds := func(conds []Condition) string {
		var deps []string
		for _, cond := range conds {
			depStr := ""
			if IsSatisfied(cond, state) {
				depStr += "[v]"
			} else {
				depStr += "[ ]"
			}
			depStr += cond.String()
			deps = append(deps, depStr)
		}
		return strings.Join(deps, ", ")
	}

	var lines []string
	lines = append(lines, "Runnable:")
	for name := range runnable {
		line := fmt.Sprintf("  %s: %s", name, stringifyConds(stepDependencies[name]))
		lines = append(lines, line)
	}
	lines = append(lines, "Non-runnable:")
	for name := range nonRunnable {
		line := fmt.Sprintf("  %s: %s", name, stringifyConds(stepDependencies[name]))
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
