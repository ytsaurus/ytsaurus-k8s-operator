package components

import (
	"context"
	"fmt"
)

type Step interface {
	StepName() string
	// Status answers if the body can and should be run.
	// If st == Ready — no need to run.
	// If st == NeedSync — need to run.
	// If st == Blocked — run is impossible.
	// msg is an optional human-readable explanation of status.
	// err is an error of status checking.
	Status(ctx context.Context, conds conditionManagerIface) (st SyncStatus, msg string, err error)
	// Run is a body of a step, it will be run only if Status() returns NeedSync.
	// If ok is true and no error, run considered successful.
	Run(ctx context.Context, conds conditionManagerIface) (ok bool, err error)
	// PostRun can be used for cleanup or some other post-actions,
	// typical usage is to clean up some intermediate conditions which are set in step.
	PostRun(ctx context.Context, conds conditionManagerIface) error
}

// statusCommon is reusable part for calculation step status.
// It decides if step is ready/blocked/need sync based on statusCondition and statusFunc
// (ones that not provided considered as satisfied and resulted in Ready status).
func statusCommon(
	ctx context.Context,
	conds conditionManagerIface,
	statusCondition Condition,
	statusFunc func(ctx context.Context) (st SyncStatus, msg string, err error),
) (SyncStatus, string, error) {
	if statusCondition.Name != "" && conds.IsNotSatisfied(statusCondition) {
		return SyncStatusReady, fmt.Sprintf("condtion %s is not satisfied", statusCondition), nil
	}
	if statusFunc != nil {
		return statusFunc(ctx)
	}
	return SyncStatusNeedSync, fmt.Sprintf("condtion is satisfied"), nil
}

func postRunCommon(
	ctx context.Context,
	conds conditionManagerIface,
	onSuccessCondition Condition,
	onSuccessFunc func(ctx context.Context) error,
) error {
	if onSuccessCondition.Name != "" {
		if err := conds.SetCond(ctx, onSuccessCondition); err != nil {
			return err
		}
	}
	if onSuccessFunc != nil {
		return onSuccessFunc(ctx)
	}
	return nil
}

// StepRun has RunFunc which only returns error, run considered successful if no error returned.
type StepRun struct {
	// NB: common part is copy-pasted to make usage more concise and readable.
	Name string
	// RunIfCondition should be satisfied for step to run.
	RunIfCondition Condition
	// StatusFunc should return NeedSync status for step to run.
	// If both RunIfCondition and StatusFunc a specified, then both should be resolved as true.
	StatusFunc func(ctx context.Context) (st SyncStatus, msg string, err error)

	// OnSuccessCondition will be set after successful execution of the step.
	OnSuccessCondition Condition
	// OnSuccessCondition will be called after successful execution of the step.
	OnSuccessFunc func(ctx context.Context) error

	RunFunc func(ctx context.Context) error
}

func (s StepRun) StepName() string { return s.Name }
func (s StepRun) Status(ctx context.Context, conds conditionManagerIface) (SyncStatus, string, error) {
	return statusCommon(ctx, conds, s.RunIfCondition, s.StatusFunc)
}
func (s StepRun) PostRun(ctx context.Context, conds conditionManagerIface) error {
	return postRunCommon(ctx, conds, s.OnSuccessCondition, s.OnSuccessFunc)
}
func (s StepRun) Run(ctx context.Context, _ conditionManagerIface) (bool, error) {
	if s.RunFunc == nil {
		return true, nil
	}
	if err := s.RunFunc(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// StepCheck has RunFunc which returns ok and error, run considered successful if ok is true and no error returned.
type StepCheck struct {
	// NB: common part is copy-pasted to make usage more concise and readable.
	Name string
	// RunIfCondition should be satisfied for step to run.
	RunIfCondition Condition
	// StatusFunc should return NeedSync status for step to run.
	// If both RunIfCondition and StatusFunc a specified, then both should be resolved as true.
	StatusFunc func(ctx context.Context) (st SyncStatus, msg string, err error)

	// OnSuccessCondition will be set after successful execution of the step.
	OnSuccessCondition Condition
	// OnSuccessCondition will be called after successful execution of the step.
	OnSuccessFunc func(ctx context.Context) error

	RunFunc func(ctx context.Context) (ok bool, err error)
}

func (s StepCheck) StepName() string { return s.Name }
func (s StepCheck) Status(ctx context.Context, conds conditionManagerIface) (SyncStatus, string, error) {
	return statusCommon(ctx, conds, s.RunIfCondition, s.StatusFunc)
}
func (s StepCheck) PostRun(ctx context.Context, conds conditionManagerIface) error {
	return postRunCommon(ctx, conds, s.OnSuccessCondition, s.OnSuccessFunc)
}
func (s StepCheck) Run(ctx context.Context, _ conditionManagerIface) (bool, error) {
	if s.RunFunc == nil {
		return true, nil
	}
	return s.RunFunc(ctx)
}

type StepComposite struct {
	// NB: common part is copy-pasted to make usage more concise and readable.
	Name string
	// RunIfCondition should be satisfied for step to run.
	RunIfCondition Condition
	// StatusConditionFunc should return NeedSync status for step to run.
	// If both RunIfCondition and StatusConditionFunc a specified, then both should be resolved as true.
	StatusConditionFunc func(ctx context.Context) (st SyncStatus, msg string, err error)

	// OnSuccessCondition will be set after successful execution of the step.
	OnSuccessCondition Condition
	// OnSuccessCondition will be called after successful execution of the step.
	OnSuccessFunc func(ctx context.Context) error

	Steps []Step
}

func (s StepComposite) StepName() string { return s.Name }
func (s StepComposite) Run(ctx context.Context, conds conditionManagerIface) (bool, error) {
	for _, step := range s.Steps {
		st, _, err := step.Status(ctx, conds)
		if err != nil {
			return false, err
		}
		if st == SyncStatusReady {
			continue
		}

		runOk, err := step.Run(ctx, conds)
		if err != nil {
			return false, err
		}
		if !runOk {
			return false, nil
		}

		if err = step.PostRun(ctx, conds); err != nil {
			return false, err
		}
	}
	return true, nil
}

// Status of StepComposite is more complex:
//   - at first it checks if it itself need to run
//   - and since its body is other steps — it locates first not-ready step and return its status,
//     so Run method would go in step list to found step and execute it.
func (s StepComposite) Status(ctx context.Context, conds conditionManagerIface) (SyncStatus, string, error) {
	st, msg, err := statusCommon(ctx, conds, s.RunIfCondition, s.StatusConditionFunc)
	if st == SyncStatusReady || err != nil {
		return st, msg, err
	}

	for _, step := range s.Steps {
		st, msg, err = step.Status(ctx, conds)
		if err != nil {
			return "", msg, err
		}
		if st != SyncStatusReady {
			return st, msg, nil
		}
	}
	return SyncStatusReady, "all substeps are done", nil
}
func (s StepComposite) PostRun(ctx context.Context, conds conditionManagerIface) error {
	return postRunCommon(ctx, conds, s.OnSuccessCondition, s.OnSuccessFunc)
}
