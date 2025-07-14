package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type RemoteTabletNode struct {
	server server
	cfgen  *ytconfig.NodeGenerator
	spec   *ytv1.TabletNodesSpec
	baseComponent
}

func NewRemoteTabletNodes(
	cfgen *ytconfig.NodeGenerator,
	nodes *ytv1.RemoteTabletNodes,
	proxy apiproxy.APIProxy,
	spec ytv1.TabletNodesSpec,
	commonSpec ytv1.CommonSpec,
) *RemoteTabletNode {
	l := cfgen.GetComponentLabeller(consts.TabletNodeType, spec.Name)

	srv := newServerConfigured(
		l,
		proxy,
		commonSpec,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-tablet-node.yson",
		func() ([]byte, error) {
			return cfgen.GetTabletNodeConfig(spec)
		},
		cfgen,
		&ytv1.YtsaurusSpec{},
		consts.TabletNodeMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.TabletNodeRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)
	return &RemoteTabletNode{
		baseComponent: baseComponent{labeller: l},
		server:        srv,
		cfgen:         cfgen,
		spec:          &spec,
	}
}

func (n *RemoteTabletNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if n.server.needSync() || n.server.needUpdate() {
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

func (n *RemoteTabletNode) GetType() consts.ComponentType { return consts.TabletNodeType }

func (n *RemoteTabletNode) Sync(ctx context.Context) (ComponentStatus, error) {
	return n.doSync(ctx, false)
}

func (n *RemoteTabletNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}
