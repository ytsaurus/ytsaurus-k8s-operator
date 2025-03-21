package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type DataNode struct {
	localServerComponent
	cfgen  *ytconfig.NodeGenerator
	master Component
}

func NewDataNode(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	spec ytv1.DataNodesSpec,
) *DataNode {
	l := cfgen.GetComponentLabeller(consts.DataNodeType, spec.Name)

	srv := newServer(
		l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-data-node.yson",
		func() ([]byte, error) {
			return cfgen.GetDataNodeConfig(spec)
		},
		consts.DataNodeMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.DataNodeRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &DataNode{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               master,
	}
}

func (n *DataNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}

func (n *DataNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(n.ytsaurus.GetClusterState()) && n.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if n.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if n.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForImaginaryChunksAbsence {
			if status, err := n.handleImaginaryChunksMigration(ctx, dry); status != nil {
				return *status, err
			}
		}
		if status, err := handleUpdatingClusterState(ctx, n.ytsaurus, n, &n.localComponent, n.server, dry); status != nil {
			return *status, err
		}
	}

	masterStatus, err := n.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, n.master.GetFullName()), err
	}

	if n.NeedSync() {
		if !dry {
			err = n.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !n.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (n *DataNode) Status(ctx context.Context) (ComponentStatus, error) {
	return n.doSync(ctx, true)
}

func (n *DataNode) Sync(ctx context.Context) error {
	_, err := n.doSync(ctx, false)
	return err
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
			&n.localComponent,
		)
	}
	return ptr.To(WaitingStatus(SyncStatusUpdating, "pods removal for imaginary chunks migration")), err
}
