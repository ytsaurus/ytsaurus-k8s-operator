package ytflow

import (
	"context"
)

type statusRegistry struct {
	single map[ComponentName]componentStatus
}

func observe(ctx context.Context, comps *componentRegistry) (*statusRegistry, error) {
	single := make(map[ComponentName]componentStatus)

	// Fetch stage.
	for _, comp := range comps.single {
		if err := comp.Fetch(ctx); err != nil {
			return nil, err
		}
	}

	// Status stage.
	for name, comp := range comps.single {
		single[name] = comp.Status(ctx)
	}

	return &statusRegistry{
		single: single,
	}, nil
}
