package components

import (
	"context"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type dataNodeBase struct {
	cfgen  *ytconfig.NodeGenerator
	server server
}

type DataNode struct {
	componentBase
	dataNodeBase
	master Component
}

type DataNodeRemote struct {
	componentBase
	dataNodeBase
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
		MonitoringPort: consts.DataNodeMonitoringPort,
	}

	server := newServer(
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
		componentBase: componentBase{
			labeller: &l,
			ytsaurus: ytsaurus,
		},
		dataNodeBase: dataNodeBase{
			cfgen:  cfgen,
			server: server,
		},
		master: master,
	}
}

func NewDataNodeRemote(
	cfgen *ytconfig.NodeGenerator,
	parentResource parentResource,
	spec ytv1.DataNodesSpec,
) *DataNodeRemote {
	meta := parentResource.GetObjectMeta()
	l := labeller.Labeller{
		ObjectMeta:     &meta,
		APIProxy:       parentResource.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelDataNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault("DataNode", spec.Name),
		MonitoringPort: consts.DataNodeMonitoringPort,
	}

	server := newServer(
		&l,
		parentResource,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-data-node.yson",
		cfgen.GetDataNodesStatefulSetName(spec.Name),
		cfgen.GetDataNodesServiceName(spec.Name),
		func() ([]byte, error) {
			return cfgen.GetDataNodeConfig(spec)
		},
	)

	return &DataNodeRemote{
		dataNodeBase: dataNodeBase{
			cfgen:  cfgen,
			server: server,
		},
	}
}

func (n *DataNode) IsUpdatable() bool {
	return true
}

func (n *dataNodeBase) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}

func (n *DataNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(n.ytsaurus.GetClusterState()) && n.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate), err
	}

	if n.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, n.ytsaurus, n, &n.componentBase, n.server, dry); status != nil {
			return *status, err
		}
	}

	if !IsRunningStatus(n.master.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, n.master.GetName()), err
	}

	return n.doSyncBase(ctx, dry)
}

func (n *dataNodeBase) doSyncBase(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if n.server.needSync() {
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

func (n *DataNode) Status(ctx context.Context) ComponentStatus {
	status, err := n.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (n *DataNode) Sync(ctx context.Context) error {
	_, err := n.doSync(ctx, false)
	return err
}

func (n *DataNodeRemote) Status(ctx context.Context) ComponentStatus {
	status, err := n.doSyncBase(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (n *DataNodeRemote) Sync(ctx context.Context) error {
	_, err := n.doSyncBase(ctx, false)
	return err
}
