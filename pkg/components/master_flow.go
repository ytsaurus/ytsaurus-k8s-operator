package components

import (
	"context"
)

var (
	masterUpdatePossibleCond              = isTrue("MasterUpdatePossible")
	masterSafeModeEnabledCond             = isTrue("MasterSafeModeEnabled")
	masterSnapshotsBuildStartedCond       = isTrue("MasterSnapshotsBuildStarted")
	masterSnapshotsBuildFinishedCond      = isTrue("MasterSnapshotsBuildFinished")
	masterExitReadOnlyPrepareStartedCond  = isTrue("MasterExitReadOnlyPrepareStarted")
	masterExitReadOnlyPrepareFinishedCond = isTrue("MasterExitReadOnlyPrepareFinished")
	masterExitReadOnlyFinished            = isTrue("MasterExitReadOnlyFinished")
	masterSafeModeDisabledCond            = isTrue("MasterSafeModeDisabled")
)

func (m *Master) getBuildFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(m, m.doServerSync),
			getStandardWaitBuildFinishedStep(m, m.server.inSync),
			getStandardInitFinishedStep(m, func(ctx context.Context) (ok bool, err error) {
				m.initJob.SetInitScript(m.createInitScript())
				st, err := m.initJob.Sync(ctx, false)
				return st.SyncStatus == SyncStatusReady, err
			}),
		},
	}
}

func (m *Master) getUpdateFlow() Step {
	return getStandardUpdateStep(
		m,
		m.condManager,
		m.server.inSync,
		[]Step{
			StepCheck{
				StepMeta: StepMeta{
					Name: "UpdatePossibleCheck",
					StatusFunc: func(ctx context.Context) (SyncStatus, string, error) {
						if m.condManager.IsSatisfied(masterUpdatePossibleCond) {
							return SyncStatusReady, "", nil
						}
						possible, msg, err := m.ytClient.HandlePossibilityCheck(ctx)
						// N.B.: here we return NeedSync (not Ready), so empty body of step become executed
						// and masterUpdatePossibleCond became set.
						st := SyncStatusNeedSync
						if !possible {
							st = SyncStatusBlocked
						}
						return st, msg, err
					},
					OnSuccessCondition: masterUpdatePossibleCond,
				},
			},
			StepRun{
				StepMeta: StepMeta{
					Name:               "EnableSafeMode",
					RunIfCondition:     not(masterSafeModeEnabledCond),
					OnSuccessCondition: masterSafeModeEnabledCond,
				},
				Body: m.ytClient.EnableSafeMode,
			},
			StepRun{
				StepMeta: StepMeta{
					Name:               "BuildSnapshots",
					RunIfCondition:     not(masterSnapshotsBuildStartedCond),
					OnSuccessCondition: masterSnapshotsBuildStartedCond,
				},
				Body: func(ctx context.Context) error {
					monitoringPaths, err := m.ytClient.GetMasterMonitoringPaths(ctx)
					if err != nil {
						return err
					}
					if err = m.storeMasterMonitoringPaths(ctx, monitoringPaths); err != nil {
						return err
					}
					return m.ytClient.StartBuildMasterSnapshots(ctx, monitoringPaths)
				},
			},
			StepCheck{
				StepMeta: StepMeta{
					Name:               "CheckSnapshotsBuild",
					RunIfCondition:     not(masterSnapshotsBuildFinishedCond),
					OnSuccessCondition: masterSnapshotsBuildFinishedCond,
				},
				Body: func(ctx context.Context) (ok bool, err error) {
					paths := m.getStoredMasterMonitoringPaths()
					return m.ytClient.AreMasterSnapshotsBuilt(ctx, paths)
				},
			},
			getStandardStartRebuildStep(m, m.server.removePods),
			getStandardWaitPodsRemovedStep(m, m.server.arePodsRemoved),
			getStandardPodsCreateStep(m, m.doServerSync),
			getStandardWaiRebuildFinishedStep(m, m.server.inSync),
			StepRun{
				StepMeta: StepMeta{
					Name:               "StartPrepareMasterExitReadOnly",
					RunIfCondition:     not(masterExitReadOnlyPrepareStartedCond),
					OnSuccessCondition: masterExitReadOnlyPrepareStartedCond,
				},
				Body: func(ctx context.Context) error {
					return m.exitReadOnlyJob.prepareRestart(ctx, false)
				},
			},
			StepCheck{
				StepMeta: StepMeta{
					Name:               "WaitMasterExitReadOnlyPrepared",
					RunIfCondition:     not(masterExitReadOnlyPrepareFinishedCond),
					OnSuccessCondition: masterExitReadOnlyPrepareFinishedCond,
				},
				Body: func(ctx context.Context) (bool, error) {
					return m.exitReadOnlyJob.isRestartPrepared(), nil
				},
			},
			StepCheck{
				StepMeta: StepMeta{
					Name:               "WaitMasterExitsReadOnly",
					RunIfCondition:     not(masterExitReadOnlyFinished),
					OnSuccessCondition: masterExitReadOnlyFinished,
				},
				Body: func(ctx context.Context) (ok bool, err error) {
					m.exitReadOnlyJob.SetInitScript(m.createInitScript())
					st, err := m.exitReadOnlyJob.Sync(ctx, false)
					return st.SyncStatus == SyncStatusReady, err
				},
			},
			StepRun{
				StepMeta: StepMeta{
					Name:               "DisableSafeMode",
					RunIfCondition:     not(masterSafeModeDisabledCond),
					OnSuccessCondition: masterSafeModeDisabledCond,
				},
				Body: m.ytClient.DisableSafeMode,
			},
		},
	)
}

func (m *Master) storeMasterMonitoringPaths(ctx context.Context, paths []string) error {
	return m.stateManager.SetMasterMonitoringPaths(ctx, paths)
}

func (m *Master) getStoredMasterMonitoringPaths() []string {
	return m.stateManager.GetMasterMonitoringPaths()
}
