package controllers

import (
	"fmt"
	"strings"
)

type StepSyncStatus string
type StepName string

const (
	// StepSyncStatusDone means that step is done.
	StepSyncStatusDone StepSyncStatus = "Done"
	// StepSyncStatusSkip means that step must be skipped.
	StepSyncStatusSkip StepSyncStatus = "Skip"
	// StepSyncStatusUpdating means that step execution is in progress,
	// but currently nothing can be done by operator but wait.
	StepSyncStatusUpdating StepSyncStatus = "Updating"
	// StepSyncStatusBlocked means that step can't be executed for some reason,
	// and it is possible that human needed to proceed.
	StepSyncStatusBlocked StepSyncStatus = "Blocked"
	// StepSyncStatusNeedRun means that step should be executed.
	StepSyncStatusNeedRun StepSyncStatus = "NeedRun"
)

type StepStatus struct {
	SyncStatus StepSyncStatus
	Message    string
}

type baseStep struct {
	name StepName
}

func (s *baseStep) GetName() StepName {
	return s.name
}

type executionStats struct {
	order    []StepName
	statuses map[StepName]StepStatus
}

func newExecutionStats(steps []Step) executionStats {
	var order []StepName
	for _, step := range steps {
		order = append(order, step.GetName())
	}
	return executionStats{
		order:    order,
		statuses: make(map[StepName]StepStatus),
	}
}

func (s *executionStats) Collect(name StepName, status StepStatus) {
	s.statuses[name] = status
}

func (s *executionStats) isSkipped(name StepName) bool {
	status, wasExecuted := s.statuses[name]
	if !wasExecuted {
		return false
	}
	return status.SyncStatus == StepSyncStatusSkip
}

func (s *executionStats) AsLines() []string {
	var lines []string
	total := len(s.order)
	for idx, stepName := range s.order {
		status, ok := s.statuses[stepName]
		icon := statusToIcon(status.SyncStatus)
		if !ok {
			icon = "[ ]"
		}
		line := fmt.Sprintf("%s (%02d/%02d) %s", icon, idx+1, total, stepName)
		if status.Message != "" {
			line += ": " + status.Message
		}

		lines = append(lines, line)
	}
	return lines
}

func (s *executionStats) AsText() string {
	return strings.Join(s.AsLines(), "\n")
}
