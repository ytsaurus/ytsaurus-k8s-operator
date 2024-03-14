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
		steps[compNameToStepName(compName)] = newComponentStep(singleComp)
	}
	return steps
}

func buildActionSteps(comps *componentRegistry, state stateManager) map[StepName]stepType {
	yc := comps.single[YtsaurusClientName].(ytsaurusClient)
	return map[StepName]stepType{
		CheckFullUpdatePossibilityStep: checkFullUpdatePossibility(yc, state),
		EnableSafeModeStep:             enableSafeMode(yc, state),
		BackupTabletCellsStep:          backupTabletCells(yc, state),
		BuildMasterSnapshotsStep:       buildMasterSnapshots(yc, state),
		MasterExitReadOnlyStep:         masterExitReadOnly(state),
		RecoverTabletCellsStep:         recoverTabletCells(yc, state),
		UpdateOpArchiveStep:            updateOpArchive(state),
		InitQueryTrackerStep:           initQueryTracker(state),
		DisableSafeModeStep:            disableSafeMode(yc, state),
	}
}

func buildSteps(comps *componentRegistry, actions map[StepName]stepType) *stepRegistry {
	steps := make(map[StepName]stepType)

	for name, step := range buildComponentSteps(comps) {
		steps[name] = step
	}
	for name, step := range actions {
		steps[name] = step
	}
	return &stepRegistry{
		steps: steps,
	}
}
