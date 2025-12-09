package components

import (
	"context"
	"fmt"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
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

// IsInstanceCountEqualYTSpec checks if the number of instances registered in YTsaurus
// matches the expected instanceCount from the spec.
func IsInstanceCountEqualYTSpec(ctx context.Context, ytClient yt.Client, componentType consts.ComponentType, expectedCount int) error {
	if ytClient == nil {
		return fmt.Errorf("YT client is not available")
	}

	cypressPath := consts.ComponentCypressPath(componentType)
	var instances []string
	err := ytClient.ListNode(ctx, ypath.Path(cypressPath), &instances, nil)
	if err != nil {
		return fmt.Errorf("failed to list %s instances from %s: %w", componentType, cypressPath, err)
	}

	actualCount := len(instances)
	if actualCount != expectedCount {
		return fmt.Errorf("%s instance count mismatch: expected %d, got %d instances in %s", componentType, expectedCount, actualCount, cypressPath)
	}

	return nil
}
