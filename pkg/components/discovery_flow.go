package components

func (d *Discovery) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(d, d.server.Sync),
			getStandardWaitBuildFinishedStep(d, d.server.inSync),
			getStandardUpdateStep(
				d,
				d.condManager,
				d.server.inSync,
				[]Step{
					getStandardStartRebuildStep(d, d.server.removePods),
					getStandardWaitPodsRemovedStep(d, d.server.arePodsRemoved),
					getStandardPodsCreateStep(d, d.server.Sync),
					getStandardWaiRebuildFinishedStep(d, d.server.inSync),
				},
			),
		},
	}
}
