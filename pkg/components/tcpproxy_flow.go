package components

func (tp *TcpProxy) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(tp, tp.doServerSync),
			getStandardWaitBuildFinishedStep(tp, tp.serverInSync),
			getStandardUpdateStep(
				tp,
				tp.condManager,
				tp.serverInSync,
				[]Step{
					getStandardStartRebuildStep(tp, tp.server.removePods),
					getStandardWaitPodsRemovedStep(tp, tp.server.arePodsRemoved),
					getStandardPodsCreateStep(tp, tp.doServerSync),
					getStandardWaiRebuildFinishedStep(tp, tp.serverInSync),
				},
			),
		},
	}
}
