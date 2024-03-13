package ytflow

import (
	"context"
	"fmt"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

func updateComponentsBasedConditions(ctx context.Context, statuses *statusRegistry, conds stateManager) error {
	allBuilt := true

	// Actualize <ComponentName>Built condition for the single components.
	for compName, status := range statuses.single {
		cond := isBuilt(compName)
		isComponentBuilt := status.SyncStatus == components.SyncStatusReady
		if !isComponentBuilt {
			allBuilt = false
		}
		msg := fmt.Sprintf("%s: %s", status.SyncStatus, status.Message)
		if err := conds.Set(ctx, cond.name, isComponentBuilt, msg); err != nil {
			return err
		}
	}

	// Actualize <ComponentName>Built condition for the multi components.
	for compName, substatuses := range statuses.multi {
		condName := isBuiltCondName(compName)

		allSubcomponentsBuilt := true
		msg := ""
		for subComponentName, status := range substatuses {
			if status.SyncStatus != components.SyncStatusReady {
				allBuilt = false
				allSubcomponentsBuilt = false
				msg = fmt.Sprintf("%s for %s: %s", status.SyncStatus, subComponentName, status.Message)
			}
		}
		if err := conds.Set(ctx, condName, allSubcomponentsBuilt, msg); err != nil {
			return err
		}
	}

	// Actualize AllComponentsBuilt
	// TODO: maybe message in case of not built would be useful
	if err := conds.Set(ctx, AllComponentsBuiltCondName, allBuilt, ""); err != nil {
		return err
	}

	// Actualize NeedFullUpdate
	isFullUpdateNeeded := statuses.single[MasterName].SyncStatus == components.SyncStatusNeedLocalUpdate
	if err := conds.Set(ctx, IsFullUpdateNeededCond, isFullUpdateNeeded, ""); err != nil {
		return err
	}

	return nil
}

func updateClusterBasedConditions(ctx context.Context, state stateManager) error {
	if state.GetClusterState() == ytv1.ClusterStateCreated {
		for _, dep := range initialDependencies {
			if err := state.Set(ctx, dep.name, dep.val, "set on cluster create"); err != nil {
				return err
			}
		}
	}
	return state.SetClusterState(ctx, ytv1.ClusterStateInitializing)
}