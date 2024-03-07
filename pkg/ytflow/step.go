package ytflow

import (
	"context"
)

type stepType interface {
	Run(ctx context.Context) error
}

type stepRegistry struct {
	steps map[stepName]stepType
}

func buildComponentSteps(comps *componentRegistry) map[stepName]stepType {
	steps := make(map[stepName]stepType)

	for compName, singleComp := range comps.single {
		steps[compNameToStepName(compName)] = newSingleComponentStep(singleComp)
	}

	for compName, multiComp := range comps.multi {
		steps[compNameToStepName(compName)] = newMultiComponentStep(multiComp)
	}
	return steps
}

func buildActionSteps(comps *componentRegistry, conds conditionManagerType) map[stepName]stepType {
	yc := comps.single[YtsaurusClientName].(ytsaurusClient)
	return map[stepName]stepType{
		CheckFullUpdatePossibilityStep: checkFullUpdatePossibility(yc, conds),
		EnableSafeModeStep:             enableSafeMode(yc, conds),
		BackupTabletCellsStep:          backupTabletCells(yc, conds),
		BuildMasterSnapshotsStep:       buildMasterSnapshots(yc, conds),
		RecoverTabletCellsStep:         recoverTabletCells(yc, conds),
		DisableSafeModeStep:            disableSafeMode(yc, conds),
	}
}

func buildSteps(comps *componentRegistry, conds conditionManagerType) *stepRegistry {
	steps := make(map[stepName]stepType)

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
