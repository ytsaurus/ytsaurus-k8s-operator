package ytflow

import (
	"context"
)

type fakeActionStep struct {
	name           StepName
	state          stateManager
	condsOnSuccess []Condition

	spy *executionSpy
}

func newFakeActionStep(name StepName, spy *executionSpy, state stateManager) *fakeActionStep {
	return &fakeActionStep{
		name:           name,
		state:          state,
		condsOnSuccess: []Condition{},

		spy: spy,
	}
}

func (c *fakeActionStep) onSuccess(conds ...Condition) {
	c.condsOnSuccess = conds
}

func (c *fakeActionStep) Run(ctx context.Context) error {
	c.spy.record(string(c.name))
	for _, cond := range c.condsOnSuccess {
		_ = c.state.Set(ctx, cond.Name, cond.Val, "test run")
	}
	return nil
}
