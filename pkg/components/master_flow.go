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

func (m *Master) getFlow() Step {
	name := m.GetName()
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
				RunFunc:            m.doServerSync,
				OnSuccessCondition: buildStartedCond,
			},
			StepCheck{
				Name:               StepWaitBuildFinished,
				RunIfCondition:     not(builtFinishedCond),
				OnSuccessCondition: builtFinishedCond,
				RunFunc:            m.server.inSync,
			},
			StepCheck{
				Name:               StepInitFinished,
				RunIfCondition:     not(initCond),
				OnSuccessCondition: initCond,
				RunFunc: func(ctx context.Context) (ok bool, err error) {
					m.initJob.SetInitScript(m.createInitScript())
					st, err := m.initJob.Sync(ctx, false)
					return st.SyncStatus == SyncStatusReady, err
				},
			},
			StepComposite{
				Name: StepUpdate,
				// Update should be run if either diff exists or updateRequired condition is set,
				// because a diff should disappear in the middle of the update, but it still need
				// to finish actions after the update (master exit read only, safe mode, etc.).
				StatusConditionFunc: func(ctx context.Context) (SyncStatus, string, error) {
					inSync, err := m.server.inSync(ctx)
					if err != nil {
						return "", "", err
					}
					if !inSync {
						if err = m.condManager.SetCond(ctx, updateRequiredCond); err != nil {
							return "", "", err
						}
					}
					if !inSync || m.condManager.IsSatisfied(updateRequiredCond) {
						return SyncStatusNeedSync, "", nil
					}
					return SyncStatusReady, "", nil
				},
				OnSuccessCondition: not(updateRequiredCond),
				OnSuccessFunc: func(ctx context.Context) error {
					return m.condManager.SetCondMany(
						ctx,
						not(masterUpdatePossibleCond),
						not(masterSafeModeEnabledCond),
						not(masterSnapshotsBuildStartedCond),
						not(masterSnapshotsBuildFinishedCond),
						not(rebuildStarted(name)),
						not(rebuildFinished(name)),
						not(masterExitReadOnlyPrepareStartedCond),
						not(masterExitReadOnlyPrepareFinishedCond),
						not(masterExitReadOnlyFinished),
						not(masterSafeModeDisabledCond),
					)
				},
				Steps: []Step{
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
					StepRun{
						Name:               StepStartRebuild,
						RunIfCondition:     not(rebuildStartedCond),
						OnSuccessCondition: rebuildStartedCond,
						RunFunc:            m.server.removePods,
					},
					StepCheck{
						Name:               StepWaitRebuildFinished,
						RunIfCondition:     not(rebuildFinishedCond),
						OnSuccessCondition: rebuildFinishedCond,
						RunFunc:            m.server.inSync,
					},
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
			},
		},
	}
}

func (m *Master) storeMasterMonitoringPaths(ctx context.Context, paths []string) error {
	return m.stateManager.SetMasterMonitoringPaths(ctx, paths)
}

func (m *Master) getStoredMasterMonitoringPaths() []string {
	return m.stateManager.GetMasterMonitoringPaths()
}
