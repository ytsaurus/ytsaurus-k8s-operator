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
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(s, s.server.Sync),
			getStandardWaitBuildFinishedStep(s, s.server.inSync),
			getStandardInitFinishedStep(s, func(ctx context.Context) (ok bool, err error) {
				s.initUser.SetInitScript(s.createInitUserScript())
				st, err := s.initUser.Sync(ctx, false)
				return st.SyncStatus == SyncStatusReady, err
			}),
			getStandardUpdateStep(
				s,
				s.condManager,
				s.server.inSync,
				[]Step{
					getStandardStartRebuildStep(s, s.server.removePods),
					getStandardWaiRebuildFinishedStep(s, s.server.inSync),
					StepRun{
						StepMeta: StepMeta{
							Name:               "StartPrepareUpdateOpArchive",
							RunIfCondition:     not(schedulerUpdateOpArchivePrepareStartedCond),
							OnSuccessCondition: schedulerUpdateOpArchivePrepareStartedCond,
						},
						Body: func(ctx context.Context) error {
							return s.initOpArchive.prepareRestart(ctx, false)
						},
					},
					StepCheck{
						StepMeta: StepMeta{
							Name:               "WaitUpdateOpArchivePrepared",
							RunIfCondition:     not(schedulerUpdateOpArchivePrepareFinishedCond),
							OnSuccessCondition: schedulerUpdateOpArchivePrepareFinishedCond,
							OnSuccessFunc: func(ctx context.Context) error {
								s.prepareInitOperationsArchive(s.initOpArchive)
								return nil
							},
						},
						Body: func(ctx context.Context) (bool, error) {
							return s.initOpArchive.isRestartPrepared(), nil
						},
					},
					StepCheck{
						StepMeta: StepMeta{
							Name:               "WaitUpdateOpArchive",
							RunIfCondition:     not(schedulerUpdateOpArchiveFinishedCond),
							OnSuccessCondition: schedulerUpdateOpArchiveFinishedCond,
						},
						Body: func(ctx context.Context) (ok bool, err error) {
							st, err := s.initOpArchive.Sync(ctx, false)
							return st.SyncStatus == SyncStatusReady, err
						},
					},
				},
			),
		},
	}
}
