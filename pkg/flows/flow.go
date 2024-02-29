package flows

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
)

type StepType interface {
	StepName() StepName
}

type runnableStepType interface {
	StepType
	Run(context.Context) error
}

type runnableStatusStepType interface {
	runnableStepType
	Status(context.Context) (StepStatus, error)
	// Cacheable for runnableStatusStepType means that Status will be called until it returns successfully Done status.
	// N.B. Run method could be called multiple times if Status returns NeedRun status.
	Cacheable() bool
}

type conditionStepType interface {
	StepType
	GetBranches() map[string][]StepType
	RunCondition(ctx context.Context) (string, error)
	// Cacheable for conditionStepType it means that RunCondition result will be saved and used
	// until for branch choosing until the flow is complete.
	Cacheable() bool
}

type stateStorage interface {
	Put(string)
	Exists(string) bool
	Clear(context.Context) error
}

type Flow struct {
	steps   []StepType
	storage stateStorage
	logger  logr.Logger
}

func NewFlow(steps []StepType, storage stateStorage, logger logr.Logger) *Flow {
	return &Flow{
		steps:   steps,
		storage: storage,
		logger:  logger,
	}
}

// Advance figures out the next step of the flow (if any) and executes it.
// It returns chosen step status, so caller code could decide when and if to call it next time.
func (f *Flow) Advance(ctx context.Context) (StepSyncStatus, error) {
	step, status, err := f.getNextStep(ctx)
	if err != nil {
		return "", err
	}
	if step == nil {
		// Flow is complete, no steps to run.
		return StepSyncStatusDone, nil
	}

	stepSyncStatus := status.SyncStatus
	switch stepSyncStatus {
	case StepSyncStatusUpdating:
		return StepSyncStatusUpdating, nil
	case StepSyncStatusBlocked:
		return StepSyncStatusBlocked, nil
	case StepSyncStatusNeedRun:
		return StepSyncStatusUpdating, f.runStep(step, ctx)
	default:
		return "", errors.New("unexpected step sync status: " + string(stepSyncStatus))
	}
}

func (f *Flow) logStep(step StepType, status StepStatus, err error) {
	statusToIcon := map[StepSyncStatus]string{
		StepSyncStatusDone:     "[v]",
		StepSyncStatusUpdating: "[.]",
		StepSyncStatusBlocked:  "[x]",
		StepSyncStatusNeedRun:  "[ ]",
	}

	var icon string
	if err != nil {
		icon = "[E]"
	} else {
		icon = statusToIcon[status.SyncStatus]

	}
	line := fmt.Sprintf("%s %s", icon, step.StepName())
	if status.Message != "" {
		line += ": " + status.Message
	}
	f.logger.Info(line)
}

func (f *Flow) getNextStep(ctx context.Context) (runnableStepType, StepStatus, error) {
	var err error
	idx := 0
	steps := f.steps
	for {
		if idx == len(steps) {
			break
		}
		nextStep := steps[idx]

		switch step := nextStep.(type) {
		case runnableStatusStepType:
			var status StepStatus
			status, err = f.getStepStatus(ctx, step)
			// TODO: maybe support all steps logging in the end of flow
			f.logStep(step, status, err)
			if err != nil {
				return nil, StepStatus{}, err
			}

			if status.SyncStatus == StepSyncStatusDone {
				idx++
				continue
			}
			return step, status, nil
		case runnableStepType:
			if f.isDoneMarkExist(step.StepName()) {
				idx++
				continue
			}
			return step, StepStatus{StepSyncStatusNeedRun, "step haven't being run yet"}, nil
		case conditionStepType:
			// If we've met ConditionStep we follow the branch chosen according the condition.
			steps, err = f.getConditionBranch(ctx, step)
			if err != nil {
				return nil, StepStatus{}, err
			}
			// We have a new slice of steps to follow after the condition step.
			idx = 0
			continue
		default:
			return nil, StepStatus{}, fmt.Errorf("unexpected type of step %s in flow", step.StepName())
		}
	}

	// Flow is complete, no more steps to run, need to reset the state.
	err = f.reset(ctx)
	if err != nil {
		return nil, StepStatus{}, err
	}
	return nil, StepStatus{}, nil
}

