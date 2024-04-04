package components

func (ca *ControllerAgent) getFlow() Step {
	return StepComposite{
		Steps: []Step{
			getStandardStartBuildStep(ca, ca.server.Sync),
			getStandardWaitBuildFinishedStep(ca, ca.server.inSync),
			getStandardUpdateStep(
				ca,
				ca.condManager,
				ca.server.inSync,
				[]Step{
					getStandardStartRebuildStep(ca, ca.server.removePods),
					getStandardWaiRebuildFinishedStep(ca, ca.server.inSync),
				},
			),
		},
	}
}
