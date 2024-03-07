package ytflow

import (
	"context"
	"fmt"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

func updateComponentsConditions(ctx context.Context, statuses *statusRegistry, conds conditionManagerType) error {

	// Actualize built condition for the single components.
	for compName, status := range statuses.single {
		sName := compNameToStepName(compName)
		condName := isBuilt(sName)

		isComponentBuilt := status.SyncStatus != components.SyncStatusReady
		msg := fmt.Sprintf("%s: %s", status.SyncStatus, status.Message)
		if err := conds.Set(ctx, condName, isComponentBuilt, msg); err != nil {
			return err
		}
	}

	// Actualize built condition for the multi components.
	for compName, substatuses := range statuses.multi {
		sName := compNameToStepName(compName)
		condName := isBuilt(sName)

		for subComponentName, status := range substatuses {
			if status.SyncStatus != components.SyncStatusReady {
				msg := fmt.Sprintf("%s for %s: %s", status.SyncStatus, subComponentName, status.Message)
				if err := conds.SetFalse(ctx, condName, msg); err != nil {
					return err
				}
			}
		}
		if err := conds.SetTrue(ctx, condName, ""); err != nil {
			return err
		}
	}

	// Check is full update needed.
	isFullUpdateNeeded := statuses.single[MasterName].SyncStatus == components.SyncStatusNeedLocalUpdate
	if err := conds.Set(ctx, NeedFullUpdate, isFullUpdateNeeded, ""); err != nil {
		return err
	}

	return nil
}
