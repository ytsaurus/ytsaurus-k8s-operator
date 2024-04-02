package components

import (
	"context"

	"go.ytsaurus.tech/library/go/ptr"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
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
	resource := ytsaurus.GetResource()

	l := labeller.NewMultiComponentLabeller(
		&resource.ObjectMeta,
		consts.DataNodeType,
		consts.YTComponentLabelDataNode,
		spec.Name,
		ytsaurus.GetCommonSpec().ExtraPodAnnotations,
	)

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.Int32(consts.DataNodeMonitoringPort)
	}

	srv := newServer(
		&l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-data-node.yson",
		cfgen.GetDataNodesStatefulSetName(spec.Name),
		cfgen.GetDataNodesServiceName(spec.Name),
		func() ([]byte, error) {
			return cfgen.GetDataNodeConfig(spec)
		},
	)

	return &DataNode{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               master,
	}
}

func (n *DataNode) IsUpdatable() bool {
	return true
}

func (n *DataNode) GetType() consts.ComponentType { return consts.DataNodeType }

func (n *DataNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}

func (n *DataNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(n.ytsaurus.GetClusterState()) && n.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if n.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, n.ytsaurus, n, &n.localComponent, n.server, dry); status != nil {
			return *status, err
		}
	}

	masterStatus, err := n.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, n.master.GetName()), err
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
