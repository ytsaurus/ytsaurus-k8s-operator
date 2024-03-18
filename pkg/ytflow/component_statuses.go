package ytflow

import (
	"context"
	"fmt"
	"strings"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

func interpretSyncStatus(st syncStatus) (isBuilt bool, needSync bool) {
	switch st {
	case components.SyncStatusReady:
		return true, false
	case components.SyncStatusNeedLocalUpdate:
		return true, true
	default:
		// Pending (from scratch or smth)
		// Updating (for example wait pods to be deleted)
		// Blocked (for example wait pods to be created)
		return false, true
	}
}

func updateComponentsBasedConditions(ctx context.Context, statuses *statusRegistry, state stateManager) error {
	allSynced := true
	var becameSynced []ComponentName

	_, masterNeedsSync := interpretSyncStatus(statuses.statuses[MasterName].SyncStatus)

	// Actualize `built` and `needSync` conditions.
	for compName, status := range statuses.statuses {
		neededSyncBefore := state.Get(needSync(compName).Name)
		compIsBuilt, compNeedsSync := interpretSyncStatus(status.SyncStatus)

		if masterNeedsSync {
			// Current behaviour is we sync all the components in case of masterComponent update.
			// WE may want to change this in the future, but after all diffs in spec would be considered by the operator.
			compNeedsSync = true
		}

		msg := fmt.Sprintf("%s: %s", status.SyncStatus, status.Message)
		if err := state.Set(ctx, isBuilt(compName).Name, compIsBuilt, msg); err != nil {
			return err
		}
		if err := state.Set(ctx, needSync(compName).Name, compNeedsSync, msg); err != nil {
			return err
		}

		if neededSyncBefore && !compNeedsSync {
			becameSynced = append(becameSynced, compName)
		}

		if compNeedsSync {
			allSynced = false
		}
	}

	for _, compName := range becameSynced {
		if compName == SchedulerName {
			if err := state.SetTrue(ctx, OperationArchiveNeedUpdate.Name, "scheduler have became synced"); err != nil {
				return err
			}
		}
		if compName == QueryTrackerName {
			if err := state.SetTrue(ctx, QueryTrackerNeedsInit.Name, "query tracker have became synced"); err != nil {
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
	masterNeedSync := state.Get(needSync(MasterName).Name)
	isInSafeMode := state.Get(SafeModeEnabled.Name)

	// This could be improved by implementing OR for conditions deps.
	// Since we only have masterComponent now it may not being worth it.
	masterCanBeSynced := clusterCreated || isInReadOnly
	var msgs []string
	if clusterCreated {
		msgs = append(msgs, "cluster just created")
	} else {
		msgs = append(msgs, "cluster is not just created")
	}
	if isInReadOnly {
		msgs = append(msgs, "master is in read only")
	} else {
		msgs = append(msgs, "master is not in read only")
	}
	err := state.Set(ctx,
		MasterCanBeSynced.Name, masterCanBeSynced,
		strings.Join(msgs, "; "),
	)
	if err != nil {
		return err
	}

	// Other components can be synced either after the masterComponent can be updated in full update case
	// otherwise when needed.
	compsCanBeSynced := (masterNeedSync && masterCanBeSynced) || !masterNeedSync
	msgs = []string{}
	if masterNeedSync {
		msgs = append(msgs, "master needs sync")
	} else {
		msgs = append(msgs, "master doesn't need sync")
	}
	if masterCanBeSynced {
		msgs = append(msgs, "master can be synced")
	} else {
		msgs = append(msgs, "master cant' be synced")
	}
	err = state.Set(ctx,
		ComponentsCanBeSynced.Name, compsCanBeSynced,
		strings.Join(msgs, "; "),
	)
	if err != nil {
		return err
	}

	// TODO: i guess they should depend on tablet nodes readiness also actually
	tabletCellsNeedRecover := state.Get(TabletCellsNeedRecover.Name)
	fullUpdateMode := isInSafeMode || masterNeedSync
	tabletCellsReady := fullUpdateMode && !tabletCellsNeedRecover || !fullUpdateMode
	return state.Set(ctx, TabletCellsReady.Name, tabletCellsReady, "")
}
