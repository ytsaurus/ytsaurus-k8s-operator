package controllers

import (
	"context"
	"errors"
	"fmt"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

func getStatuses(
	ctx context.Context,
	registry *componentRegistry,
) (map[string]components.ComponentStatus, error) {
	statuses := make(map[string]components.ComponentStatus)
	for _, c := range registry.list() {
		componentStatus, err := c.Status(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get component %s status: %w", c.GetName(), err)
		}
		statuses[c.GetName()] = componentStatus
	}

	return statuses, nil
}

// componentsOrder is an order in which components will be built.
// The main rules are:
//   - if component A needs component B for building (running jobs, using yt client, etc), it
//     should be placed in some of the sections after component B section.
var componentsOrder = [][]consts.ComponentType{
	{
		// Discovery doesn't depend on anyone.
		consts.DiscoveryType,
		// Proxy are placed first since it is needed for ytsaurus client to work.
		consts.HttpProxyType,
	},
	{
		consts.YtsaurusClientType,
	},
	{
		// Master uses ytsaurus client when being updated, so if both are down,
		// ytsaurus client (and proxies) should be built first.
		consts.MasterType,
	},
	{
		consts.UIType,
		consts.RpcProxyType,
		consts.TcpProxyType,
		consts.DataNodeType,
		consts.MasterCacheType,
	},
	{
		consts.TabletNodeType,
		consts.ExecNodeType,
	},
	{
		consts.SchedulerType,
		consts.ControllerAgentType,
		consts.QueryTrackerType,
		consts.QueueAgentType,
		consts.YqlAgentType,
	},
	{
		consts.StrawberryControllerType,
	},
}

func syncComponents(
	ctx context.Context,
	registry *componentRegistry,
	resource *ytv1.Ytsaurus,
) (components.ComponentStatus, error) {
	statuses, err := getStatuses(ctx, registry)
	if err != nil {
		return components.ComponentStatus{}, err
	}
	logComponentStatuses(ctx, registry, statuses, componentsOrder, resource)

	var batchToSync []component
	for _, typesInBatch := range componentsOrder {
		compsInBatch := registry.listByType(typesInBatch...)
		for _, comp := range compsInBatch {
			status := statuses[comp.GetName()]
			if status.SyncStatus != components.SyncStatusReady && batchToSync == nil {
				batchToSync = compsInBatch
			}
		}
	}

	if batchToSync == nil {
		// YTsaurus is running and happy.
		return components.ComponentStatus{SyncStatus: components.SyncStatusReady}, nil
	}

	// Run sync for non-ready components in the batch.
	batchNotReadyStatuses := make(map[string]components.ComponentStatus)
	var errList []error
	for _, comp := range batchToSync {
		status := statuses[comp.GetName()]
		if status.SyncStatus == components.SyncStatusReady {
			continue
		}
		batchNotReadyStatuses[comp.GetName()] = status
		if err = comp.Sync(ctx); err != nil {
			errList = append(errList, fmt.Errorf("failed to sync %s: %w", comp.GetName(), err))
		}
	}

	if len(errList) != 0 {
		return components.ComponentStatus{}, errors.Join(errList...)
	}

	// Choosing the most important status for the batch to report up.
	batchStatus := components.ComponentStatus{
		SyncStatus: components.SyncStatusUpdating,
		Message:    "",
	}
	for compName, st := range batchNotReadyStatuses {
		if st.SyncStatus == components.SyncStatusBlocked {
			batchStatus.SyncStatus = components.SyncStatusBlocked
		}
		batchStatus.Message += fmt.Sprintf("; %s=%s (%s)", compName, st.SyncStatus, st.Message)
	}
	return batchStatus, nil
}
