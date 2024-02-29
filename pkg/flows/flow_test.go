package flows

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testStatusErrorMsg = "test status error"
	testRunErrorMsg    = "test run error"
)

func success(context.Context) error {
	return nil
}

func fail(context.Context) error {
	return errors.New(testRunErrorMsg)
}

var chooseCases = []struct {
	name                 string
	steps                []StepType
	storageBefore        map[string]struct{}
	expectedStepName     StepName
	expectedStepStatus   StepSyncStatus
	expectedErrorMessage string
}{
	{
		name: "simple-first-step-need-run",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusNeedRun),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepName:   "step1",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "simple-first-step-updating",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusUpdating),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepName:   "step1",
		expectedStepStatus: StepSyncStatusUpdating,
	},
	{
		name: "simple-first-step-blocked",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusBlocked),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepName:   "step1",
		expectedStepStatus: StepSyncStatusBlocked,
	},
	{
		name: "simple-first-step-done-second-run",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusDone),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepName:   "step2",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "simple-first-step-done-second-updating",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusDone),
			newCachelessStep("step2", StepSyncStatusUpdating),
		},
		expectedStepName:   "step2",
		expectedStepStatus: StepSyncStatusUpdating,
	},
	{
		name: "simple-first-step-done-second-done",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusDone),
			newCachelessStep("step2", StepSyncStatusDone),
		},
	},
	{
		name: "simple-first-step-error",
		steps: []StepType{
			newStatusErrorStep("step1", StepSyncStatusNeedRun),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedErrorMessage: "failed to get status for step step1",
	},
	{
		name: "simple-action-success",
		steps: []StepType{
			NewActionStep("step1", success),
			NewActionStep("step2", success),
		},
		expectedStepName:   "step1",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "simple-action-step1-done-step2-run",
		steps: []StepType{
			NewActionStep("step1", success),
			NewActionStep("step2", success),
		},
		storageBefore: map[string]struct{}{
			"step1Done": {},
		},
		expectedStepName:   "step2",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "simple-action-all-done",
		steps: []StepType{
			NewActionStep("step1", success),
			NewActionStep("step2", success),
		},
		storageBefore: map[string]struct{}{
			"step1Done": {},
			"step2Done": {},
		},
	},
	{
		name: "simple-mixed-steps-first-status-run",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusNeedRun),
			NewActionStep("step2", success),
		},
		expectedStepName: "step1",
	},
	{
		name: "simple-mixed-steps-first-status-done",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusDone),
			NewActionStep("step2", success),
		},
		expectedStepName: "step2",
	},
	{
		name: "simple-mixed-steps-first-cached-run",
		steps: []StepType{
			NewActionStep("step1", success),
			newCachelessStep("step2", StepSyncStatusDone),
		},
		expectedStepName: "step1",
	},
	{
		name: "simple-mixed-steps-first-cached-done",
		steps: []StepType{
			NewActionStep("step1", success),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		storageBefore: map[string]struct{}{
			"step1Done": {},
		},
		expectedStepName: "step2",
	},
	{
		name: "condition-simple-steps-branch-1",
		steps: []StepType{
			NewConditionStep(
				"cond",
				func(ctx context.Context) (string, error) {
					return "True", nil
				},
				map[string][]StepType{
					"True": {
						newCachelessStep("step11", StepSyncStatusDone),
						newCachelessStep("step12", StepSyncStatusNeedRun),
					},
					"False": {
						newCachelessStep("step21", StepSyncStatusDone),
						newCachelessStep("step22", StepSyncStatusNeedRun),
					},
				},
			),
		},
		expectedStepName:   "step12",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "condition-simple-steps-branch-2",
		steps: []StepType{
			NewConditionStep(
				"cond",
				func(ctx context.Context) (string, error) {
					return "False", nil
				},
				map[string][]StepType{
					"True": {
						newCachelessStep("step11", StepSyncStatusDone),
						newCachelessStep("step12", StepSyncStatusNeedRun),
					},
					"False": {
						newCachelessStep("step21", StepSyncStatusDone),
						newCachelessStep("step22", StepSyncStatusNeedRun),
					},
				},
			),
		},
		expectedStepName:   "step22",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "condition-simple-steps-branch-1-done",
		steps: []StepType{
			NewConditionStep(
				"cond",
				func(ctx context.Context) (string, error) {
					return "True", nil
				},
				map[string][]StepType{
					"True": {
						newCachelessStep("step11", StepSyncStatusDone),
						newCachelessStep("step12", StepSyncStatusDone),
					},
					"False": {
						newCachelessStep("step21", StepSyncStatusNeedRun),
						newCachelessStep("step22", StepSyncStatusNeedRun),
					},
				},
			),
		},
	},
	{
		name: "condition-result-cached",
		steps: []StepType{
			NewConditionStep(
				"cond",
				func(ctx context.Context) (string, error) {
					// This should be ignored as False was cached from before.
					return "True", nil
				},
				map[string][]StepType{
					"True": {
						newCachelessStep("step11", StepSyncStatusDone),
						newCachelessStep("step12", StepSyncStatusNeedRun),
					},
					"False": {
						newCachelessStep("step21", StepSyncStatusDone),
						newCachelessStep("step22", StepSyncStatusNeedRun),
					},
				},
			),
		},
		storageBefore: map[string]struct{}{
			"condFalse": {},
		},
		expectedStepName:   "step22",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "condition-action-steps-branch-1",
		steps: []StepType{
			NewConditionStep(
				"cond",
				func(ctx context.Context) (string, error) {
					return "True", nil
				},
				map[string][]StepType{
					"True": {
						NewActionStep("step11", success),
						NewActionStep("step12", success),
					},
					"False": {
						newCachelessStep("step21", StepSyncStatusDone),
					},
				},
			),
		},
		expectedStepName:   "step11",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "condition-action-steps-branch-1-first-done",
		steps: []StepType{
			NewConditionStep(
				"cond",
				func(ctx context.Context) (string, error) {
					return "True", nil
				},
				map[string][]StepType{
					"True": {
						NewActionStep("step11", success),
						NewActionStep("step12", success),
					},
					"False": {
						newCachelessStep("step21", StepSyncStatusDone),
					},
				},
			),
		},
		storageBefore: map[string]struct{}{
			"step11Done": {},
		},
		expectedStepName:   "step12",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
}

func TestStepChoosing(t *testing.T) {
	for _, testCase := range chooseCases {
		t.Run(testCase.name, func(t *testing.T) {
			storage := newTestStorage(testCase.storageBefore)
			flow := NewFlow(testCase.steps, storage)

			step, status, err := flow.getNextStep(context.Background())
			if testCase.expectedErrorMessage != "" {
				require.ErrorContains(t, err, testCase.expectedErrorMessage)
			} else {
				require.NoError(t, err)
			}

			if testCase.expectedStepName != "" {
				require.NotNil(t, step)
				require.Equal(t, testCase.expectedStepName, step.StepName())
			} else {
				require.Nil(t, step)
			}

			if testCase.expectedStepStatus != "" {
				require.Equal(t, testCase.expectedStepStatus, status.SyncStatus)
			}
		})
	}
}

var advanceCases = []struct {
	name                 string
	steps                []StepType
	storageBefore        map[string]struct{}
	expectedStepStatus   StepSyncStatus
	expectedErrorMessage string
	expectedStorageAfter map[string]struct{}
}{
	{
		name: "simple-first-step-need-run",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusNeedRun),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus: StepSyncStatusUpdating,
	},
	{
		name: "simple-first-step-updating",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusUpdating),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus: StepSyncStatusUpdating,
	},
	{
		name: "simple-first-step-blocked",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusBlocked),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus: StepSyncStatusBlocked,
	},
	{
		name: "simple-first-empty-state-1",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusBlocked),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus:   StepSyncStatusBlocked,
		expectedStorageAfter: map[string]struct{}{},
	},
	{
		name: "simple-first-empty-state-2",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusNeedRun),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus:   StepSyncStatusUpdating,
		expectedStorageAfter: map[string]struct{}{},
	},
	{
		name: "simple-first-empty-state-3",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusDone),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus:   StepSyncStatusUpdating,
		expectedStorageAfter: map[string]struct{}{},
	},
	{
		name: "simple-first-all-done",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusDone),
			newCachelessStep("step2", StepSyncStatusDone),
		},
		expectedStepStatus:   StepSyncStatusDone,
		expectedStorageAfter: map[string]struct{}{},
	},
	{
		name: "actions-first-run-success",
		steps: []StepType{
			NewActionStep("step1", success),
			NewActionStep("step2", success),
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]struct{}{
			"step1Done": {},
		},
	},
	{
		name: "actions-second-run-success",
		steps: []StepType{
			NewActionStep("step1", success),
			NewActionStep("step2", success),
		},
		storageBefore: map[string]struct{}{
			"step1Done": {},
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]struct{}{
			"step1Done": {},
			"step2Done": {},
		},
	},
	{
		name: "actions-first-run-error",
		steps: []StepType{
			NewActionStep("step1", fail),
			NewActionStep("step2", success),
		},
		expectedErrorMessage: "step1 execution failed",
		expectedStorageAfter: map[string]struct{}{},
	},
	{
		name: "actions-first-run-cached-error",
		steps: []StepType{
			// fail is ignored, because we have stored state
			NewActionStep("step1", fail),
			NewActionStep("step2", success),
		},
		storageBefore: map[string]struct{}{
			"step1Done": {},
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]struct{}{
			"step1Done": {},
			"step2Done": {},
		},
	},
	{
		name: "actions-first-state-reset",
		steps: []StepType{
			NewActionStep("step1", success),
			NewActionStep("step2", success),
		},
		storageBefore: map[string]struct{}{
			"step1Done": {},
			"step2Done": {},
		},
		expectedStepStatus:   StepSyncStatusDone,
		expectedStorageAfter: map[string]struct{}{},
	},
	{
		name: "mixed-simple-action-first-run",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusNeedRun),
			NewActionStep("step2", success),
		},
		expectedStepStatus:   StepSyncStatusUpdating,
		expectedStorageAfter: map[string]struct{}{},
	},
	{
		name: "mixed-simple-action-second-run",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusDone),
			NewActionStep("step2", success),
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]struct{}{
			"step2Done": {},
		},
	},
	{
		name: "mixed-simple-action-all-done",
		steps: []StepType{
			newCachelessStep("step1", StepSyncStatusDone),
			NewActionStep("step2", success),
		},
		storageBefore: map[string]struct{}{
			"step2Done": {},
		},
		expectedStepStatus:   StepSyncStatusDone,
		expectedStorageAfter: map[string]struct{}{},
	},
	{
		name: "mixed-action-simple-first-run",
		steps: []StepType{
			NewActionStep("step1", success),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]struct{}{
			"step1Done": {},
		},
	},
	{
		name: "mixed-action-simple-second-run",
		steps: []StepType{
			NewActionStep("step1", success),
			newCachelessStep("step2", StepSyncStatusNeedRun),
		},
		storageBefore: map[string]struct{}{
			"step1Done": {},
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]struct{}{
			"step1Done": {},
		},
	},
	{
		name: "mixed-action-simple-all-done",
		steps: []StepType{
			NewActionStep("step1", success),
			newCachelessStep("step2", StepSyncStatusDone),
		},
		storageBefore: map[string]struct{}{
			"step1Done": {},
		},
		expectedStepStatus:   StepSyncStatusDone,
		expectedStorageAfter: map[string]struct{}{},
	},
}

func TestAdvance(t *testing.T) {
	for _, testCase := range advanceCases {
		t.Run(testCase.name, func(t *testing.T) {
			storage := newTestStorage(testCase.storageBefore)
			flow := NewFlow(testCase.steps, storage)

			status, err := flow.Advance(context.Background())
			if testCase.expectedErrorMessage != "" {
				require.ErrorContains(t, err, testCase.expectedErrorMessage)
			} else {
				require.NoError(t, err)
			}

			if testCase.expectedStepStatus != "" {
				require.Equal(t, testCase.expectedStepStatus, status)
			}

			if testCase.expectedStorageAfter != nil {
				require.Equal(t, testCase.expectedStorageAfter, storage.store)
			}
		})
	}
}
