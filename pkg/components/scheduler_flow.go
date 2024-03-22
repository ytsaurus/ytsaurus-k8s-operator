package components

import (
	"context"
)

var (
	schedulerUpdateOpArchivePrepareStartedCond  = isTrue("UpdateOpArchivePrepareStarted")
	schedulerUpdateOpArchivePrepareFinishedCond = isTrue("UpdateOpArchivePrepareFinished")
	schedulerUpdateOpArchiveFinishedCond        = isTrue("UpdateOpArchiveFinished")
)

func (s *Scheduler) getFlow() Step {
	name := s.GetName()
	buildStartedCond := buildStarted(name)
	builtFinishedCond := buildFinished(name)
	initCond := initializationFinished(name)
	updateRequiredCond := updateRequired(name)
	rebuildStartedCond := rebuildStarted(name)
	rebuildFinishedCond := rebuildFinished(name)

	return StepComposite{
		Steps: []Step{
			StepRun{
				Name:               StepStartBuild,
				RunIfCondition:     not(buildStartedCond),
				RunFunc:            s.server.Sync,
				OnSuccessCondition: buildStartedCond,
			},
			StepCheck{
				Name:               StepWaitBuildFinished,
				RunIfCondition:     not(builtFinishedCond),
				OnSuccessCondition: builtFinishedCond,
				RunFunc:            s.server.inSync,
			},
			StepCheck{
				Name:               StepInitFinished,
				RunIfCondition:     not(initCond),
				OnSuccessCondition: initCond,
				RunFunc: func(ctx context.Context) (ok bool, err error) {
					s.initUser.SetInitScript(s.createInitUserScript())
					st, err := s.initUser.Sync(ctx, false)
					return st.SyncStatus == SyncStatusReady, err
				},
			},
			StepComposite{
				Name: StepUpdate,
				// Update should be run if either diff exists or updateRequired condition is set,
				// because a diff should disappear in the middle of the update, but it still need
				// to finish actions after the update (master exit read only, safe mode, etc.).
				StatusConditionFunc: func(ctx context.Context) (SyncStatus, string, error) {
					inSync, err := s.server.inSync(ctx)
					if err != nil {
						return "", "", err
					}
					if !inSync {
						if err = s.condManager.SetCond(ctx, updateRequiredCond); err != nil {
							return "", "", err
						}
					}
					if !inSync || s.condManager.IsSatisfied(updateRequiredCond) {
						return SyncStatusNeedSync, "", nil
					}
					return SyncStatusReady, "", nil
				},
				OnSuccessCondition: not(updateRequiredCond),
				OnSuccessFunc: func(ctx context.Context) error {
					return s.condManager.SetCondMany(ctx,
						not(rebuildStarted(name)),
						not(rebuildFinished(name)),
						not(schedulerUpdateOpArchivePrepareStartedCond),
						not(schedulerUpdateOpArchivePrepareFinishedCond),
						not(schedulerUpdateOpArchiveFinishedCond),
					)
				},
				Steps: []Step{
					StepRun{
						Name:               StepStartRebuild,
						RunIfCondition:     not(rebuildStartedCond),
						OnSuccessCondition: rebuildStartedCond,
						RunFunc:            s.server.removePods,
					},
					StepCheck{
						Name:               StepWaitRebuildFinished,
						RunIfCondition:     not(rebuildFinishedCond),
						OnSuccessCondition: rebuildFinishedCond,
						RunFunc:            s.server.inSync,
					},
					StepRun{
						Name:               "StartPrepareUpdateOpArchive",
						RunIfCondition:     not(schedulerUpdateOpArchivePrepareStartedCond),
						OnSuccessCondition: schedulerUpdateOpArchivePrepareStartedCond,
						RunFunc: func(ctx context.Context) error {
							return s.initOpArchive.prepareRestart(ctx, false)
						},
					},
					StepCheck{
						Name:               "WaitUpdateOpArchivePrepared",
						RunIfCondition:     not(schedulerUpdateOpArchivePrepareFinishedCond),
						OnSuccessCondition: schedulerUpdateOpArchivePrepareFinishedCond,
						RunFunc: func(ctx context.Context) (bool, error) {
							return s.initOpArchive.isRestartPrepared(), nil
						},
						OnSuccessFunc: func(ctx context.Context) error {
							s.prepareInitOperationArchive(s.initOpArchive)
							return nil
						},
					},
					StepCheck{
						Name:               "WaitUpdateOpArchive",
						RunIfCondition:     not(schedulerUpdateOpArchiveFinishedCond),
						OnSuccessCondition: schedulerUpdateOpArchiveFinishedCond,
						RunFunc: func(ctx context.Context) (ok bool, err error) {
							st, err := s.initOpArchive.Sync(ctx, false)
							return st.SyncStatus == SyncStatusReady, err
						},
					},
				},
			},
		},
	}
}
