package components

import (
	"context"
)

func (hp *HttpProxy) getFlow() Step {
	name := hp.GetName()
	buildStartedCond := buildStarted(name)
	builtFinishedCond := buildFinished(name)
	updateRequiredCond := updateRequired(name)
	rebuildStartedCond := rebuildStarted(name)
	rebuildFinishedCond := rebuildFinished(name)

	return StepComposite{
		Steps: []Step{
			StepRun{
				Name:               StepStartBuild,
				RunIfCondition:     not(buildStartedCond),
				RunFunc:            hp.doServerSync,
				OnSuccessCondition: buildStartedCond,
			},
			StepCheck{
				Name:               StepWaitBuildFinished,
				RunIfCondition:     not(builtFinishedCond),
				OnSuccessCondition: builtFinishedCond,
				RunFunc:            hp.serverInSync,
			},
			StepComposite{
				Name: StepUpdate,
				// Update should be run if either diff exists or updateRequired condition is set,
				// because a diff should disappear in the middle of the update, but it still need
				// to finish actions after the update.
				StatusConditionFunc: func(ctx context.Context) (SyncStatus, string, error) {
					inSync, err := hp.server.inSync(ctx)
					if err != nil {
						return "", "", err
					}
					if !inSync {
						if err = hp.condManager.SetCond(ctx, updateRequiredCond); err != nil {
							return "", "", err
						}
					}
					if !inSync || hp.condManager.IsSatisfied(updateRequiredCond) {
						return SyncStatusNeedSync, "", nil
					}
					return SyncStatusReady, "", nil
				},
				OnSuccessCondition: not(updateRequiredCond),
				Steps: []Step{
					StepRun{
						Name:               StepStartRebuild,
						RunIfCondition:     not(rebuildStartedCond),
						OnSuccessCondition: rebuildStartedCond,
						RunFunc:            hp.server.removePods,
					},
					StepCheck{
						Name:               StepWaitRebuildFinished,
						RunIfCondition:     not(rebuildFinishedCond),
						OnSuccessCondition: rebuildFinishedCond,
						RunFunc:            hp.server.inSync,
					},
				},
				OnSuccessFunc: func(ctx context.Context) error {
					return hp.condManager.SetCond(
						ctx,
						rebuildStarted(name),
					)
				},
			},
		},
	}

}
