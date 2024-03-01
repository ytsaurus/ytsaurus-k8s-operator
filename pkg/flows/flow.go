package flows

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"golang.org/x/exp/slices"
)

type StepType interface {
	StepName() StepName
}

type runnableStepType interface {
	StepType
	Run(context.Context) error

	// Cacheable == true means that Run and Done events will be cached until the end of a flow.
	// I.e. in onw flow Run wouldn't be executed more than once if returned nil error and
	// PostRun will be executed until it returns StepSyncStatusDone.
	//Cacheable() bool
}

var (
	validPreRunStatuses = []StepSyncStatus{
		StepSyncStatusNeedRun,
		StepSyncStatusBlocked,
		StepSyncStatusUpdating,
	}
	validPostRunStatuses = []StepSyncStatus{
		StepSyncStatusDone,
		StepSyncStatusUpdating,
		StepSyncStatusBlocked,
	}
)

// autoManagedRunnableStep steps execution is partially controlled by a flow.
// Run and Done events will be cached until the end of a flow.
// I.e. in a single flow Run method wouldn't be called more than once (except it returns an error) and
// PostRun will be executed until it returns StepSyncStatusDone.
type autoManagedRunnableStep interface {
	runnableStepType
	// PreRun is executed before Run. It can do some preparation work.
	// It should return one of the statuses:
	//  - StepSyncStatusNeedRun
	//  - StepSyncStatusBlocked
	//  - StepSyncStatusUpdating
	PreRun(context.Context) (StepStatus, error)
	Run(context.Context) error
	// PostRun is executed after Run. It can do some preparation work.
	// It should return one of the statuses:
	//  - StepSyncStatusDone
	//  - StepSyncStatusUpdating
	//  - StepSyncStatusBlocked
	PostRun(context.Context) (StepStatus, error)
}

type selfManagedRunnableStep interface {
	runnableStepType
	// Status is called before Run.
	// It must say if step should be run or be waited or already done.
	// Any existing status is valid to return:
	//  - StepSyncStatusNeedRun
	//  - StepSyncStatusBlocked
	//  - StepSyncStatusUpdating
	//  - StepSyncStatusDone
	Status(context.Context) (StepStatus, error)
}

type boolConditionStepType interface {
	StepType
	GetBranch(value bool) []StepType
	RunCondition(ctx context.Context) (bool, error)
	// AutoManaged == true means that RunCondition result will be saved and same
	// branch will be chosen until the flow is complete.
	AutoManaged() bool
}

type stateStorage interface {
	StoreRun(ctx context.Context, name StepName) error
	StoreDone(ctx context.Context, name StepName) error
	HasRun(name StepName) bool
	IsDone(name StepName) bool
	StoreConditionResult(ctx context.Context, name StepName, result bool) error
	GetConditionResult(name StepName) (result bool, ok bool)
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
		return StepSyncStatusUpdating, f.runStep(ctx, step)
	default:
		return "", errors.New("unexpected step sync status: " + string(stepSyncStatus))
	}
}

