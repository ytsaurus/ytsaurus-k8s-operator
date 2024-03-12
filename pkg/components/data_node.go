package components

import (
	"context"
	"fmt"

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
		MonitoringPort: consts.DataNodeMonitoringPort,
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
	if n.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
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
	var err error

	// TODO: check if we remove pods BEFORE
	if n.server.needUpdate() {
		if err = removePods(ctx, n.server, &n.localComponent); err != nil {
			return fmt.Errorf("failed to remove pods: %w", err)
		}
	}

	if n.NeedSync() {
		if err = n.server.Sync(ctx); err != nil {
			return fmt.Errorf("failed to sync server: %w", err)
		}
	}

	return nil
}
