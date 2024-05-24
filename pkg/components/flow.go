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
	Status(ctx context.Context, conds conditionManagerIface) (ComponentStatus, error)
	// Run is a body of a step, it will be run only if Status() returns NeedSync.
	// If ok is true and no error, run considered successful and postRun executed.
	Run(ctx context.Context, conds conditionManagerIface) (ok bool, err error)
	// PostRun can be used for cleanup or some other post-actions,
	// typical usage is to clean up some intermediate conditions which are set in step.
	PostRun(ctx context.Context, conds conditionManagerIface) error
}

type StepMeta struct {
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
}

func (m StepMeta) StepName() string { return m.Name }

// Status decides if step is ready/blocked/need sync based on statusCondition and statusFunc
// (ones that not provided considered as satisfied and resulted in Ready status).
func (m StepMeta) Status(
	ctx context.Context,
	conds conditionManagerIface,
) (ComponentStatus, error) {
	if m.RunIfCondition.Name != "" && conds.IsNotSatisfied(m.RunIfCondition) {
		return ComponentStatus{
			SyncStatus: SyncStatusReady,
			Message:    fmt.Sprintf("%s ok", m.Name),
			Stage:      m.Name,
		}, nil
	}
	if m.StatusFunc != nil {
		syncSt, msg, err := m.StatusFunc(ctx)
		return ComponentStatus{
			SyncStatus: syncSt,
			Message:    msg,
			Stage:      m.Name,
		}, err
	}
	return ComponentStatus{
		SyncStatus: SyncStatusNeedSync,
		Message:    fmt.Sprintf("%s not done", m.Name),
		Stage:      m.Name,
	}, nil
}

func (m StepMeta) PostRun(
	ctx context.Context,
	conds conditionManagerIface,
) error {
	if m.OnSuccessCondition.Name != "" {
		if err := conds.SetCond(ctx, m.OnSuccessCondition); err != nil {
			return err
		}
	}
	if m.OnSuccessFunc != nil {
		return m.OnSuccessFunc(ctx)
	}
	return nil
}

// StepRun has Body func which only returns error,
// run considered successful if no error returned.
type StepRun struct {
	StepMeta
	Body func(ctx context.Context) error
}

func (s StepRun) Run(ctx context.Context, _ conditionManagerIface) (bool, error) {
	if s.Body == nil {
		return true, nil
	}
	err := s.Body(ctx)
	return err == nil, err
}

// StepCheck has Body func which returns ok and error,
// run considered successful if ok is true and no error returned.
type StepCheck struct {
	StepMeta
	Body func(ctx context.Context) (ok bool, err error)
}

func (s StepCheck) Run(ctx context.Context, _ conditionManagerIface) (bool, error) {
	if s.Body == nil {
		return true, nil
	}
	return s.Body(ctx)
}

type StepComposite struct {
	StepMeta
	Body []Step
}

func (s StepComposite) Run(ctx context.Context, conds conditionManagerIface) (bool, error) {
	for _, step := range s.Body {
		st, err := step.Status(ctx, conds)
		if err != nil {
			return false, err
		}
		if st.SyncStatus == SyncStatusReady {
			continue
		}

		runOk, err := step.Run(ctx, conds)
		if err != nil {
			return false, err
		}
		if runOk {
			err = step.PostRun(ctx, conds)
			return err != nil, err
		}
		return false, nil
	}
	return true, nil
}

// Status of StepComposite is more complex:
//   - at first it checks if it itself need to run
//   - and since its body consists of steps — it locates first not-ready step and return its status,
//     so Run method would go in step list to found step and execute it.
func (s StepComposite) Status(ctx context.Context, conds conditionManagerIface) (ComponentStatus, error) {
	st, err := s.StepMeta.Status(ctx, conds)
	if st.SyncStatus == SyncStatusReady || err != nil {
		return st, err
	}

	for _, step := range s.Body {
		st, err = step.Status(ctx, conds)
		if err != nil {
			return ComponentStatus{}, err
		}
		if st.SyncStatus != SyncStatusReady {
			return st, nil
		}
	}
	return ComponentStatus{
		SyncStatus: SyncStatusReady,
		Message:    "ok",
	}, nil
}
