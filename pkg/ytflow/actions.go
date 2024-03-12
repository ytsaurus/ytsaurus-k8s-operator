package ytflow

//func checkFullUpdatePossibility(yc *components.YtsaurusClient, conds stateManager) actionStep {
//	preRun := func(ctx context.Context) (ActionPreRunStatus, error) {
//		possible, msg, err := yc.HandlePossibilityCheck(ctx)
//		if err != nil {
//			return ActionPreRunStatus{}, err
//		}
//		if !possible {
//			return ActionPreRunStatus{
//				ActionSubStatus: ActionBlocked,
//				Message:         msg,
//			}, nil
//		}
//		return ActionPreRunStatus{
//			ActionSubStatus: ActionNeedRun,
//			Message:         msg,
//		}, nil
//	}
//	return actionStep{
//		name:       CheckFullUpdatePossibilityStep,
//		preRunFunc: preRun,
//		conds:      conds,
//	}
//}
//
//func enableSafeMode(yc *components.YtsaurusClient, conds stateManager) actionStep {
//	return actionStep{
//		name:    EnableSafeModeStep,
//		runFunc: yc.EnableSafeMode,
//		conds:   conds,
//	}
//}
//
//func backupTabletCells(yc *components.YtsaurusClient, conds stateManager) actionStep {
//	preRun := func(ctx context.Context) (ActionPreRunStatus, error) {
//		cells, err := yc.GetTabletCells(ctx)
//		if err != nil {
//			return ActionPreRunStatus{}, err
//		}
//		err = conds.UpdateStatusRetryOnConflict(ctx, func(ytsaurusResource *ytv1.Ytsaurus) {
//			ytsaurusResource.Status.UpdateStatus.TabletCellBundles = cells
//		})
//		if err != nil {
//			return ActionPreRunStatus{}, err
//		}
//		return ActionPreRunStatus{
//			ActionSubStatus: ActionNeedRun,
//			Message:         "tablet cell bundles are stored in the resource state",
//		}, nil
//	}
//	run := yc.RemoveTabletCells
//	postRun := func(ctx context.Context) (ActionPostRunStatus, error) {
//		done, err := yc.AreTabletCellsRemoved(ctx)
//		if err != nil {
//			return ActionPostRunStatus{}, err
//		}
//		if done {
//			return ActionPostRunStatus{
//				ActionSubStatus: ActionDone,
//				Message:         "tablet cells were successfully removed",
//			}, nil
//		}
//		return ActionPostRunStatus{
//			ActionSubStatus: ActionUpdating,
//			Message:         "tablet cells not have been removed yet",
//		}, nil
//	}
//
//	return actionStep{
//		name:        BackupTabletCellsStep,
//		preRunFunc:  preRun,
//		runFunc:     run,
//		postRunFunc: postRun,
//		conds:       conds,
//	}
//}
//
//func buildMasterSnapshots(yc *components.YtsaurusClient, conds stateManager) actionStep {
//	preRun := func(ctx context.Context) (ActionPreRunStatus, error) {
//		paths, err := yc.GetMasterMonitoringPaths(ctx)
//		if err != nil {
//			return ActionPreRunStatus{}, err
//		}
//		err = conds.UpdateStatusRetryOnConflict(ctx, func(ytsaurusResource *ytv1.Ytsaurus) {
//			ytsaurusResource.Status.UpdateStatus.MasterMonitoringPaths = paths
//		})
//		if err != nil {
//			return ActionPreRunStatus{}, err
//		}
//		return ActionPreRunStatus{
//			ActionSubStatus: ActionNeedRun,
//			Message:         "master monitor paths were saved in state",
//		}, nil
//	}
//	run := yc.StartBuildMasterSnapshots
//	postRun := func(ctx context.Context) (ActionPostRunStatus, error) {
//		done, err := yc.AreMasterSnapshotsBuilt(ctx)
//		if err != nil {
//			return ActionPostRunStatus{}, err
//		}
//		if done {
//			return ActionPostRunStatus{
//				ActionSubStatus: ActionDone,
//				Message:         "master snapshots were successfully built",
//			}, nil
//		}
//		return ActionPostRunStatus{
//			ActionSubStatus: ActionUpdating,
//			Message:         "master snapshots haven't been not removed yet",
//		}, nil
//	}
//
//	return actionStep{
//		name:        BuildMasterSnapshotsStep,
//		preRunFunc:  preRun,
//		runFunc:     run,
//		postRunFunc: postRun,
//		conds:       conds,
//	}
//}

//func masterExitReadOnly(job *components.JobStateless) actionStep {
//	return newJobStep(
//		MasterExitReadOnlyStep,
//		job,
//		components.CreateExitReadOnlyScript(),
//	)
//}

//func recoverTabletCells(yc *components.YtsaurusClient, conds stateManager) actionStep {
//	return actionStep{
//		name:    RecoverTabletCellsStep,
//		runFunc: yc.RecoverTableCells,
//		conds:   conds,
//	}
//}
//
////
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

//func disableSafeMode(yc *components.YtsaurusClient, conds stateManager) actionStep {
//	return actionStep{
//		name:    DisableSafeModeStep,
//		runFunc: yc.DisableSafeMode,
//		conds:   conds,
//	}
//}
