package ytflow

import (
	"context"
)

type componentStep struct {
	comp component
}

func newComponentStep(comp component) *componentStep {
	return &componentStep{
		comp: comp,
	}
}

func (s *componentStep) Run(ctx context.Context) error {
	return s.comp.Sync(ctx)
}
