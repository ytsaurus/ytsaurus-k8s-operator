package components

import (
	"context"
)

func (yc *YtsaurusClient) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(yc, yc.doKubeSync),
			getStandardWaitBuildFinishedStep(yc, yc.isInSync),
			getStandardInitFinishedStep(
				yc,
				func(ctx context.Context) (ok bool, err error) {
					yc.initUserJob.SetInitScript(yc.createInitUserScript())
					st, err := yc.initUserJob.Sync(ctx, false)
					return st.SyncStatus == SyncStatusReady, err
				},
			),
			getStandardUpdateStep(
				yc,
				yc.condManager,
				yc.isInSync,
				[]Step{
					getStandardStartRebuildStep(yc, yc.doKubeSync),
					getStandardWaiRebuildFinishedStep(yc, yc.isInSync),
				},
			),
		},
	}
}
