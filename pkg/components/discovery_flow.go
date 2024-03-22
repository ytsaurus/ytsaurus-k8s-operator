package components

import (
	"context"
)

func (d *Discovery) getFlow() Step {
	name := d.GetName()
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
				RunFunc:            d.server.Sync,
				OnSuccessCondition: buildStartedCond,
			},
			StepCheck{
				Name:               StepWaitBuildFinished,
				RunIfCondition:     not(builtFinishedCond),
				OnSuccessCondition: builtFinishedCond,
				RunFunc:            d.server.inSync,
			},
			StepComposite{
				Name: StepUpdate,
				// Update should be run if either diff exists or updateRequired condition is set,
				// because a diff should disappear in the middle of the update, but it still need
				// to finish actions after the update.
				StatusConditionFunc: func(ctx context.Context) (SyncStatus, string, error) {
					inSync, err := d.server.inSync(ctx)
					if err != nil {
						return "", "", err
					}
					if !inSync {
						if err = d.condManager.SetCond(ctx, updateRequiredCond); err != nil {
							return "", "", err
						}
					}
					if !inSync || d.condManager.IsSatisfied(updateRequiredCond) {
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
						RunFunc:            d.server.removePods,
					},
					StepCheck{
						Name:               StepWaitRebuildFinished,
						RunIfCondition:     not(rebuildFinishedCond),
						OnSuccessCondition: rebuildFinishedCond,
						RunFunc:            d.server.inSync,
					},
				},
				OnSuccessFunc: func(ctx context.Context) error {
					return d.condManager.SetCond(
						ctx,
						rebuildStarted(name),
					)
				},
			},
		},
	}

}
