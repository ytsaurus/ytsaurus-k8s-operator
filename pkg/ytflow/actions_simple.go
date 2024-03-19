package ytflow

import (
	"context"
	"fmt"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type ytsaurusClient interface {
	component
	HandlePossibilityCheck(context.Context) (bool, string, error)
	EnableSafeMode(context.Context) error
	DisableSafeMode(context.Context) error

	GetTabletCells(context.Context) ([]ytv1.TabletCellBundleInfo, error)
	RemoveTabletCells(context.Context) error
	RecoverTableCells(context.Context, []ytv1.TabletCellBundleInfo) error
	AreTabletCellsRemoved(context.Context) (bool, error)

	GetMasterMonitoringPaths(context.Context) ([]string, error)
	StartBuildMasterSnapshots(context.Context, []string) error
	AreMasterSnapshotsBuilt(context.Context, []string) (bool, error)
}

type masterComponent interface {
	GetMasterExitReadOnlyJob() *components.InitJob
}

type schedulerComponent interface {
	GetUpdateOpArchiveJob() *components.InitJob
	PrepareInitOperationArchive(job *components.InitJob)
}

type queryTrackerComponent interface {
	GetInitQueryTrackerJob() *components.InitJob
	PrepareInitQueryTrackerState(job *components.InitJob)
}

type simpleActionStep struct {
	runFunc func(ctx context.Context) error
}

func (a simpleActionStep) Run(ctx context.Context) error {
	return a.runFunc(ctx)
}

// TODO: this way it is really hard to signal blocking or updating messages.
func checkFullUpdatePossibility(yc ytsaurusClient, state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		possible, msg, err := yc.HandlePossibilityCheck(ctx)
		if err != nil {
			return err
		}
		return state.Set(ctx, SafeModeCanBeEnabled.Name, possible, msg)
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
}

func enableSafeMode(yc ytsaurusClient, state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		err := yc.EnableSafeMode(ctx)
		if err != nil {
			return err
		}
		if err = state.SetTrue(ctx, SafeModeEnabled.Name, ""); err != nil {
			return err
		}
		return state.SetFalse(ctx, SafeModeCanBeEnabled.Name, "disabled after safe mode was enabled")
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
}

func backupTabletCells(yc ytsaurusClient, state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		actionStartedCond := actionStarted("backupTabletCells")

		if state.IsFalse(actionStartedCond.Name) {
			cells, err := yc.GetTabletCells(ctx)
			if err != nil {
				return err
			}

			// TODO: if table cells are not empty then it could be a sign that last time error is occured
			// we can override data with empty values.
			err = state.SetTabletCellBundles(ctx, cells)
			if err != nil {
				return err
			}

			if err = yc.RemoveTabletCells(ctx); err != nil {
				return err
			}

			err = state.SetTrue(ctx, actionStartedCond.Name, fmt.Sprintf("%d cell bundles stored", len(cells)))
			if err != nil {
				return err
			}
			return nil
		}

		// At this point tablet cells are backed up in the status and removal is started.

		removed, err := yc.AreTabletCellsRemoved(ctx)
		if err != nil {
			return err
		}

		if removed {
			err = state.SetTrue(ctx, TabletCellsNeedRecover.Name, "")
			if err != nil {
				return err
			}
		}

		return nil
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
}

func buildMasterSnapshots(yc ytsaurusClient, state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		actionStartedCond := actionStarted("buildMasterSnapshots")

		if state.IsFalse(actionStartedCond.Name) {
			paths, err := yc.GetMasterMonitoringPaths(ctx)
			if err != nil {
				return err
			}
			if err = state.SetMasterMonitoringPaths(ctx, paths); err != nil {
				return err
			}

			if err = yc.StartBuildMasterSnapshots(ctx, paths); err != nil {
				return err
			}

			err = state.SetTrue(ctx, actionStartedCond.Name, fmt.Sprintf("%d monitoring paths was stored", len(paths)))
			if err != nil {
				return err
			}
			return nil
		}

		// At this point masterComponent snapshots build was started.
		built, err := yc.AreMasterSnapshotsBuilt(ctx, state.GetMasterMonitoringPaths())
		if err != nil {
			return err
		}

		if built {
			if err = state.SetMasterMonitoringPaths(ctx, []string{}); err != nil {
				return err
			}
			if err = state.SetTrue(ctx, MasterIsInReadOnly.Name, ""); err != nil {
				return err
			}
		}

		return nil
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
}

func masterExitReadOnly(master masterComponent, state stateManager) simpleActionStep {
	return getJobStep(
		master.GetMasterExitReadOnlyJob(),
		func(initJob *components.InitJob) {
			initJob.SetInitScript(components.CreateExitReadOnlyScript())
		},
		not(MasterIsInReadOnly),
		state,
	)
}

// TODO: rollback if failure happens between state saving and recover or smth
func recoverTabletCells(yc ytsaurusClient, state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		if err := yc.RecoverTableCells(ctx, state.GetTabletCellBundles()); err != nil {
			return err
		}

		err := state.SetTabletCellBundles(ctx, []ytv1.TabletCellBundleInfo{})
		if err != nil {
			return err
		}

		return state.SetFalse(ctx, TabletCellsNeedRecover.Name, "")
	}

	return simpleActionStep{
		runFunc: runFunc,
	}
}

// TODO: maybe wait for tablet specifically to run update operations
// TODO: fake job if archive can't be built because of tablet nodes
func updateOpArchive(sch schedulerComponent, state stateManager) simpleActionStep {
	return getJobStep(
		sch.GetUpdateOpArchiveJob(),
		sch.PrepareInitOperationArchive,
		not(OperationArchiveNeedUpdate),
		state,
	)
}

func initQueryTracker(qt queryTrackerComponent, state stateManager) simpleActionStep {
	return getJobStep(
		qt.GetInitQueryTrackerJob(),
		qt.PrepareInitQueryTrackerState,
		not(QueryTrackerNeedsInit),
		state,
	)
}

func getJobStep(job *components.InitJob, preRun func(initJob *components.InitJob), successCondition Condition, state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		if err := job.Fetch(ctx); err != nil {
			return err
		}

		startedCondName := actionStarted(job.GetName()).Name
		if !state.Get(startedCondName) {
			if err := job.PrepareRestart(ctx, false); err != nil {
				return err
			}
			return state.SetTrue(ctx, startedCondName, "")
		}

		preRun(job)
		status, err := job.Sync(ctx, false)
		if err != nil {
			return err
		}
		if status.SyncStatus != components.SyncStatusReady {
			return nil
		}
		return state.Set(ctx, successCondition.Name, successCondition.Val, "")
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
}

func disableSafeMode(yc ytsaurusClient, state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		err := yc.DisableSafeMode(ctx)
		if err != nil {
			return err
		}
		return state.SetFalse(ctx, SafeModeEnabled.Name, "")
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
}
