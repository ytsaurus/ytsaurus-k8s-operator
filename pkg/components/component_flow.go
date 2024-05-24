package components

import (
	"context"
	"fmt"

	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
)

const (
	StepStartBuild          = "StartBuild"
	StepWaitBuildFinished   = "WaitBuildFinished"
	StepInitStarted         = "InitStarted"
	StepInitFinished        = "InitFinished"
	StepCheckUpdateRequired = "CheckUpdateRequired"
	StepUpdate              = "Update"
	StepStartRebuild        = "StartRebuild"
	StepWaitPodsRemoved     = "WaitPodsRemoved"
	StepPodsCreate          = "PodsCreate"
	StepWaitRebuildFinished = "WaitRebuildFinished"
)

type withName interface {
	GetName() string
}

type fetchableWithName interface {
	resources.Fetchable
	withName
}

func flowToStatus(ctx context.Context, c fetchableWithName, flow Step, condManager *ConditionManager) (ComponentStatus, error) {
	if err := c.Fetch(ctx); err != nil {
		return ComponentStatus{}, fmt.Errorf("failed to fetch component %s: %w", c.GetName(), err)
	}

	return flow.Status(ctx, condManager)
}

func flowToSync(ctx context.Context, flow Step, condManager *ConditionManager) error {
	_, err := flow.Run(ctx, condManager)
	return err
}

// TODO: make all components have standard method for build/check issync.
func getStandardStartBuildStep(c withName, build func(ctx context.Context) error) StepRun {
	name := c.GetName()
	buildStartedCond := buildStarted(name)
	return StepRun{
		StepMeta: StepMeta{
			Name:               StepStartBuild,
			RunIfCondition:     not(buildStartedCond),
			OnSuccessCondition: buildStartedCond,
		},
		Body: build,
	}
}
func getStandardWaitBuildFinishedStep(c withName, check func(ctx context.Context) (ok bool, err error)) StepCheck {
	name := c.GetName()
	builtFinishedCond := buildFinished(name)
	return StepCheck{
		StepMeta: StepMeta{
			Name:               StepWaitBuildFinished,
			RunIfCondition:     not(builtFinishedCond),
			OnSuccessCondition: builtFinishedCond,
		},
		Body: check,
	}
}
func getStandardStartRebuildStep(c withName, run func(ctx context.Context) error) StepRun {
	name := c.GetName()
	rebuildStartedCond := rebuildStarted(name)
	return StepRun{
		StepMeta: StepMeta{
			Name:               StepStartRebuild,
			RunIfCondition:     not(rebuildStartedCond),
			OnSuccessCondition: rebuildStartedCond,
		},
		Body: run,
	}
}
func getStandardWaitPodsRemovedStep(c withName, check func(ctx context.Context) bool) StepCheck {
	name := c.GetName()
	podsRemovedCond := podsRemoved(name)
	return StepCheck{
		StepMeta: StepMeta{
			Name:               StepWaitPodsRemoved,
			RunIfCondition:     not(podsRemovedCond),
			OnSuccessCondition: podsRemovedCond,
		},
		Body: func(ctx context.Context) (ok bool, err error) {
			return check(ctx), nil
		},
	}
}
func getStandardPodsCreateStep(c withName, build func(ctx context.Context) error) StepRun {
	name := c.GetName()
	podsCreatedCond := podsCreated(name)
	return StepRun{
		StepMeta: StepMeta{
			Name:               StepPodsCreate,
			RunIfCondition:     not(podsCreatedCond),
			OnSuccessCondition: podsCreatedCond,
		},
		Body: build,
	}
}
func getStandardWaiRebuildFinishedStep(c withName, check func(ctx context.Context) (ok bool, err error)) StepCheck {
	name := c.GetName()
	rebuildFinishedCond := rebuildFinished(name)
	return StepCheck{
		StepMeta: StepMeta{
			Name:               StepWaitRebuildFinished,
			RunIfCondition:     not(rebuildFinishedCond),
			OnSuccessCondition: rebuildFinishedCond,
		},
		Body: check,
	}
}
func getStandardInitFinishedStep(c withName, check func(ctx context.Context) (ok bool, err error)) StepCheck {
	name := c.GetName()
	initCond := initializationFinished(name)
	return StepCheck{
		StepMeta: StepMeta{
			Name:               StepInitFinished,
			RunIfCondition:     not(initCond),
			OnSuccessCondition: initCond,
		},
		Body: check,
	}
}
func getStandardUpdateStep(
	c withName,
	condManager *ConditionManager,
	check func(ctx context.Context) (bool, error),
	steps []Step,
) StepComposite {
	name := c.GetName()
	updateRequiredCond := updateRequired(name)

	allSteps := []Step{
		StepRun{
			StepMeta: StepMeta{
				Name:               StepCheckUpdateRequired,
				RunIfCondition:     not(updateRequiredCond),
				OnSuccessCondition: updateRequiredCond,
				// If update started — setting updateRequired unconditionally.
			},
		},
	}
	allSteps = append(allSteps, steps...)

	var onSuccessConditions []Condition
	for _, stepI := range allSteps {
		var cond Condition
		switch step := stepI.(type) {
		case StepRun:
			cond = step.OnSuccessCondition
		case StepCheck:
			cond = step.OnSuccessCondition
		case StepComposite:
			cond = step.OnSuccessCondition
		}
		onSuccessConditions = append(onSuccessConditions, not(cond))
	}

	return StepComposite{
		StepMeta: StepMeta{
			Name: StepUpdate,
			// Update should be run if either diff exists or updateRequired condition is set,
			// because a diff should disappear in the middle of the update, but it still needs
			// to finish actions after the update.
			StatusFunc: func(ctx context.Context) (SyncStatus, string, error) {
				inSync, err := check(ctx)
				if err != nil {
					return "", "", err
				}
				if !inSync || condManager.IsSatisfied(updateRequiredCond) {
					return SyncStatusNeedSync, "", nil
				}
				return SyncStatusReady, "", nil
			},
			OnSuccessFunc: func(ctx context.Context) error {
				return condManager.SetCondMany(
					ctx,
					onSuccessConditions...,
				)
			},
		},
		Body: allSteps,
	}
}
