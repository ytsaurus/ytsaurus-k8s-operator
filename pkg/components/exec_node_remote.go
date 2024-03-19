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

type RemoteExecNode struct {
	baseExecNode
	baseComponent
}

func NewRemoteExecNodes(
	cfgen *ytconfig.NodeGenerator,
	nodes *ytv1.RemoteExecNodes,
	proxy apiproxy.APIProxy,
	spec ytv1.ExecNodesSpec,
	commonSpec ytv1.CommonSpec,
) *RemoteExecNode {
	l := labeller.Labeller{
		ObjectMeta:     &nodes.ObjectMeta,
		APIProxy:       proxy,
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelExecNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault("ExecNode", spec.Name),
	}

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.Int32(consts.ExecNodeMonitoringPort)
	}

	srv := newServerConfigured(
		&l,
		proxy,
		commonSpec,
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
		baseComponent: baseComponent{labeller: &l},
		baseExecNode: baseExecNode{
			server:     srv,
			cfgen:      cfgen,
			sidecars:   spec.Sidecars,
			privileged: spec.Privileged,
		},
	}
}

func (n *RemoteExecNode) Sync(ctx context.Context) (ComponentStatus, error) {
	var err error

	if n.server.needSync() || n.server.needUpdate() {
		return WaitingStatus(SyncStatusPending, "components"), n.SyncBase(ctx)
	}

	if !n.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}
