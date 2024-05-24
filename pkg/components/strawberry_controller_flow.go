package components

import (
	"context"
)

func (c *StrawberryController) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(c, c.doServerSync),
			getStandardWaitBuildFinishedStep(c, c.serverInSync),
			StepCheck{
				StepMeta: StepMeta{
					Name:               "InitUserAndUrl",
					RunIfCondition:     not(initializationStarted(c.GetName())),
					OnSuccessCondition: initializationStarted(c.GetName()),
				},
				Body: func(ctx context.Context) (ok bool, err error) {
					c.initUserAndUrlJob.SetInitScript(c.createInitUserAndUrlScript())
					st, err := c.initUserAndUrlJob.Sync(ctx, false)
					return st.SyncStatus == SyncStatusReady, err
				},
			},
			getStandardInitFinishedStep(c, func(ctx context.Context) (ok bool, err error) {
				c.initChytClusterJob.SetInitScript(c.createInitChytClusterScript())
				st, err := c.initChytClusterJob.Sync(ctx, false)
				return st.SyncStatus == SyncStatusReady, err
			}),
			getStandardUpdateStep(
				c,
				c.condManager,
				c.serverInSync,
				[]Step{
					getStandardStartRebuildStep(c, c.microservice.removePods),
					getStandardWaitPodsRemovedStep(c, c.microservice.arePodsRemoved),
					getStandardPodsCreateStep(c, c.doServerSync),
					getStandardWaiRebuildFinishedStep(c, c.serverInSync),
				},
			),
		},
	}
}
