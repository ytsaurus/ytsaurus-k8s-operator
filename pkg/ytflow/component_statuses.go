package ytflow

import (
	"context"
	"fmt"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

func interpretSyncStatus(st syncStatus) (isBuilt bool, needSync bool) {
	switch st {
	case components.SyncStatusReady:
		return true, false
	case components.SyncStatusNeedLocalUpdate:
		return true, true
	case components.SyncStatusPending:
		return false, true
	default:
		// Updating (for example wait pods to be deleted)
		// Blocked (for example wait pods to be created)
		return false, false
	}
}

func updateComponentsBasedConditions(ctx context.Context, statuses *statusRegistry, state stateManager) error {
	allSynced := true
	var becameBuilt []ComponentName

	// Actualize `built` and `needSync` conditions for the single components.
	for compName, status := range statuses.single {
		wasBuilt := state.Get(isBuilt(compName).Name)
		compIsBuilt, compNeedsSync := interpretSyncStatus(status.SyncStatus)
		msg := fmt.Sprintf("%s: %s", status.SyncStatus, status.Message)
		if err := state.Set(ctx, isBuilt(compName).Name, compIsBuilt, msg); err != nil {
			return err
		}
		if err := state.Set(ctx, needSync(compName).Name, compNeedsSync, msg); err != nil {
			return err
		}

		if !wasBuilt && compIsBuilt {
			becameBuilt = append(becameBuilt, compName)
		}

		if compNeedsSync {
			allSynced = false
		}
	}

	for _, compName := range becameBuilt {
		if compName == SchedulerName {
			if err := state.SetTrue(ctx, OperationArchiveNeedUpdate.Name, "scheduler have became built"); err != nil {
				return err
			}
		}
		if compName == QueryTrackerName {
			if err := state.SetTrue(ctx, QueryTrackerNeedsInit.Name, "query tracker have became built"); err != nil {
				return err
			}
		}
	}

	// Actualize AllComponentsSynced
	// TODO: maybe message what is not synced would be useful
	if err := state.Set(ctx, AllComponentsSynced.Name, allSynced, ""); err != nil {
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
