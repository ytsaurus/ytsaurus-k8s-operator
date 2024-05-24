package components

import (
	"context"
)

func (yqla *YqlAgent) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(yqla, yqla.doServerSync),
			getStandardWaitBuildFinishedStep(yqla, yqla.serverInSync),
			getStandardInitFinishedStep(yqla, func(ctx context.Context) (ok bool, err error) {
				yqla.initEnvironment.SetInitScript(yqla.createInitScript())
				st, err := yqla.initEnvironment.Sync(ctx, false)
				return st.SyncStatus == SyncStatusReady, err
			}),
			getStandardUpdateStep(
				yqla,
				yqla.condManager,
				yqla.serverInSync,
				[]Step{
					getStandardStartRebuildStep(yqla, yqla.server.removePods),
					getStandardWaitPodsRemovedStep(yqla, yqla.server.arePodsRemoved),
					getStandardPodsCreateStep(yqla, yqla.doServerSync),
					getStandardWaiRebuildFinishedStep(yqla, yqla.serverInSync),
				},
			),
		},
	}
}
