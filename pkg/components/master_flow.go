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
		Steps: []Step{
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
				Name:           "UpdatePossibleCheck",
				RunIfCondition: not(masterUpdatePossibleCond),
				StatusFunc: func(ctx context.Context) (SyncStatus, string, error) {
					possible, msg, err := m.ytClient.HandlePossibilityCheck(ctx)
					st := SyncStatusReady
					if !possible {
						st = SyncStatusBlocked
					}
					return st, msg, err
				},
				OnSuccessCondition: masterUpdatePossibleCond,
			},
			StepRun{
				Name:               "EnableSafeMode",
				RunIfCondition:     not(masterSafeModeEnabledCond),
				OnSuccessCondition: masterSafeModeEnabledCond,
				RunFunc:            m.ytClient.EnableSafeMode,
			},
			StepRun{
				Name:               "BuildSnapshots",
				RunIfCondition:     not(masterSnapshotsBuildStartedCond),
				OnSuccessCondition: masterSnapshotsBuildStartedCond,
				RunFunc: func(ctx context.Context) error {
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
				Name:               "CheckSnapshotsBuild",
				RunIfCondition:     not(masterSnapshotsBuildFinishedCond),
				OnSuccessCondition: masterSnapshotsBuildFinishedCond,
				RunFunc: func(ctx context.Context) (ok bool, err error) {
					paths := m.getStoredMasterMonitoringPaths()
					return m.ytClient.AreMasterSnapshotsBuilt(ctx, paths)
				},
			},
			getStandardStartRebuildStep(m, m.server.removePods),
			getStandardWaiRebuildFinishedStep(m, m.server.inSync),
			StepRun{
				Name:               "StartPrepareMasterExitReadOnly",
				RunIfCondition:     not(masterExitReadOnlyPrepareStartedCond),
				OnSuccessCondition: masterExitReadOnlyPrepareStartedCond,
				RunFunc: func(ctx context.Context) error {
					return m.exitReadOnlyJob.prepareRestart(ctx, false)
				},
			},
			StepCheck{
				Name:               "WaitMasterExitReadOnlyPrepared",
				RunIfCondition:     not(masterExitReadOnlyPrepareFinishedCond),
				OnSuccessCondition: masterExitReadOnlyPrepareFinishedCond,
				RunFunc: func(ctx context.Context) (bool, error) {
					return m.exitReadOnlyJob.isRestartPrepared(), nil
				},
				OnSuccessFunc: func(ctx context.Context) error {
					m.exitReadOnlyJob.SetInitScript(m.createInitScript())
					return nil
				},
			},
			StepCheck{
				Name:               "WaitMasterExitsReadOnly",
				RunIfCondition:     not(masterExitReadOnlyFinished),
				OnSuccessCondition: masterExitReadOnlyFinished,
				RunFunc: func(ctx context.Context) (ok bool, err error) {
					st, err := m.exitReadOnlyJob.Sync(ctx, false)
					return st.SyncStatus == SyncStatusReady, err
				},
			},
			StepRun{
				Name:               "DisableSafeMode",
				RunIfCondition:     not(masterSafeModeDisabledCond),
				OnSuccessCondition: masterSafeModeDisabledCond,
				RunFunc:            m.ytClient.DisableSafeMode,
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
