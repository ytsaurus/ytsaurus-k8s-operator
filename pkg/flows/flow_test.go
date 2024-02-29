package flows

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr/testr"
	"github.com/stretchr/testify/require"
)

const (
	testStatusErrorMsg = "test status error"
	testRunErrorMsg    = "test run error"
)

func fail(context.Context) error {
	return errors.New(testRunErrorMsg)
}

var chooseCases = []struct {
	name                 string
	steps                []StepType
	storageBefore        map[string]bool
	expectedStepName     StepName
	expectedStepStatus   StepSyncStatus
	expectedErrorMessage string
}{
	{
		name: "simple-first-step-need-run",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusNeedRun),
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepName:   "step1",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "simple-first-step-updating",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusUpdating),
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepName:   "step1",
		expectedStepStatus: StepSyncStatusUpdating,
	},
	{
		name: "simple-first-step-blocked",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusBlocked),
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepName:   "step1",
		expectedStepStatus: StepSyncStatusBlocked,
	},
	{
		name: "simple-first-step-done-second-run",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusDone),
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepName:   "step2",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "simple-first-step-done-second-updating",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusDone),
			newSelfManagedStep("step2", StepSyncStatusUpdating),
		},
		expectedStepName:   "step2",
		expectedStepStatus: StepSyncStatusUpdating,
	},
	{
		name: "simple-first-step-done-second-done",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusDone),
			newSelfManagedStep("step2", StepSyncStatusDone),
		},
	},
	{
		name: "simple-first-step-error",
		steps: []StepType{
			newSelfManagedErrorStep("step1", StepSyncStatusNeedRun),
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedErrorMessage: "failed to collect status for step step1",
	},
	{
		name: "simple-action-success",
		steps: []StepType{
			ActionStep{Name: "step1"},
			ActionStep{Name: "step2"},
		},
		expectedStepName:   "step1",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "simple-action-step1-done-step2-run",
		steps: []StepType{
			ActionStep{Name: "step1"},
			ActionStep{Name: "step2"},
		},
		storageBefore: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
		},
		expectedStepName:   "step2",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "simple-action-all-done",
		steps: []StepType{
			ActionStep{Name: "step1"},
			ActionStep{Name: "step2"},
		},
		storageBefore: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
			"step2Run":  true,
			"step2Done": true,
		},
	},
	{
		name: "simple-mixed-steps-first-status-run",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusNeedRun),
			ActionStep{Name: "step2"},
		},
		expectedStepName: "step1",
	},
	{
		name: "simple-mixed-steps-first-status-done",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusDone),
			ActionStep{Name: "step2"},
		},
		expectedStepName: "step2",
	},
	{
		name: "simple-mixed-steps-first-cached-run",
		steps: []StepType{
			ActionStep{Name: "step1"},
			newSelfManagedStep("step2", StepSyncStatusDone),
		},
		expectedStepName: "step1",
	},
	{
		name: "simple-mixed-steps-first-cached-done",
		steps: []StepType{
			ActionStep{Name: "step1"},
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		storageBefore: map[string]bool{
			"step1Done": true,
			"step1Run":  true,
		},
		expectedStepName: "step2",
	},
	{
		name: "condition-simple-steps-branch-1",
		steps: []StepType{
			BoolConditionStep{
				Name: "cond",
				Cond: func(ctx context.Context) (bool, error) {
					return true, nil
				},
				True: []StepType{
					newSelfManagedStep("step11", StepSyncStatusDone),
					newSelfManagedStep("step12", StepSyncStatusNeedRun),
				},
				False: []StepType{
					newSelfManagedStep("step21", StepSyncStatusDone),
					newSelfManagedStep("step22", StepSyncStatusNeedRun),
				},
			},
		},
		expectedStepName:   "step12",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "condition-simple-steps-branch-2",
		steps: []StepType{
			BoolConditionStep{
				Name: "cond",
				Cond: func(ctx context.Context) (bool, error) {
					return false, nil
				},
				True: []StepType{
					newSelfManagedStep("step11", StepSyncStatusDone),
					newSelfManagedStep("step12", StepSyncStatusNeedRun),
				},
				False: []StepType{
					newSelfManagedStep("step21", StepSyncStatusDone),
					newSelfManagedStep("step22", StepSyncStatusNeedRun),
				},
			},
		},
		expectedStepName:   "step22",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "condition-simple-steps-branch-1-done",
		steps: []StepType{
			BoolConditionStep{
				Name: "cond",
				Cond: func(ctx context.Context) (bool, error) {
					return false, nil
				},
				True: []StepType{
					newSelfManagedStep("step11", StepSyncStatusDone),
					newSelfManagedStep("step12", StepSyncStatusDone),
				},
				False: []StepType{
					newSelfManagedStep("step21", StepSyncStatusNeedRun),
					newSelfManagedStep("step22", StepSyncStatusNeedRun),
				},
			},
		},
		expectedStepName:   "step21",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "condition-result-cached",
		steps: []StepType{
			BoolConditionStep{
				Name: "cond",
				Cond: func(ctx context.Context) (bool, error) {
					return true, nil
				},
				True: []StepType{
					newSelfManagedStep("step11", StepSyncStatusDone),
					newSelfManagedStep("step12", StepSyncStatusNeedRun),
				},
				False: []StepType{
					newSelfManagedStep("step21", StepSyncStatusDone),
					newSelfManagedStep("step22", StepSyncStatusNeedRun),
				},
			},
		},
		storageBefore: map[string]bool{
			"condCond": false,
		},
		expectedStepName:   "step22",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "condition-action-steps-branch-1",
		steps: []StepType{
			BoolConditionStep{
				Name: "cond",
				Cond: func(ctx context.Context) (bool, error) {
					return true, nil
				},
				True: []StepType{
					ActionStep{Name: "step11"},
					ActionStep{Name: "step12"},
				},
				False: []StepType{
					newSelfManagedStep("step21", StepSyncStatusDone),
				},
			},
		},
		expectedStepName:   "step11",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
	{
		name: "condition-action-steps-branch-1-first-done",
		steps: []StepType{
			BoolConditionStep{
				Name: "cond",
				Cond: func(ctx context.Context) (bool, error) {
					return true, nil
				},
				True: []StepType{
					ActionStep{Name: "step11"},
					ActionStep{Name: "step12"},
				},
				False: []StepType{
					newSelfManagedStep("step21", StepSyncStatusDone),
				},
			},
		},
		storageBefore: map[string]bool{
			"step11Run":  true,
			"step11Done": true,
		},
		expectedStepName:   "step12",
		expectedStepStatus: StepSyncStatusNeedRun,
	},
}

func TestStepChoosing(t *testing.T) {
	for _, testCase := range chooseCases {
		t.Run(testCase.name, func(t *testing.T) {
			storage := newTestStorage(testCase.storageBefore)
			flow := NewFlow(testCase.steps, storage, testr.New(t))

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
	storageBefore        map[string]bool
	expectedStepStatus   StepSyncStatus
	expectedErrorMessage string
	expectedStorageAfter map[string]bool
}{
	{
		name: "simple-first-step-need-run",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusNeedRun),
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus: StepSyncStatusUpdating,
	},
	{
		name: "simple-first-step-updating",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusUpdating),
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus: StepSyncStatusUpdating,
	},
	{
		name: "simple-first-step-blocked",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusBlocked),
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus: StepSyncStatusBlocked,
	},
	{
		name: "simple-first-empty-state-1",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusBlocked),
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus:   StepSyncStatusBlocked,
		expectedStorageAfter: map[string]bool{},
	},
	{
		name: "simple-first-empty-state-2",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusNeedRun),
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus:   StepSyncStatusUpdating,
		expectedStorageAfter: map[string]bool{},
	},
	{
		name: "simple-first-empty-state-3",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusDone),
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus:   StepSyncStatusUpdating,
		expectedStorageAfter: map[string]bool{},
	},
	{
		name: "simple-first-all-done",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusDone),
			newSelfManagedStep("step2", StepSyncStatusDone),
		},
		expectedStepStatus:   StepSyncStatusDone,
		expectedStorageAfter: map[string]bool{},
	},
	{
		name: "actions-first-run-success",
		steps: []StepType{
			ActionStep{Name: "step1"},
			ActionStep{Name: "step2"},
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]bool{
			"step1Run": true,
		},
	},
	{
		name: "actions-first-done-after-run-success",
		steps: []StepType{
			ActionStep{Name: "step1"},
			ActionStep{Name: "step2"},
		},
		expectedStepStatus: StepSyncStatusUpdating,
		storageBefore: map[string]bool{
			"step1Run": true,
		},
		expectedStorageAfter: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
			"step2Run":  true,
		},
	},
	{
		name: "actions-second-run-success",
		steps: []StepType{
			ActionStep{Name: "step1"},
			ActionStep{Name: "step2"},
		},
		storageBefore: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
			"step2Run":  true,
		},
	},
	{
		name: "actions-second-run-done-success",
		steps: []StepType{
			ActionStep{Name: "step1"},
			ActionStep{Name: "step2"},
		},
		storageBefore: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
			"step2Run":  true,
		},
		expectedStepStatus:   StepSyncStatusDone,
		expectedStorageAfter: map[string]bool{},
	},
	{
		name: "actions-first-run-error",
		steps: []StepType{
			ActionStep{Name: "step1", RunFunc: fail},
			ActionStep{Name: "step2"},
		},
		expectedErrorMessage: "step1 execution failed",
		expectedStorageAfter: map[string]bool{},
	},
	{
		name: "actions-first-run-cached-error",
		steps: []StepType{
			// fail is ignored, because we have stored state
			ActionStep{Name: "step1", RunFunc: fail},
			ActionStep{Name: "step2"},
		},
		storageBefore: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
			"step2Run":  true,
		},
	},
	{
		name: "actions-first-state-reset",
		steps: []StepType{
			ActionStep{Name: "step1"},
			ActionStep{Name: "step2"},
		},
		storageBefore: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
			"step2Run":  true,
			"step2Done": true,
		},
		expectedStepStatus:   StepSyncStatusDone,
		expectedStorageAfter: map[string]bool{},
	},
	{
		name: "mixed-simple-action-first-run",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusNeedRun),
			ActionStep{Name: "step2"},
		},
		expectedStepStatus:   StepSyncStatusUpdating,
		expectedStorageAfter: map[string]bool{},
	},
	{
		name: "mixed-simple-action-second-run",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusDone),
			ActionStep{Name: "step2"},
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]bool{
			"step2Run": true,
		},
	},
	{
		name: "mixed-simple-action-all-done",
		steps: []StepType{
			newSelfManagedStep("step1", StepSyncStatusDone),
			ActionStep{Name: "step2"},
		},
		storageBefore: map[string]bool{
			"step2Run":  true,
			"step2Done": true,
		},
		expectedStepStatus:   StepSyncStatusDone,
		expectedStorageAfter: map[string]bool{},
	},
	{
		name: "mixed-action-simple-first-run",
		steps: []StepType{
			ActionStep{Name: "step1"},
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]bool{
			"step1Run": true,
		},
	},
	{
		name: "mixed-action-simple-second-run",
		steps: []StepType{
			ActionStep{Name: "step1"},
			newSelfManagedStep("step2", StepSyncStatusNeedRun),
		},
		storageBefore: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
		},
		expectedStepStatus: StepSyncStatusUpdating,
		expectedStorageAfter: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
		},
	},
	{
		name: "mixed-action-simple-all-done",
		steps: []StepType{
			ActionStep{Name: "step1"},
			newSelfManagedStep("step2", StepSyncStatusDone),
		},
		storageBefore: map[string]bool{
			"step1Run":  true,
			"step1Done": true,
		},
		expectedStepStatus:   StepSyncStatusDone,
		expectedStorageAfter: map[string]bool{},
	},
}

func TestAdvance(t *testing.T) {
	for _, testCase := range advanceCases {
		t.Run(testCase.name, func(t *testing.T) {
			storage := newTestStorage(testCase.storageBefore)
			flow := NewFlow(testCase.steps, storage, testr.New(t))

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
