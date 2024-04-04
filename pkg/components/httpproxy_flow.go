package components

func (hp *HttpProxy) getFlow() Step {
	return StepComposite{
		Steps: []Step{
			getStandardStartBuildStep(hp, hp.doServerSync),
			getStandardWaitBuildFinishedStep(hp, hp.serverInSync),
			getStandardUpdateStep(
				hp,
				hp.condManager,
				hp.serverInSync,
				[]Step{
					getStandardStartRebuildStep(hp, hp.server.removePods),
					getStandardWaiRebuildFinishedStep(hp, hp.serverInSync),
				},
			),
		},
	}
}
