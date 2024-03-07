package ytflow

type singleComponentStep struct {
	comp component
	//status componentStatus
}

func newSingleComponentStep(comp component, status componentStatus) *singleComponentStep {
	return &singleComponentStep{
		comp: comp,
		//status: status,
	}
}

type multiComponentStep struct {
	comps map[string]component
	//statuses map[string]componentStatus
}

func newMultiComponentStep(comps map[string]component, statuses map[string]componentStatus) *multiComponentStep {
	return &multiComponentStep{
		comps: comps,
		//statuses: statuses,
	}
}
