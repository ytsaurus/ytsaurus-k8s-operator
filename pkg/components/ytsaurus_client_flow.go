package components

import (
	"context"
)

func (yc *YtsaurusClient) getFlow() Step {
	name := yc.GetName()
	buildStartedCond := buildStarted(name)
	builtFinishedCond := buildFinished(name)
	updateRequiredCond := updateRequired(name)
	rebuildStartedCond := rebuildStarted(name)
	rebuildFinishedCond := rebuildFinished(name)
	initStartedCond := initializationStarted(name)
	initFinishedCond := initializationFinished(name)

	return StepComposite{
		Steps: []Step{
			StepRun{
				Name:               StepStartBuild,
				RunIfCondition:     not(buildStartedCond),
				RunFunc:            yc.doKubeSync,
				OnSuccessCondition: buildStartedCond,
			},
			StepCheck{
				Name:               StepWaitBuildFinished,
				RunIfCondition:     not(builtFinishedCond),
				OnSuccessCondition: builtFinishedCond,
				RunFunc:            yc.isInSync,
			},
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
			StepComposite{
				Name: StepUpdate,
				// Update should be run if either diff exists or updateRequired condition is set,
				// because a diff should disappear in the middle of the update, but it still need
				// to finish actions after the update.
				StatusConditionFunc: func(ctx context.Context) (SyncStatus, string, error) {
					inSync, err := yc.isInSync(ctx)
					if err != nil {
						return "", "", err
					}
					if !inSync || yc.condManager.IsSatisfied(updateRequiredCond) {
						return SyncStatusNeedSync, "", nil
					}
					return SyncStatusReady, "", nil
				},
				OnSuccessCondition: not(updateRequiredCond),
				Steps: []Step{
					StepRun{
						Name:               StepCheckUpdateRequired,
						RunIfCondition:     not(updateRequiredCond),
						OnSuccessCondition: updateRequiredCond,
						// If update started — setting updateRequired unconditionally.
					},
					StepRun{
						Name:               StepStartRebuild,
						RunIfCondition:     not(rebuildStartedCond),
						OnSuccessCondition: rebuildStartedCond,
						RunFunc:            yc.doKubeSync,
					},
					StepCheck{
						Name:               StepWaitRebuildFinished,
						RunIfCondition:     not(rebuildFinishedCond),
						OnSuccessCondition: rebuildFinishedCond,
						RunFunc:            yc.isInSync,
					},
				},
				OnSuccessFunc: func(ctx context.Context) error {
					return yc.condManager.SetCond(
						ctx,
						rebuildStarted(name),
					)
				},
			},
		},
	}

}
