package components

import (
	"context"
	"fmt"
)

const (
	StepStartBuild          = "StartBuild"
	StepWaitBuildFinished   = "WaitBuildFinished"
	StepInitFinished        = "InitFinished"
	StepCheckUpdateRequired = "CheckUpdateRequired"
	StepUpdate              = "Update"
	StepStartRebuild        = "StartRebuild"
	StepWaitRebuildFinished = "WaitRebuildFinished"
)

func flowToStatus(ctx context.Context, c Component, flow Step, condManager conditionManagerIface) (ComponentStatus, error) {
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
