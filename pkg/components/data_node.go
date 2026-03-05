package components

import (
	"context"
	"fmt"
	"sort"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type DataNode struct {
	serverComponent

	cfgen          *ytconfig.NodeGenerator
	master         Component
	ytsaurusClient internalYtsaurusClient
}

type dataNodeCounterCheck struct {
	path ypath.Path
	name string
}

func NewDataNode(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	yc internalYtsaurusClient,
	spec ytv1.DataNodesSpec,
) *DataNode {
	l := cfgen.GetComponentLabeller(consts.DataNodeType, spec.Name)

	srv := newServer(
		l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		[]ConfigGenerator{{
			"ytserver-data-node.yson",
			ConfigFormatYson,
			func() ([]byte, error) { return cfgen.GetDataNodeConfig(spec) },
		}},
		consts.DataNodeMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.DataNodeRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &DataNode{
		serverComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:           cfgen,
		master:          master,
		ytsaurusClient:  yc,
	}
}

func (n *DataNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}

func (n *DataNode) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if n.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if n.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForImaginaryChunksAbsence {
			if status, err := n.handleImaginaryChunksMigration(ctx, dry); status != nil {
				return *status, err
			}
		}
		if IsUpdatingComponent(n.ytsaurus, n) {
			switch getComponentUpdateStrategy(n.ytsaurus, consts.DataNodeType, n.GetShortName()) {
			case ytv1.ComponentUpdateModeTypeRollingUpdate:
				if status, err := n.handleRackRollingUpdatingClusterState(ctx, dry); status != nil {
					return *status, err
				}
			default:
				if status, err := handleBulkUpdatingClusterState(ctx, n.ytsaurus, n, &n.component, n.server, dry); status != nil {
					return *status, err
				}
			}
		} else {
			return ComponentStatusReadyAfter("Not updating component"), nil
		}
	}

	if masterStatus := n.master.GetStatus(); !masterStatus.IsRunning() {
		return ComponentStatusBlockedBy(n.master.GetFullName()), nil
	}

	if n.NeedSync() {
		if !dry {
			err = n.server.Sync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if !n.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	return ComponentStatusReady(), err
}

func (n *DataNode) handleRackRollingUpdatingClusterState(ctx context.Context, dry bool) (*ComponentStatus, error) {
	if n.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsRemoval {
		return nil, nil
	}

	// if we have a specific dataNode in the updatePlan (with Name) - run it through regular rolling update.
	if !usesUnnamedDataNodesRollingSelector(n.ytsaurus) {
		if status, err := handleRollingUpdatingClusterState(ctx, n.ytsaurus, n, n.server, dry); status != nil {
			return status, err
		}
		return nil, nil
	}

	activeRack, found := getActiveDataNodeRollingRack(n.ytsaurus, n.GetLabeller())
	if !found {
		return nil, nil
	}

	// skip all racks except active one
	if n.GetShortName() != activeRack {
		return ptr.To(ComponentStatusReadyAfter(fmt.Sprintf("Waiting for data-node rack %q rolling update", activeRack))), nil
	}

	if dry {
		return ptr.To(ComponentStatusUpdateStep("data-node rack rolling update")), nil
	}

	if !isPodsRemovingStarted(&n.component) {
		if status, err := runPrechecks(ctx, n.ytsaurus, n); status != nil {
			return status, err
		}

		if err := n.server.removePods(ctx); err != nil {
			return ptr.To(ComponentStatusBlocked("failed to start data-node rack pods removal")), err
		}
		setPodsRemovingStartedCondition(ctx, &n.component)
		return ptr.To(ComponentStatusUpdateStep("data-node rack pods removal")), nil
	}

	if !n.ytsaurus.IsUpdateStatusConditionTrue(n.component.labeller.GetPodsRemovedCondition()) {
		if !n.server.arePodsRemoved(ctx) {
			return ptr.To(ComponentStatusUpdateStep("data-node rack pods removal")), nil
		}
		setPodsRemovedCondition(ctx, &n.component)
	}

	if n.server.needSync(false) {
		if err := n.server.Sync(ctx); err != nil {
			return ptr.To(ComponentStatusBlocked("failed to recreate data-node rack pods")), err
		}
		return ptr.To(ComponentStatusUpdateStep("data-node rack pods creation")), nil
	}

	if !n.server.arePodsReady(ctx) || !n.server.arePodsUpdatedToNewRevision(ctx) {
		return ptr.To(ComponentStatusUpdateStep("data-node rack rolling update")), nil
	}

	setPodsUpdatedCondition(ctx, n.ytsaurus, n)
	return nil, nil
}

func usesUnnamedDataNodesRollingSelector(ytsaurus *apiproxy.Ytsaurus) bool {
	for _, selector := range ytsaurus.GetResource().Spec.UpdatePlan {
		if selector.Component.Type != consts.DataNodeType {
			continue
		}
		if selector.Component.Name != "" {
			continue
		}
		if selector.Strategy == nil || selector.Strategy.RollingUpdate == nil {
			continue
		}
		return true
	}
	return false
}

// getActiveDataNodeRollingRack answers question which rack should be rolling right now
// by finding all racks that are still pending.
// we need bool in return in order to handle case when dnd name is default, it will be populated as "" later by yhe updatingComponents
func getActiveDataNodeRollingRack(ytsaurus *apiproxy.Ytsaurus, baseLabeller *labeller.Labeller) (string, bool) {
	var pendingRacks []string
	for _, component := range ytsaurus.GetUpdatingComponents() {
		if component.Type != consts.DataNodeType {
			continue
		}

		conditionType := baseLabeller.ForComponent(consts.DataNodeType, component.Name).GetPodsUpdatedCondition()
		if ytsaurus.IsUpdateStatusConditionTrue(conditionType) {
			continue
		}
		pendingRacks = append(pendingRacks, component.Name)
	}

	if len(pendingRacks) == 0 {
		return "", false
	}

	sort.Strings(pendingRacks)
	return pendingRacks[0], true
}

func (n *DataNode) UpdatePreCheck(ctx context.Context) ComponentStatus {
	var dataNodeRollingCounterChecks = [...]dataNodeCounterCheck{
		{path: ypath.Path(consts.LostVitalChunksCountPath), name: "lost vital chunks"},
		{path: ypath.Path(consts.ParityMissingChunksCountPath), name: "parity missing chunks"},
		{path: ypath.Path(consts.DataMissingChunksCountPath), name: "data missing chunks"},
		{path: ypath.Path(consts.UnsafelyPlacedChunksCountPath), name: "unsafely placed chunks"},
		{path: ypath.Path(consts.QuorumMissingChunksCountPath), name: "quorum missing chunks"},
	}

	if n.ytsaurusClient == nil {
		return ComponentStatusBlocked("YtsaurusClient component is not available")
	}
	ytClient := n.ytsaurusClient.GetYtClient()
	if ytClient == nil {
		return ComponentStatusBlocked("YT client is not available")
	}

	for _, check := range dataNodeRollingCounterChecks {
		if status := checkDataNodeCounter(ctx, ytClient, check); status.SyncStatus != SyncStatusReady {
			return status
		}
	}

	return ComponentStatusReady()
}

func checkDataNodeCounter(ctx context.Context, ytClient yt.Client, check dataNodeCounterCheck) ComponentStatus {
	count := 0
	if err := ytClient.GetNode(ctx, check.path, &count, nil); err != nil {
		return ComponentStatusBlocked(fmt.Sprintf("failed to get %s count: %v", check.name, err))
	}
	if count > 0 {
		return ComponentStatusBlocked(fmt.Sprintf("there are %s: %v", check.name, count))
	}
	return ComponentStatusReady()
}

// handleImaginaryChunksMigration will remove dnd pods if client component detects active dnds with imaginary chunks
// and sets relevant condition.
// Nodes pods will be recreated (and updated if needed) later at UpdateStateWaitingForPodsCreation stage.
// After that they will be re-registered in master with real chunks, since client component will set an attribute in cypress.
func (n *DataNode) handleImaginaryChunksMigration(ctx context.Context, dry bool) (*ComponentStatus, error) {
	if !n.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionDataNodesNeedPodsRemoval) {
		// nothing to do, either no imaginary chunks is detected or condition to remove pods is not set yet
		return nil, nil
	}
	// https://github.com/ytsaurus/ytsaurus-k8s-operator/issues/396
	var err error
	if !dry {
		err = removePods(
			ctx,
			n.server,
			&n.component,
		)
	}
	return ptr.To(ComponentStatusUpdateStep("pods removal for imaginary chunks migration")), err
}
