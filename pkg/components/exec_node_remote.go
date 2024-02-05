package components

import (
	"context"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type RemoteExecNode struct {
	baseExecNode
	labellable
}

func NewRemoteExecNodes(
	cfgen *ytconfig.NodeGenerator,
	nodes *ytv1.RemoteExecNodes,
	proxy apiproxy.APIProxy,
	spec ytv1.ExecNodesSpec,
	configSpec ytv1.ConfigurationSpec,
) *RemoteExecNode {
	l := labeller.Labeller{
		ObjectMeta:     &nodes.ObjectMeta,
		APIProxy:       proxy,
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelExecNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault("ExecNode", spec.Name),
		MonitoringPort: consts.ExecNodeMonitoringPort,
	}
	srv := newServerConfigured(
		&l,
		proxy,
		configSpec,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-exec-node.yson",
		cfgen.GetExecNodesStatefulSetName(spec.Name),
		cfgen.GetExecNodesServiceName(spec.Name),
		func() ([]byte, error) {
			return cfgen.GetExecNodeConfig(spec)
		},
	)
	return &RemoteExecNode{
		labellable: labellable{labeller: &l},
		baseExecNode: baseExecNode{
			server:     srv,
			cfgen:      cfgen,
			sidecars:   spec.Sidecars,
			privileged: spec.Privileged,
		},
	}
}

func (n *RemoteExecNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if n.server.needSync() || n.server.needUpdate() {
		return n.doSyncBase(ctx, dry)
	}

	if !n.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (n *RemoteExecNode) Sync(ctx context.Context) (ComponentStatus, error) {
	return n.doSync(ctx, false)
}
