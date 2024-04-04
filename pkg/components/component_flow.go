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
	StepWaitRebuildFinished = "WaitRebuildFinished"
)

type withName interface {
	GetName() string
}

type fetchableWithName interface {
	resources.Fetchable
	withName
}

func flowToStatus(ctx context.Context, c fetchableWithName, flow Step, condManager conditionManagerIface) (ComponentStatus, error) {
	if err := c.Fetch(ctx); err != nil {
		return ComponentStatus{}, fmt.Errorf("failed to fetch component %s: %w", c.GetName(), err)
	}

	st, msg, err := flow.Status(ctx, condManager)
	return ComponentStatus{
		SyncStatus: st,
		Message:    msg,
	}, err
}

func flowToSync(ctx context.Context, flow Step, condManager conditionManagerIface) error {
	_, err := flow.Run(ctx, condManager)
	return err
}

// TODO: make all components have standard method for build/check issync.
func getStandardStartBuildStep(c withName, build func(ctx context.Context) error) StepRun {
	name := c.GetName()
	buildStartedCond := buildStarted(name)
	return StepRun{
		Name:               StepStartBuild,
		RunIfCondition:     not(buildStartedCond),
		RunFunc:            build,
		OnSuccessCondition: buildStartedCond,
	}
}
func getStandardWaitBuildFinishedStep(c withName, check func(ctx context.Context) (ok bool, err error)) StepCheck {
	name := c.GetName()
	builtFinishedCond := buildFinished(name)
	return StepCheck{
		Name:               StepWaitBuildFinished,
		RunIfCondition:     not(builtFinishedCond),
		OnSuccessCondition: builtFinishedCond,
		RunFunc:            check,
	}
}
func getStandardStartRebuildStep(c withName, run func(ctx context.Context) error) StepRun {
	name := c.GetName()
	rebuildStartedCond := rebuildStarted(name)
	return StepRun{
		Name:               StepStartRebuild,
		RunIfCondition:     not(rebuildStartedCond),
		OnSuccessCondition: rebuildStartedCond,
		RunFunc:            run,
	}
}
func getStandardWaiRebuildFinishedStep(c withName, check func(ctx context.Context) (ok bool, err error)) StepCheck {
	name := c.GetName()
	rebuildFinishedCond := rebuildFinished(name)
	return StepCheck{
		Name:               StepWaitBuildFinished,
		RunIfCondition:     not(rebuildFinishedCond),
		OnSuccessCondition: rebuildFinishedCond,
		RunFunc:            check,
	}
}
func getStandardInitFinishedStep(c withName, check func(ctx context.Context) (ok bool, err error)) StepCheck {
	name := c.GetName()
	initCond := initializationFinished(name)
	return StepCheck{
		Name:               StepInitFinished,
		RunIfCondition:     not(initCond),
		OnSuccessCondition: initCond,
		RunFunc:            check,
	}
}
func getStandardUpdateStep(
	c withName,
	condManager conditionManagerIface,
	check func(ctx context.Context) (bool, error),
	steps []Step,
) StepComposite {
	name := c.GetName()
	updateRequiredCond := updateRequired(name)

	allSteps := []Step{
		StepRun{
			Name:               StepCheckUpdateRequired,
			RunIfCondition:     not(updateRequiredCond),
			OnSuccessCondition: updateRequiredCond,
			// If update started — setting updateRequired unconditionally.
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
		Name: StepUpdate,
		// Update should be run if either diff exists or updateRequired condition is set,
		// because a diff should disappear in the middle of the update, but it still needs
		// to finish actions after the update.
		StatusConditionFunc: func(ctx context.Context) (SyncStatus, string, error) {
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
		Steps: allSteps,
	}
}
