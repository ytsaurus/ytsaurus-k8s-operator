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
	serverComponent

	cfgen *ytconfig.NodeGenerator
	spec  *ytv1.TabletNodesSpec
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
		&commonSpec,
		&ytv1.PodSpec{},
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-tablet-node.yson",
		func() ([]byte, error) {
			return cfgen.GetTabletNodeConfig(spec)
		},
		consts.TabletNodeMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.TabletNodeRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)
	return &RemoteTabletNode{
		serverComponent: newServerComponent(l, proxy, srv),

		cfgen: cfgen,
		spec:  &spec,
	}
}

func (n *RemoteTabletNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

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

func (n *RemoteTabletNode) Sync(ctx context.Context) (ComponentStatus, error) {
	return n.doSync(ctx, false)
}

func (n *RemoteTabletNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}
