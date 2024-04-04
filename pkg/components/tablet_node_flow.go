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
	initFinishedCond := initializationFinished(name)
	return StepComposite{
		Steps: []Step{
			getStandardStartBuildStep(tn, tn.server.Sync),
			getStandardWaitBuildFinishedStep(tn, tn.server.inSync),
			StepRun{
				Name:               StepInitFinished,
				RunIfCondition:     not(initFinishedCond),
				OnSuccessCondition: initFinishedCond,
				RunFunc: func(ctx context.Context) error {
					if !tn.doInitialization {
						return nil
					}
					return tn.initializeBundles(ctx)
				},
			},
			getStandardUpdateStep(
				tn,
				tn.condManager,
				tn.server.inSync,
				[]Step{
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
					getStandardStartRebuildStep(tn, tn.server.removePods),
					getStandardWaiRebuildFinishedStep(tn, tn.server.inSync),
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
			),
		},
	}
}

func (tn *TabletNode) storeTabletCellBundles(ctx context.Context, bundles []ytv1.TabletCellBundleInfo) error {
	return tn.stateManager.SetTabletCellBundles(ctx, bundles)
}

func (tn *TabletNode) getStoredTabletCellBundles() []ytv1.TabletCellBundleInfo {
	return tn.stateManager.GetTabletCellBundles()
}
