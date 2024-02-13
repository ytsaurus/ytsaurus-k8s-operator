package controllers

import (
	"context"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type baseStep struct {
	name         string
	runCondition func(context.Context) (bool, string, error)
}
type dummyStep struct {
	name string
}
type componentStep struct {
	baseStep
	component components.Component2
}
type actionStep struct {
	baseStep
	action    func(context.Context) error
	doneCheck func(context.Context) (bool, error)
}

func (s *baseStep) GetName() string {
	return s.name
}
func (s *baseStep) ShouldRun(ctx context.Context) (bool, string, error) {
	if s.runCondition == nil {
		return true, "no run condition is set", nil
	}
	shouldRun, comment, err := s.runCondition(ctx)
	return shouldRun, comment, err
}

func newComponentStep(component components.Component2) *componentStep {
	return &componentStep{
		baseStep: baseStep{
			name: component.GetName(),
		},
		component: component,
	}
}
func (s *componentStep) WithRunCondition(condition func(context.Context) (bool, string, error)) *componentStep {
	s.runCondition = condition
	return s
}
func (s *componentStep) Done(ctx context.Context) (bool, error) {
	err := s.component.Fetch(ctx)
	if err != nil {
		return false, err
	}
	status, err := s.component.Status2(ctx)
	return status.IsReady(), err
}
func (s *componentStep) Status(ctx context.Context) (components.ComponentStatus, error) {
	err := s.component.Fetch(ctx)
	if err != nil {
		return components.ComponentStatus{}, err
	}
	return s.component.Status2(ctx)
}
func (s *componentStep) Run(ctx context.Context) error {
	err := s.component.Fetch(ctx)
	if err != nil {
		return err
	}
	return s.component.Sync2(ctx)
}

func newActionStep(
	name string,
	action func(context.Context) error,
	doneCheck func(context.Context) (bool, error),
) *actionStep {
	return &actionStep{
		baseStep: baseStep{
			name: name,
		},
		action:    action,
		doneCheck: doneCheck,
	}
}
func (s *actionStep) WithRunCondition(condition func(ctx context.Context) (bool, string, error)) *actionStep {
	s.runCondition = condition
	return s
}
func (s *actionStep) Done(ctx context.Context) (bool, error) {
	return s.doneCheck(ctx)
}
func (s *actionStep) Status(ctx context.Context) (components.ComponentStatus, error) {
	done, err := s.Done(ctx)
	if err != nil {
		return components.ComponentStatus{}, err
	}
	if done {
		return components.SimpleStatus(components.SyncStatusReady), nil
	}
	return components.SimpleStatus(components.SyncStatusNeedLocalUpdate), nil
}
func (s *actionStep) Run(ctx context.Context) error {
	return s.action(ctx)
}

func newDummyStep() *dummyStep {
	return &dummyStep{}
}
func (s *dummyStep) Skip() bool {
	return true
}
func (s *dummyStep) Done(ctx context.Context) (bool, error) {
	return true, nil
}
func (s *dummyStep) Do(_ context.Context) error {
	return nil
}