func (f *Flow) logStep(step StepType, status StepStatus, err error) {
	// TODO: maybe support all steps logging in the end of flow
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
		name := nextStep.StepName()
		if name == "" {
			return nil, StepStatus{}, fmt.Errorf("empty step name")
		}
		var status StepStatus

		switch step := nextStep.(type) {
		case runnableStepType:
			status, err = f.getRunnableStepStatus(ctx, step)
			f.logStep(step, status, err)
			if err != nil {
				return nil, StepStatus{}, fmt.Errorf("failed to collect status for step %s: %w", name, err)
			}

			if status.SyncStatus != StepSyncStatusDone {
				// Either wait or execute.
				return step, status, nil
			}

			// Step is done — go to the next one.
			idx++
			continue
		case boolConditionStepType:
			// If we've met BoolConditionStep we follow the branch chosen according the condition.
			steps, err = f.getBoolConditionBranch(ctx, step)
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

func (f *Flow) runStep(ctx context.Context, nextStep runnableStepType) error {
	err := nextStep.Run(ctx)
	if err != nil {
		return fmt.Errorf("step %s execution failed: %s", nextStep.StepName(), err)
	}

	if _, isAuto := nextStep.(autoManagedRunnableStep); isAuto {
		if err = f.storage.StoreRun(ctx, nextStep.StepName()); err != nil {
			return err
		}
	}
	return nil
}

func (f *Flow) getBoolConditionBranch(ctx context.Context, condStep boolConditionStepType) ([]StepType, error) {
	var chosenBoolBranch bool
	var valueStored bool
	var err error
	name := condStep.StepName()

	// Checking if condition has been already ran in a flow.
	if condStep.AutoManaged() {
		chosenBoolBranch, valueStored = f.storage.GetConditionResult(name)
		if valueStored {
			return condStep.GetBranch(chosenBoolBranch), nil
		}
	}

	// Either step is self-managed or not ran yet.
	chosenBoolBranch, err = condStep.RunCondition(ctx)
	if err != nil {
		return []StepType{}, fmt.Errorf("failed to choose branch of condition step %s: %w", condStep.StepName(), err)
	}

	// If executed successfully and is cacheable — storing result.
	if condStep.AutoManaged() {
		if err = f.storage.StoreConditionResult(ctx, name, chosenBoolBranch); err != nil {
			return nil, err
		}
	}

	return condStep.GetBranch(chosenBoolBranch), nil
}

func (f *Flow) getRunnableStepStatus(ctx context.Context, runnableStep runnableStepType) (StepStatus, error) {
	switch step := runnableStep.(type) {
	case selfManagedRunnableStep:
		return step.Status(ctx)
	case autoManagedRunnableStep:
		return f.getAutoManagedStepStatus(ctx, step)
	default:
		return StepStatus{}, fmt.Errorf("unexpected runnable step `%s` type", step.StepName())
	}
}

func (f *Flow) getAutoManagedStepStatus(ctx context.Context, step autoManagedRunnableStep) (StepStatus, error) {
	name := step.StepName()
	if !f.storage.HasRun(name) {
		preRunStatus, err := step.PreRun(ctx)
		if err != nil {
			return StepStatus{}, fmt.Errorf("failed to prerun step %s: %w", name, err)
		}
		preRunSyncStatus := preRunStatus.SyncStatus
		if !slices.Contains(validPreRunStatuses, preRunSyncStatus) {
			return StepStatus{}, fmt.Errorf("unexpected PreRun() status: %s of auto managed step %s", preRunSyncStatus, name)
		}
		return preRunStatus, nil
	}
	// Step already been successfully run before.

	if f.storage.IsDone(name) {
		return StepStatus{StepSyncStatusDone, "step was done earlier"}, nil
	}

	postRunStatus, err := step.PostRun(ctx)
	if err != nil {
		return StepStatus{}, err
	}
	syncStatus := postRunStatus.SyncStatus
	if !slices.Contains(validPostRunStatuses, syncStatus) {
		return StepStatus{}, fmt.Errorf("unexpected postRun() status: %s of cacheable step %s", syncStatus, name)
	}

	if syncStatus == StepSyncStatusDone {
		if err = f.storage.StoreDone(ctx, name); err != nil {
			return StepStatus{}, err
		}
		return StepStatus{StepSyncStatusDone, "step just have become done"}, nil
	}
	return postRunStatus, nil
}

func (f *Flow) reset(ctx context.Context) error {
	return f.storage.Clear(ctx)
}

//func (f *Flow) isConditionMarkExists(condStep boolConditionStepType) (string, bool) {
//	for key := range condStep.GetBranches() {
//		markToCheck := f.getConditionMark(condStep.StepName(), key)
//		if f.storage.IsDone(markToCheck) {
//			return key, true
//		}
//	}
//	return "", false
//}
