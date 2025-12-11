package components

import (
	"context"
	"fmt"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
)

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
