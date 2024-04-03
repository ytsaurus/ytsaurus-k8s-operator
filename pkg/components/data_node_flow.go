package components

import (
	"context"
)

func (n *DataNode) getFlow() Step {
	name := n.GetName()
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
				RunFunc:            n.server.Sync,
				OnSuccessCondition: buildStartedCond,
			},
			StepCheck{
				Name:               StepWaitBuildFinished,
				RunIfCondition:     not(builtFinishedCond),
				OnSuccessCondition: builtFinishedCond,
				RunFunc:            n.server.inSync,
			},
			StepComposite{
				Name: StepUpdate,
				// Update should be run if either diff exists or updateRequired condition is set,
				// because a diff should disappear in the middle of the update, but it still need
				// to finish actions after the update (master exit read only, safe mode, etc.).
				StatusConditionFunc: func(ctx context.Context) (SyncStatus, string, error) {
					inSync, err := n.server.inSync(ctx)
					if err != nil {
						return "", "", err
					}
					if !inSync {
						if err = n.condManager.SetCond(ctx, updateRequiredCond); err != nil {
							return "", "", err
						}
					}
					if !inSync || n.condManager.IsSatisfied(updateRequiredCond) {
						return SyncStatusNeedSync, "", nil
					}
					return SyncStatusReady, "", nil
				},
				OnSuccessCondition: not(updateRequiredCond),
				OnSuccessFunc: func(ctx context.Context) error {
					return n.condManager.SetCondMany(ctx,
						rebuildStarted(name),
						rebuildFinished(name),
						tnTabletCellsBackupStartedCond,
						tnTabletCellsBackupFinishedCond,
						tnTabletCellsRecoveredCond,
					)
				},
				Steps: []Step{
					StepRun{
						Name:               StepStartRebuild,
						RunIfCondition:     not(rebuildStartedCond),
						OnSuccessCondition: rebuildStartedCond,
						RunFunc:            n.server.removePods,
					},
					StepCheck{
						Name:               StepWaitRebuildFinished,
						RunIfCondition:     not(rebuildFinishedCond),
						OnSuccessCondition: rebuildFinishedCond,
						RunFunc:            n.server.inSync,
					},
				},
			},
		},
	}
}
