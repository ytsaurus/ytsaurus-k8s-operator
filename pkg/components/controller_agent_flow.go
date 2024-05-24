package components

func (ca *ControllerAgent) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(ca, ca.server.Sync),
			getStandardWaitBuildFinishedStep(ca, ca.server.inSync),
			getStandardUpdateStep(
				ca,
				ca.condManager,
				ca.server.inSync,
				[]Step{
					getStandardStartRebuildStep(ca, ca.server.removePods),
					getStandardWaitPodsRemovedStep(ca, ca.server.arePodsRemoved),
					getStandardPodsCreateStep(ca, ca.server.Sync),
					getStandardWaiRebuildFinishedStep(ca, ca.server.inSync),
				},
			),
		},
	}
}