func (f *Flow) runStep(nextStep runnableStepType, ctx context.Context) error {
	err := nextStep.Run(ctx)
	if err != nil {
		return fmt.Errorf("step %s execution failed: %s", nextStep.StepName(), err)
	}

	// Here we save Done state for runnableStepType (those which don't have Status())
	// and do nothing for runnableStatusStepType (their Done state is saved on Status() return Done).
	switch step := nextStep.(type) {
	case runnableStatusStepType:
		// Do nothing for that type.
	case runnableStepType:
		f.setDoneMark(step.StepName())
	}

	return nil
}

func (f *Flow) getConditionBranch(ctx context.Context, condStep conditionStepType) ([]StepType, error) {
	var chosenBranchName string
	var markExists bool
	var err error

	getBranch := func(stepName StepName, branchName string) ([]StepType, error) {
		if branch, exists := condStep.GetBranches()[chosenBranchName]; exists {
			return branch, nil
		}
		return nil, fmt.Errorf(
			"executed condition step %s have returned unexpected branch name: %s",
			condStep.StepName(),
			chosenBranchName,
		)
	}

	// Checking if condition has been already ran in a flow.
	if condStep.Cacheable() {
		chosenBranchName, markExists = f.isConditionMarkExists(condStep)
		if markExists {
			return getBranch(condStep.StepName(), chosenBranchName)
		}
	}

	// Either not cacheable or not yet ran.
	chosenBranchName, err = condStep.RunCondition(ctx)
	if err != nil {
		return []StepType{}, fmt.Errorf("failed to run fork %s: %w", condStep.StepName(), err)
	}

	// If executed successfully and is cacheable — storing result.
	if condStep.Cacheable() {
		condStepMark := f.getConditionMark(condStep.StepName(), chosenBranchName)
		f.storage.Put(condStepMark)
	}

	return getBranch(condStep.StepName(), chosenBranchName)
}

func (f *Flow) getStepStatus(ctx context.Context, step runnableStatusStepType) (StepStatus, error) {
	// Checking if state has been already successfully executed (we've received Done status).
	if step.Cacheable() {
		if f.isDoneMarkExist(step.StepName()) {
			return StepStatus{
				SyncStatus: StepSyncStatusDone,
				Message:    "Step was successfully done in previous iterations",
			}, nil
		}
	}

	// Either not cacheable, or not yet successfully executed.
	status, err := step.Status(ctx)
	if err != nil {
		return StepStatus{}, fmt.Errorf("failed to get status for step %s: %w", step.StepName(), err)
	}

	// Saving status only in case of successfully and fully executed.
	if step.Cacheable() && status.SyncStatus == StepSyncStatusDone {
		f.setDoneMark(step.StepName())
	}

	return status, nil
}

func (f *Flow) reset(ctx context.Context) error {
	return f.storage.Clear(ctx)
}

func (f *Flow) getDoneMark(name StepName) string {
	return fmt.Sprintf("%sDone", name)
}

func (f *Flow) isDoneMarkExist(name StepName) bool {
	mark := f.getDoneMark(name)
	return f.storage.Exists(mark)
}

func (f *Flow) setDoneMark(name StepName) {
	mark := f.getDoneMark(name)
	f.storage.Put(mark)
}

func (f *Flow) getConditionMark(name StepName, result string) string {
	return fmt.Sprintf("%s%s", name, result)
}

func (f *Flow) isConditionMarkExists(condStep conditionStepType) (string, bool) {
	for key := range condStep.GetBranches() {
		markToCheck := f.getConditionMark(condStep.StepName(), key)
		if f.storage.Exists(markToCheck) {
			return key, true
		}
	}
	return "", false
}
