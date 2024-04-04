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

type fetchableWithName interface {
	resources.Fetchable
	GetName() string
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
