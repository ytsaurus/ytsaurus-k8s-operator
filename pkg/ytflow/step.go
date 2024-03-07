package ytflow

type stepType interface {
	//Name() stepName
}

type stepRegistry struct {
	steps map[stepName]stepType
}

func buildComponentSteps(comps *componentRegistry, statuses *statusRegistry) map[stepName]stepType {
	steps := make(map[stepName]stepType)

	for compName, singleComp := range comps.single {
		steps[compNameToStepName(compName)] = newSingleComponentStep(
			singleComp,
			statuses.single[compName],
		)
	}

	for compName, multiComp := range comps.multi {
		steps[compNameToStepName(compName)] = newMultiComponentStep(
			multiComp,
			statuses.multi[compName],
		)
	}
	return steps
}

func buildSteps(comps *componentRegistry, statuses *statusRegistry) *stepRegistry {
	steps := make(map[stepName]stepType)

	for name, step := range buildComponentSteps(comps, statuses) {
		steps[name] = step
	}
	return &stepRegistry{
		steps: steps,
	}
}
