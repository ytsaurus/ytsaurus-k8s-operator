package components

import (
	"context"

	"go.ytsaurus.tech/library/go/ptr"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type ExecNode struct {
	baseExecNode
	localComponent
	master Component
}

func NewExecNode(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	spec ytv1.ExecNodesSpec,
) *ExecNode {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelExecNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault("ExecNode", spec.Name),
	}

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.Int32(consts.ExecNodeMonitoringPort)
	}

	srv := newServer(
		&l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-exec-node.yson",
		cfgen.GetExecNodesStatefulSetName(spec.Name),
		cfgen.GetExecNodesServiceName(spec.Name),
		func() ([]byte, error) {
			return cfgen.GetExecNodeConfig(spec)
		},
	)

	return &ExecNode{
		localComponent: newLocalComponent(&l, ytsaurus),
		baseExecNode: baseExecNode{
			server:     srv,
			cfgen:      cfgen,
			sidecars:   spec.Sidecars,
			privileged: spec.Privileged,
		},
		master: master,
	}
}

func (n *ExecNode) IsUpdatable() bool {
	return true
}

func (n *ExecNode) Status(ctx context.Context) ComponentStatus {
	if ytv1.IsReadyToUpdateClusterState(n.ytsaurus.GetClusterState()) && n.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
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

	if LocalServerNeedSync(n.server, n.ytsaurus) {
		return WaitingStatus(SyncStatusPending, "components")
	}

	if !n.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods")
	}

	return SimpleStatus(SyncStatusReady)
}

func (n *ExecNode) Sync(ctx context.Context) error {
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

	if LocalServerNeedSync(n.server, n.ytsaurus) {
		return n.SyncBase(ctx)
	}

	if !n.server.arePodsReady(ctx) {
		return nil
	}

	return nil
}
