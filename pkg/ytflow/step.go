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

	for compName, singleComp := range comps.components {
		steps[compNameToStepName(compName)] = newComponentStep(singleComp)
	}
	return steps
}

func buildActionSteps(comps *componentRegistry, state stateManager) map[StepName]stepType {
	yc := comps.components[YtsaurusClientName].(ytsaurusClient)
	ms := comps.components[MasterName].(masterComponent)

	var updateOpArchiveStep stepType
	if sch, exists := comps.components[SchedulerName]; exists {
		updateOpArchiveStep = updateOpArchive(sch.(schedulerComponent), state)
	} else {
		// TODO: for now it is simpler, but i'm not sure if it's good to have
		// condition flipped even if no real job was done.
		updateOpArchiveStep = newNoopStep(not(OperationArchiveNeedUpdate), state)
	}

	var initQueryTrackerStep stepType
	if qt, exists := comps.components[QueryTrackerName]; exists {
		initQueryTrackerStep = initQueryTracker(qt.(queryTrackerComponent), state)
	} else {
		initQueryTrackerStep = newNoopStep(not(QueryTrackerNeedsInit), state)

	}
	return map[StepName]stepType{
		CheckFullUpdatePossibilityStep: checkFullUpdatePossibility(yc, state),
		EnableSafeModeStep:             enableSafeMode(yc, state),
		BackupTabletCellsStep:          backupTabletCells(yc, state),
		BuildMasterSnapshotsStep:       buildMasterSnapshots(yc, state),
		MasterExitReadOnlyStep:         masterExitReadOnly(ms, state),
		RecoverTabletCellsStep:         recoverTabletCells(yc, state),
		UpdateOpArchiveStep:            updateOpArchiveStep,
		InitQueryTrackerStep:           initQueryTrackerStep,
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
