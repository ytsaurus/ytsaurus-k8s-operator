package ytflow

import (
	"context"
)

type singleComponentStep struct {
	comp component
}

func newSingleComponentStep(comp component) *singleComponentStep {
	return &singleComponentStep{
		comp: comp,
	}
}

func (s *singleComponentStep) Run(ctx context.Context) error {
	return s.comp.Sync(ctx)
}

type multiComponentStep struct {
	comps map[string]component
}

func newMultiComponentStep(comps map[string]component) *multiComponentStep {
	return &multiComponentStep{
		comps: comps,
	}
}

func (s *multiComponentStep) Run(ctx context.Context) error {
	for _, comp := range s.comps {
		if err := comp.Sync(ctx); err != nil {
			return err
		}
	}
	return nil
}
