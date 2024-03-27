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
	// At first, we check if master is *built* (not updated) before everything else.
	{
		consts.YtsaurusClientType,
		consts.DiscoveryType,
		consts.HttpProxyType,
		consts.RpcProxyType,
		consts.TcpProxyType,
		consts.DataNodeType,
		consts.ExecNodeType,
		consts.MasterCacheType,
	},
	{
		consts.TabletNodeType,
		consts.UIType,
		consts.ControllerAgentType,
		consts.YqlAgentType,
	},
	{
		consts.SchedulerType,
		consts.QueryTrackerType,
		consts.QueueAgentType,
	},
	{
		consts.StrawberryControllerType,
	},
	{
		// Here we UPDATE master after all the components, because it shouldn't be newer
		// than others.
		// Currently, we guarantee that only for the case when components are not redefine their images.
		consts.MasterType,
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

	// Special check before everything other component (including master) update.
	masterBuildStatus := getStatusForMasterBuild(registry.master)
	switch masterBuildStatus.SyncStatus {
	case components.SyncStatusBlocked:
		return masterBuildStatus, nil
	case components.SyncStatusNeedSync:
		return masterBuildStatus, registry.master.BuildInitial(ctx)
	}

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

func getStatusForMasterBuild(master masterComponent) components.ComponentStatus {
	masterBuiltInitially := master.IsBuildInitially()
	masterNeedBuild := master.NeedBuild()
	masterRebuildStarted := master.IsRebuildStarted()

	if !masterBuiltInitially {
		// This only happens once on cluster initialization.
		return components.NeedSyncStatus("master initial build")
	}

	if masterNeedBuild && !masterRebuildStarted {
		// Not all the master's sub-resources are running, and it is NOT because master is in update stage
		// (in which is reasonable to expect some not-yet-built sub-resources).
		// So we can't proceed with update, because almost every component need working master to be updated properly.
		return components.ComponentStatus{
			SyncStatus: components.SyncStatusBlocked,
			Message:    "Master is not built, cluster can't start the update",
		}
	}
	return components.ReadyStatus()
}
