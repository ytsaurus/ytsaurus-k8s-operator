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
		Body: []Step{
			getStandardStartBuildStep(tn, tn.server.Sync),
			getStandardWaitBuildFinishedStep(tn, tn.server.inSync),
			StepRun{
				StepMeta: StepMeta{
					Name:               StepInitFinished,
					RunIfCondition:     not(initFinishedCond),
					OnSuccessCondition: initFinishedCond,
				},
				Body: func(ctx context.Context) error {
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
						StepMeta: StepMeta{
							Name:               "SaveTabletCellBundles",
							RunIfCondition:     not(tnTabletCellsBackupStartedCond),
							OnSuccessCondition: tnTabletCellsBackupStartedCond,
						},
						Body: func(ctx context.Context) error {
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
						StepMeta: StepMeta{
							Name:               "CheckTabletCellsRemoved",
							RunIfCondition:     not(tnTabletCellsBackupFinishedCond),
							OnSuccessCondition: tnTabletCellsBackupFinishedCond,
						},
						Body: func(ctx context.Context) (ok bool, err error) {
							return tn.ytsaurusClient.AreTabletCellsRemoved(ctx)
						},
					},
					getStandardStartRebuildStep(tn, tn.server.removePods),
					getStandardWaitPodsRemovedStep(tn, tn.server.arePodsRemoved),
					getStandardPodsCreateStep(tn, tn.server.Sync),
					getStandardWaiRebuildFinishedStep(tn, tn.server.inSync),
					StepRun{
						StepMeta: StepMeta{
							Name:               "RecoverTableCells",
							RunIfCondition:     not(tnTabletCellsRecoveredCond),
							OnSuccessCondition: tnTabletCellsRecoveredCond,
						},
						Body: func(ctx context.Context) error {
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
