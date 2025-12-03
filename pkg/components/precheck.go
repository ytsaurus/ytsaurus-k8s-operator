package components

import (
	"context"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type UpdatePreChecker interface {
	PreCheck(ctx context.Context, component Component) error
}

// UpdatePreCheckerFunc allows to use simple functions as UpdatePreChecker
// and don't implement PreCheck method in every component.
type UpdatePreCheckerFunc func(ctx context.Context, component Component) error

func (f UpdatePreCheckerFunc) PreCheck(ctx context.Context, component Component) error {
	return f(ctx, component)
}

//nolint:gochecknoglobals // registry of update pre-checkers, populated by init() in component packages
var updatePreCheckers = map[consts.ComponentType]UpdatePreChecker{}

// Component packages call this in their init() to register their specific checker.
func RegisterUpdatePreChecker(componentType consts.ComponentType, checker UpdatePreChecker) {
	if checker == nil {
		return
	}
	updatePreCheckers[componentType] = checker
}

func RunUpdatePreCheck(ctx context.Context, component Component) error {
	checker, ok := updatePreCheckers[component.GetType()]
	if !ok {
		return nil
	}
	return checker.PreCheck(ctx, component)
}
