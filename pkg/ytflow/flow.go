package ytflow

import (
	"context"
	"fmt"

	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
)

func Advance(ctx context.Context, ytsaurus *apiProxy.Ytsaurus, clusterDomain string) error {
	// build components
	comps := buildComponents(ytsaurus, clusterDomain)

	// fetch all components and collect all statuses
	status, err := observe(ctx, comps)
	if err != nil {
		return fmt.Errorf("failed to observe statuses: %w", err)
	}

	// create conditions (or status?) manager
	// set conditions based on statuses
	// set conditions based on conditions (while true? with limit on cycles)

	// create steps registry
	steps := buildSteps(comps, status)
	// collect steps that can be run based on conditions
	// run each step and store results
	return nil
}
