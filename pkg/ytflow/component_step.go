package ytflow

import (
	"context"
	"sort"
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
	// Just for the test stability we execute steps in predictable order.
	var keys []string
	for key := range s.comps {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		comp := s.comps[key]
		if err := comp.Sync(ctx); err != nil {
			return err
		}
	}
	return nil
}
