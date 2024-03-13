package ytflow

import (
	"context"
)

type fakeActionStep struct {
	name StepName

	spy *executionSpy
}

func newFakeActionStep(name StepName, spy *executionSpy) *fakeActionStep {
	return &fakeActionStep{
		name: name,
		spy:  spy,
	}
}

func (c *fakeActionStep) Run(_ context.Context) error {
	c.spy.record(string(c.name))
	return nil
}
