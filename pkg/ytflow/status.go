package ytflow

import (
	"context"
)

type statusRegistry struct {
	statuses map[ComponentName]componentStatus
}

func observe(ctx context.Context, comps *componentRegistry) (*statusRegistry, error) {
	statuses := make(map[ComponentName]componentStatus)

	// Fetch stage.
	for _, comp := range comps.components {
		if err := comp.Fetch(ctx); err != nil {
			return nil, err
		}
	}

	// Status stage.
	for name, comp := range comps.components {
		statuses[name] = comp.Status(ctx)
	}

	return &statusRegistry{
		statuses: statuses,
	}, nil
}
