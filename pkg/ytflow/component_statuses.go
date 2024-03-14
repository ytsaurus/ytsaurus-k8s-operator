package ytflow

import (
	"context"
	"fmt"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

func updateComponentsBasedConditions(ctx context.Context, statuses *statusRegistry, conds stateManager) error {
	allBuilt := true

	// Actualize `built` and `needSync` conditions for the single components.
	for compName, status := range statuses.single {
		cond := isBuilt(compName)
		isComponentBuilt := status.SyncStatus == components.SyncStatusReady
		if !isComponentBuilt {
			allBuilt = false
		}
		msg := fmt.Sprintf("%s: %s", status.SyncStatus, status.Message)
		if err := conds.Set(ctx, cond.Name, isComponentBuilt, msg); err != nil {
			return err
		}
	}

	// Actualize `built` and `needSync` conditions for the multi components.
	for compName, substatuses := range statuses.multi {
		condName := isBuilt(compName).Name

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
	if err := conds.Set(ctx, AllComponentsBuilt.Name, allBuilt, ""); err != nil {
		return err
	}

	// Actualize NeedFullUpdate
	isFullUpdateNeeded := statuses.single[MasterName].SyncStatus == components.SyncStatusNeedLocalUpdate
	if err := conds.Set(ctx, FullUpdateNeeded.Name, isFullUpdateNeeded, ""); err != nil {
		return err
	}

	return nil
}

func updateSpecialConditions(ctx context.Context, state stateManager) error {
	clusterCreated := state.GetClusterState() == ytv1.ClusterStateCreated
	isInReadOnly := state.Get(MasterIsInReadOnly.Name)

	// This could be improved by implementing OR for conditions deps.
	// Since we only have master now it may not being worth it.
	masterCanBeSynced := clusterCreated || isInReadOnly
	return state.Set(ctx,
		MasterCanBeSynced.Name, masterCanBeSynced,
		fmt.Sprintf("cluster just created = %t || master is in read only = %t ", clusterCreated, isInReadOnly),
	)
}
