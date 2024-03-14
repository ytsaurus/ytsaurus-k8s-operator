package ytflow

import (
	"context"
	"fmt"

	"github.com/ytsaurus/yt-k8s-operator/pkg/state"
)

// ConditionName aliased for brevity.
type ConditionName = state.ConditionName

// Special conditions, which are set automatically by flow code.
var (
	AllComponentsSynced = isTrue("AllComponentsSynced")
	MasterCanBeSynced   = isTrue("MasterCanBeSynced")
	NothingToDo         = isTrue("NothingToDo")
	FullUpdateNeeded    = isTrue("FullUpdateNeeded")
)

// Conditions which are set automatically based on components' statuses.
func isBuilt(compName ComponentName) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sBuilt", compName)))
}
func isReady(compName ComponentName) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sReady", compName)))
}
func needSync(compName ComponentName) Condition {
	return isTrue(ConditionName(fmt.Sprintf("%sNeedSync", compName)))
}

// Conditions which are manipulated by actions.
var (
	SafeModeEnabled            = isTrue("SafeModeEnabled")
	SafeModeCanBeEnabled       = isTrue("SafeModeCanBeEnabled")
	TabletCellsNeedRecover     = isTrue("TabletCellsNeedRecover")
	MasterIsInReadOnly         = isTrue("MasterIsInReadOnly")
	OperationArchiveNeedUpdate = isTrue("OperationArchiveNeedUpdate")
	QueryTrackerNeedsInit      = isTrue("QueryTrackerNeedsInit")

	// TabletCellsRemovalStarted is an intermediate condition of Tablet cell backup action
	TabletCellsRemovalStarted = isTrue("TabletCellsRemovalStarted")
	// MasterSnapshotBuildingStarted is an intermediate condition of Build master snapshots action.
	MasterSnapshotBuildingStarted = isTrue("MasterSnapshotBuildingStarted")
)

func updateConditions(ctx context.Context, statuses *statusRegistry, condDeps map[ConditionName][]Condition, state stateManager) error {
	var err error
	if err = updateSpecialConditions(ctx, state); err != nil {
		return fmt.Errorf("failed to update cluster based conditions: %w", err)
	}
	if err = updateComponentsBasedConditions(ctx, statuses, state); err != nil {
		return fmt.Errorf("failed to update components conditions: %w", err)
	}
	if err = updateDependenciesBasedConditions(ctx, condDeps, state); err != nil {
		return err
	}
	return nil
}
