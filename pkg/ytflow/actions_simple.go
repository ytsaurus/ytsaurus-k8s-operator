package ytflow

import (
	"context"
	"fmt"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
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
		return state.Set(ctx, IsFullUpdatePossibleCond, possible, msg)
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
		return state.SetTrue(ctx, IsFullUpdatePossibleCond, "")
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
}

func backupTabletCells(yc ytsaurusClient, state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		if state.IsFalse(IsTabletCellsRemovalStartedCond) {
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

			err = state.SetTrue(ctx, IsTabletCellsRemovalStartedCond, fmt.Sprintf("%d cell bundles stored", len(cells)))
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
			err = state.SetTrue(ctx, DoTabletCellsNeedRecoverCond, "")
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
		if state.IsFalse(IsMasterSnapshotBuildingStartedCond) {
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

			err = state.SetTrue(ctx, IsMasterSnapshotBuildingStartedCond, fmt.Sprintf("%d monitoring paths was stored", len(paths)))
			if err != nil {
				return err
			}
			return nil
		}

		// At this point master snapshots build was started.
		built, err := yc.AreMasterSnapshotsBuilt(ctx, state.GetMasterMonitoringPaths())
		if err != nil {
			return err
		}

		if built {
			if err = state.SetMasterMonitoringPaths(ctx, []string{}); err != nil {
				return err
			}
			if err = state.SetTrue(ctx, IsMasterInReadOnlyCond, ""); err != nil {
				return err
			}
		}

		return nil
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
}

//func masterExitReadOnly(job *components.JobStateless) actionStep {
//	return newJobStep(
//		MasterExitReadOnlyStep,
//		job,
//		components.CreateExitReadOnlyScript(),
//	)
//}

func masterExitReadOnly(state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		return state.SetFalse(ctx, IsMasterInReadOnlyCond, "")
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
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

		return state.SetFalse(ctx, DoTabletCellsNeedRecoverCond, "")
	}

	return simpleActionStep{
		runFunc: runFunc,
	}
}

func updateOpArchive(state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		return state.SetFalse(ctx, IsOperationArchiveNeedUpdateCond, "")
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
}

func initQueryTracker(state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		return state.SetFalse(ctx, IsQueryTrackerNeedInitCond, "")
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
}

//func updateOpArchive(job *components.JobStateless, scheduler *components.Scheduler) actionStep {
//	// this wrapper is lousy
//	jobStep := newJobStep(
//		UpdateOpArchiveStep,
//		job,
//		scheduler.GetUpdateOpArchiveScript(),
//	)
//	run := func(ctx context.Context) error {
//		job.SetInitScript(scheduler.GetUpdateOpArchiveScript())
//		batchJob := job.Build()
//		container := &batchJob.Spec.Template.Spec.Containers[0]
//		container.EnvFrom = []corev1.EnvFromSource{scheduler.GetSecretEnv()}
//		return job.Sync(ctx)
//	}
//	return actionStep{
//		name:        UpdateOpArchiveStep,
//		preRunFunc:  jobStep.preRunFunc,
//		runFunc:     run,
//		postRunFunc: jobStep.postRunFunc,
//	}
//}

//func initQueryTracker(job *components.JobStateless, queryTracker *components.QueryTracker) actionStep {
//	// this wrapper is lousy
//	jobStep := newJobStep(
//		InitQTStateStep,
//		job,
//		queryTracker.GetInitQueryTrackerJobScript(),
//	)
//	run := func(ctx context.Context) error {
//		job.SetInitScript(queryTracker.GetInitQueryTrackerJobScript())
//		batchJob := job.Build()
//		container := &batchJob.Spec.Template.Spec.Containers[0]
//		container.EnvFrom = []corev1.EnvFromSource{queryTracker.GetSecretEnv()}
//		return job.Sync(ctx)
//	}
//	return actionStep{
//		name:        InitQTStateStep,
//		preRunFunc:  jobStep.preRunFunc,
//		runFunc:     run,
//		postRunFunc: jobStep.postRunFunc,
//	}
//}

func disableSafeMode(yc ytsaurusClient, state stateManager) simpleActionStep {
	runFunc := func(ctx context.Context) error {
		err := yc.DisableSafeMode(ctx)
		if err != nil {
			return err
		}
		return state.SetFalse(ctx, IsFullUpdatePossibleCond, "")
	}
	return simpleActionStep{
		runFunc: runFunc,
	}
}
