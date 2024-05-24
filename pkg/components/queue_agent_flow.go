package components

import (
	"context"
)

var (
	qaInitFinishedCond               = isTrue("QAInitFinished")
	qaUserCreatedCond                = isTrue("QAUserCreated")
	qaInitQAStatePrepareStartedCond  = isTrue("QAInitQAStatePrepareStarted")
	qaInitQAStatePrepareFinishedCond = isTrue("QAInitQAStatePrepareFinished")
	qaInitQAStateFinishedCond        = isTrue("QAInitQAStateFinished")
)

func (qa *QueueAgent) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(qa, qa.doServerSync),
			getStandardWaitBuildFinishedStep(qa, qa.serverInSync),
			StepRun{
				StepMeta: StepMeta{
					Name:               "QueueAgentInit",
					RunIfCondition:     not(qaInitFinishedCond),
					OnSuccessCondition: qaInitFinishedCond,
				},
				Body: func(ctx context.Context) error {
					return qa.init(ctx, qa.ytsaurusClient.GetYtClient())
				},
			},
			StepRun{
				StepMeta: StepMeta{
					Name:               "QueueAgentCreateUser",
					RunIfCondition:     not(qaUserCreatedCond),
					OnSuccessCondition: qaUserCreatedCond,
				},
				Body: func(ctx context.Context) error {
					return qa.createUser(ctx, qa.ytsaurusClient.GetYtClient())
				},
			},
			// TODO: initQAState on first build also
			getStandardUpdateStep(
				qa,
				qa.condManager,
				qa.serverInSync,
				[]Step{
					getStandardStartRebuildStep(qa, qa.server.removePods),
					getStandardWaitPodsRemovedStep(qa, qa.server.arePodsRemoved),
					getStandardPodsCreateStep(qa, qa.doServerSync),
					getStandardWaiRebuildFinishedStep(qa, qa.serverInSync),
					// TODO: Suppose this job should be done once in init also.
					StepRun{
						StepMeta: StepMeta{
							Name:               "StartInitQAState",
							RunIfCondition:     not(qaInitQAStatePrepareStartedCond),
							OnSuccessCondition: qaInitQAStatePrepareStartedCond,
						},
						Body: func(ctx context.Context) error {
							return qa.initQAState.prepareRestart(ctx, false)
						},
					},
					StepCheck{
						StepMeta: StepMeta{
							Name:               "WaitInitQAStatePrepared",
							RunIfCondition:     not(qaInitQAStatePrepareFinishedCond),
							OnSuccessCondition: qaInitQAStatePrepareFinishedCond,
						},
						Body: func(ctx context.Context) (bool, error) {
							return qa.initQAState.isRestartPrepared(), nil
						},
					},
					StepCheck{
						StepMeta: StepMeta{
							Name:               "WaitInitQAStateFinished",
							RunIfCondition:     not(qaInitQAStateFinishedCond),
							OnSuccessCondition: qaInitQAStateFinishedCond,
						},
						Body: func(ctx context.Context) (ok bool, err error) {
							qa.prepareInitQueueAgentState(qa.initQAState)
							st, err := qa.initQAState.Sync(ctx, false)
							return st.SyncStatus == SyncStatusReady, err
						},
					},
				},
			),
		},
	}
}
