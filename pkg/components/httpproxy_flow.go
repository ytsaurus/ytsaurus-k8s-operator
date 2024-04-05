package components

import (
	"context"
)

func (hp *HttpProxy) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(hp, hp.doServerSync),
			getStandardWaitBuildFinishedStep(hp, hp.serverInSync),
			getStandardUpdateStep(
				hp,
				hp.condManager,
				hp.serverInSync,
				[]Step{
					getStandardStartRebuildStep(hp, func(ctx context.Context) error {
						hp.server.removePodsNoSync()
						return hp.doServerSync(ctx)
					}),
					getStandardWaiRebuildFinishedStep(hp, hp.serverInSync),
				},
			),
		},
	}
}
