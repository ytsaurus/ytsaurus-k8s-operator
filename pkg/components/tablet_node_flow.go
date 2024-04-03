package components

import (
	"context"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

var (
	tnTabletCellsBackupStartedCond  = isTrue("TabletCellsBackupStarted")
	tnTabletCellsBackupFinishedCond = isTrue("TabletCellsBackupFinished")
	tnTabletCellsRecoveredCond      = isTrue("TabletCellsRecovered")
)

func (tn *TabletNode) getFlow() Step {
	name := tn.GetName()
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
				RunFunc:            tn.server.Sync,
				OnSuccessCondition: buildStartedCond,
			},
			StepCheck{
				Name:               StepWaitBuildFinished,
				RunIfCondition:     not(builtFinishedCond),
				OnSuccessCondition: builtFinishedCond,
				RunFunc:            tn.server.inSync,
			},
			StepRun{
				Name:               StepInitFinished,
				RunIfCondition:     not(initCond),
				OnSuccessCondition: initCond,
				RunFunc: func(ctx context.Context) error {
					if !tn.doInitialization {
						return nil
					}
					return tn.initializeBundles(ctx)
				},
			},
			StepComposite{
				Name: StepUpdate,
				// Update should be run if either diff exists or updateRequired condition is set,
				// because a diff should disappear in the middle of the update, but it still need
				// to finish actions after the update (master exit read only, safe mode, etc.).
				StatusConditionFunc: func(ctx context.Context) (SyncStatus, string, error) {
					inSync, err := tn.server.inSync(ctx)
					if err != nil {
						return "", "", err
					}
					if !inSync {
						if err = tn.condManager.SetCond(ctx, updateRequiredCond); err != nil {
							return "", "", err
						}
					}
					if !inSync || tn.condManager.IsSatisfied(updateRequiredCond) {
						return SyncStatusNeedSync, "", nil
					}
					return SyncStatusReady, "", nil
				},
				OnSuccessCondition: not(updateRequiredCond),
				OnSuccessFunc: func(ctx context.Context) error {
					return tn.condManager.SetCondMany(ctx,
						rebuildStarted(name),
						rebuildFinished(name),
						tnTabletCellsBackupStartedCond,
						tnTabletCellsBackupFinishedCond,
						tnTabletCellsRecoveredCond,
					)
				},
				Steps: []Step{
					StepRun{
						Name:               "SaveTabletCellBundles",
						RunIfCondition:     not(tnTabletCellsBackupStartedCond),
						OnSuccessCondition: tnTabletCellsBackupStartedCond,
						RunFunc: func(ctx context.Context) error {
							bundles, err := tn.ytsaurusClient.GetTabletCells(ctx)
							if err != nil {
								return err
							}
							if err = tn.storeTabletCellBundles(ctx, bundles); err != nil {
								return err
							}
							return tn.ytsaurusClient.RemoveTabletCells(ctx)
						},
					},
					StepCheck{
						Name:               "CheckTabletCellsRemoved",
						RunIfCondition:     not(tnTabletCellsBackupFinishedCond),
						OnSuccessCondition: tnTabletCellsBackupFinishedCond,
						RunFunc: func(ctx context.Context) (ok bool, err error) {
							return tn.ytsaurusClient.AreTabletCellsRemoved(ctx)
						},
					},
					StepRun{
						Name:               StepStartRebuild,
						RunIfCondition:     not(rebuildStartedCond),
						OnSuccessCondition: rebuildStartedCond,
						RunFunc:            tn.server.removePods,
					},
					StepCheck{
						Name:               StepWaitRebuildFinished,
						RunIfCondition:     not(rebuildFinishedCond),
						OnSuccessCondition: rebuildFinishedCond,
						RunFunc:            tn.server.inSync,
					},
					StepRun{
						Name:               "RecoverTableCells",
						RunIfCondition:     not(tnTabletCellsRecoveredCond),
						OnSuccessCondition: tnTabletCellsRecoveredCond,
						RunFunc: func(ctx context.Context) error {
							bundles := tn.getStoredTabletCellBundles()
							return tn.ytsaurusClient.RecoverTableCells(ctx, bundles)
						},
					},
				},
			},
		},
	}
}

func (tn *TabletNode) storeTabletCellBundles(ctx context.Context, bundles []ytv1.TabletCellBundleInfo) error {
	return tn.stateManager.SetTabletCellBundles(ctx, bundles)
}

func (tn *TabletNode) getStoredTabletCellBundles() []ytv1.TabletCellBundleInfo {
	return tn.stateManager.GetTabletCellBundles()
}
