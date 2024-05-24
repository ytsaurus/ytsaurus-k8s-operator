package components

import (
	"context"
)

var (
	qtInitFinishedCond               = isTrue("QTInitFinished")
	qtUserCreatedCond                = isTrue("QTUserCreated")
	qtInitQTStatePrepareStartedCond  = isTrue("QTInitQTStatePrepareStarted")
	qtInitQTStatePrepareFinishedCond = isTrue("QTInitQTStatePrepareFinished")
	qtInitQTStateFinishedCond        = isTrue("QTInitQTStateFinished")
)

func (qt *QueryTracker) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(qt, qt.doServerSync),
			getStandardWaitBuildFinishedStep(qt, qt.serverInSync),
			StepRun{
				StepMeta: StepMeta{
					Name:               "QueryTrackerInit",
					RunIfCondition:     not(qtInitFinishedCond),
					OnSuccessCondition: qtInitFinishedCond,
				},
				Body: func(ctx context.Context) error {
					return qt.init(ctx, qt.ytsaurusClient.GetYtClient())
				},
			},
			StepRun{
				StepMeta: StepMeta{
					Name:               "QueryTrackerCreateUser",
					RunIfCondition:     not(qtUserCreatedCond),
					OnSuccessCondition: qtUserCreatedCond,
				},
				Body: func(ctx context.Context) error {
					return qt.createUser(ctx, qt.ytsaurusClient.GetYtClient())
				},
			},
			// initQTState should be done on first build also?
			getStandardUpdateStep(
				qt,
				qt.condManager,
				qt.serverInSync,
				[]Step{
					getStandardStartRebuildStep(qt, qt.server.removePods),
					getStandardWaitPodsRemovedStep(qt, qt.server.arePodsRemoved),
					getStandardPodsCreateStep(qt, qt.doServerSync),
					getStandardWaiRebuildFinishedStep(qt, qt.serverInSync),
					// TODO: Suppose this job should be done once in init also.
					StepRun{
						StepMeta: StepMeta{
							Name:               "StartInitQTState",
							RunIfCondition:     not(qtInitQTStatePrepareStartedCond),
							OnSuccessCondition: qtInitQTStatePrepareStartedCond,
						},
						Body: func(ctx context.Context) error {
							return qt.initQTState.prepareRestart(ctx, false)
						},
					},
					StepCheck{
						StepMeta: StepMeta{
							Name:               "WaitInitQTStatePrepared",
							RunIfCondition:     not(qtInitQTStatePrepareFinishedCond),
							OnSuccessCondition: qtInitQTStatePrepareFinishedCond,
						},
						Body: func(ctx context.Context) (bool, error) {
							return qt.initQTState.isRestartPrepared(), nil
						},
					},
					StepCheck{
						StepMeta: StepMeta{
							Name:               "WaitInitQTStateFinished",
							RunIfCondition:     not(qtInitQTStateFinishedCond),
							OnSuccessCondition: qtInitQTStateFinishedCond,
						},
						Body: func(ctx context.Context) (ok bool, err error) {
							qt.prepareInitQueryTrackerState(qt.initQTState)
							st, err := qt.initQTState.Sync(ctx, false)
							return st.SyncStatus == SyncStatusReady, err
						},
					},
				},
			),
		},
	}
}
