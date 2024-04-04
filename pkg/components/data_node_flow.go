package components

func (n *DataNode) getFlow() Step {
	return StepComposite{
		Steps: []Step{
			getStandardStartBuildStep(n, n.server.Sync),
			getStandardWaitBuildFinishedStep(n, n.server.inSync),
			getStandardUpdateStep(
				n,
				n.condManager,
				n.server.inSync,
				[]Step{
					getStandardStartRebuildStep(n, n.server.removePods),
					getStandardWaiRebuildFinishedStep(n, n.server.inSync),
				},
			),
		},
	}
}
