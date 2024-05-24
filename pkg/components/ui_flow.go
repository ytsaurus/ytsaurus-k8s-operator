package components

import (
	"context"
)

func (u *UI) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(u, u.doServerSync),
			getStandardWaitBuildFinishedStep(u, u.serverInSync),
			getStandardInitFinishedStep(u, func(ctx context.Context) (ok bool, err error) {
				u.initJob.SetInitScript(u.createInitScript())
				st, err := u.initJob.Sync(ctx, false)
				return st.SyncStatus == SyncStatusReady, err
			}),
			getStandardUpdateStep(
				u,
				u.condManager,
				u.serverInSync,
				[]Step{
					getStandardStartRebuildStep(u, u.microservice.removePods),
					getStandardWaitPodsRemovedStep(u, u.microservice.arePodsRemoved),
					getStandardPodsCreateStep(u, u.doServerSync),
					getStandardWaiRebuildFinishedStep(u, u.serverInSync),
				},
			),
		},
	}
}
