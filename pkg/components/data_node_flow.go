package components

func (n *DataNode) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(n, n.server.Sync),
			getStandardWaitBuildFinishedStep(n, n.server.inSync),
			getStandardUpdateStep(
				n,
				n.condManager,
				n.server.inSync,
				[]Step{
					getStandardStartRebuildStep(n, n.server.removePods),
					getStandardWaitPodsRemovedStep(n, n.server.arePodsRemoved),
					getStandardPodsCreateStep(n, n.server.Sync),
					getStandardWaiRebuildFinishedStep(n, n.server.inSync),
				},
			),
		},
	}
}
