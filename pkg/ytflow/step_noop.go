package ytflow

import (
	"context"
)

type noopStep struct {
	cond  Condition
	state stateManager
}

func newNoopStep(cond Condition, state stateManager) *noopStep {
	return &noopStep{
		cond:  cond,
		state: state,
	}
}

func (s *noopStep) Run(ctx context.Context) error {
	return s.state.Set(ctx, s.cond.Name, s.cond.Val, "noop")
}
