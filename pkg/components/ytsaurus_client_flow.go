package components

import (
	"context"
)

func (yc *YtsaurusClient) getFlow() Step {
	name := yc.GetName()
	initStartedCond := initializationStarted(name)
	initFinishedCond := initializationFinished(name)

	return StepComposite{
		Steps: []Step{
			getStandardStartBuildStep(yc, yc.doKubeSync),
			getStandardWaitBuildFinishedStep(yc, yc.isInSync),
			StepRun{
				Name: StepInitStarted,
				StatusFunc: func(ctx context.Context) (st SyncStatus, msg string, err error) {
					if yc.ytClient != nil {
						return SyncStatusReady, "", nil
					}
					return SyncStatusNeedSync, "yt client needs sync", nil
				},
				RunIfCondition:     not(initStartedCond),
				OnSuccessCondition: initStartedCond,
				RunFunc:            yc.doInit,
			},
			StepCheck{
				Name:               StepInitFinished,
				RunIfCondition:     not(initFinishedCond),
				OnSuccessCondition: initFinishedCond,
				RunFunc: func(ctx context.Context) (ok bool, err error) {
					yc.initUserJob.SetInitScript(yc.createInitUserScript())
					st, err := yc.initUserJob.Sync(ctx, false)
					return st.SyncStatus == SyncStatusReady, err
				},
			},
			getStandardUpdateStep(
				yc,
				yc.condManager,
				yc.isInSync,
				[]Step{
					getStandardStartRebuildStep(yc, yc.doKubeSync),
					getStandardWaiRebuildFinishedStep(yc, yc.isInSync),
				},
			),
		},
	}
}
