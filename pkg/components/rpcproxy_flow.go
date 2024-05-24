package components

func (rp *RpcProxy) getFlow() Step {
	return StepComposite{
		Body: []Step{
			getStandardStartBuildStep(rp, rp.doServerSync),
			getStandardWaitBuildFinishedStep(rp, rp.serverInSync),
			getStandardUpdateStep(
				rp,
				rp.condManager,
				rp.serverInSync,
				[]Step{
					getStandardStartRebuildStep(rp, rp.server.removePods),
					getStandardWaitPodsRemovedStep(rp, rp.server.arePodsRemoved),
					getStandardPodsCreateStep(rp, rp.doServerSync),
					getStandardWaiRebuildFinishedStep(rp, rp.serverInSync),
				},
			),
		},
	}
}
