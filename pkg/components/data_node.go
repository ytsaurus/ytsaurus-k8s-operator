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
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelDataNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault("DataNode", spec.Name),
	}

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

func (n *DataNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}

func (n *DataNode) Status(ctx context.Context) ComponentStatus {
	if ytv1.IsReadyToUpdateClusterState(n.ytsaurus.GetClusterState()) && n.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate)
	}

	if n.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, n.ytsaurus, n, &n.localComponent, n.server, true)
		if status != nil {
			if err != nil {
				panic(err)
			}
			return *status
		}
	}

	if !IsRunningStatus(n.master.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, n.master.GetName())
	}

	if n.NeedSync() {
		return WaitingStatus(SyncStatusPending, "components")
	}

	if !n.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods")
	}

	return SimpleStatus(SyncStatusReady)
}

func (n *DataNode) Sync(ctx context.Context) error {
	if ytv1.IsReadyToUpdateClusterState(n.ytsaurus.GetClusterState()) && n.server.needUpdate() {
		return nil
	}

	if n.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, n.ytsaurus, n, &n.localComponent, n.server, false)
		if status != nil {
			return err
		}
	}

	if !IsRunningStatus(n.master.Status(ctx).SyncStatus) {
		return nil
	}

	if n.NeedSync() {
		err := n.server.Sync(ctx)
		return err
	}

	if !n.server.arePodsReady(ctx) {
		return nil
	}

	return nil
}
