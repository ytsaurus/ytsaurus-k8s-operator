package components

func (mc *MasterCache) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(mc, mc.doServerSync),
			getStandardWaitBuildFinishedStep(mc, mc.server.inSync),
			getStandardUpdateStep(
				mc,
				mc.condManager,
				mc.server.inSync,
				[]Step{
					getStandardStartRebuildStep(mc, mc.server.removePods),
					getStandardWaitPodsRemovedStep(mc, mc.server.arePodsRemoved),
					getStandardPodsCreateStep(mc, mc.doServerSync),
					getStandardWaiRebuildFinishedStep(mc, mc.server.inSync),
				},
			),
		},
	}
}
