package ytflow

import (
	"context"
)

// TODO: merge with componentRegistry
type statusRegistry struct {
	single map[ComponentName]componentStatus
	multi  map[ComponentName]map[string]componentStatus
}

func observe(ctx context.Context, comps *componentRegistry) (*statusRegistry, error) {
	single := make(map[ComponentName]componentStatus)
	multi := make(map[ComponentName]map[string]componentStatus)

	// Fetch stage.
	for _, comp := range comps.single {
		if err := comp.Fetch(ctx); err != nil {
			return nil, err
		}
	}
	for _, compList := range comps.multi {
		for _, comp := range compList {
			if err := comp.Fetch(ctx); err != nil {
				return nil, err
			}
		}
	}

	// Status stage.
	for name, comp := range comps.single {
		single[name] = comp.Status(ctx)
	}
	for name, compList := range comps.multi {
		submap := make(map[string]componentStatus, len(compList))
		for _, comp := range compList {
			submap[comp.GetName()] = comp.Status(ctx)
		}
		multi[name] = submap
	}

	return &statusRegistry{
		single: single,
		multi:  multi,
	}, nil
}
