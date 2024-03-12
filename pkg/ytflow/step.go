package ytflow

import (
	"context"
)

type stepType interface {
	Run(ctx context.Context) error
}

type stepRegistry struct {
	steps map[StepName]stepType
}

func buildComponentSteps(comps *componentRegistry) map[StepName]stepType {
	steps := make(map[StepName]stepType)

	for compName, singleComp := range comps.single {
		steps[compNameToStepName(compName)] = newSingleComponentStep(singleComp)
	}

	for compName, multiComp := range comps.multi {
		steps[compNameToStepName(compName)] = newMultiComponentStep(multiComp)
	}
	return steps
}

func buildActionSteps(comps *componentRegistry, conds stateManager) map[StepName]stepType {
	yc := comps.single[YtsaurusClientName].(ytsaurusClient)
	return map[StepName]stepType{
		CheckFullUpdatePossibilityStep: checkFullUpdatePossibility(yc, conds),
		EnableSafeModeStep:             enableSafeMode(yc, conds),
		BackupTabletCellsStep:          backupTabletCells(yc, conds),
		BuildMasterSnapshotsStep:       buildMasterSnapshots(yc, conds),
		RecoverTabletCellsStep:         recoverTabletCells(yc, conds),
		DisableSafeModeStep:            disableSafeMode(yc, conds),
	}
}

func buildSteps(comps *componentRegistry, conds stateManager) *stepRegistry {
	steps := make(map[StepName]stepType)

	for name, step := range buildComponentSteps(comps) {
		steps[name] = step
	}
	for name, step := range buildActionSteps(comps, conds) {
		steps[name] = step
	}
	return &stepRegistry{
		steps: steps,
	}
}
