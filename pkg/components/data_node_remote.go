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

type RemoteDataNode struct {
	server server
	cfgen  *ytconfig.NodeGenerator
	spec   *ytv1.DataNodesSpec
	baseComponent
}

func NewRemoteDataNodes(
	cfgen *ytconfig.NodeGenerator,
	nodes *ytv1.RemoteDataNodes,
	proxy apiproxy.APIProxy,
	spec ytv1.DataNodesSpec,
	commonSpec ytv1.CommonSpec,
) *RemoteDataNode {
	l := cfgen.GetComponentLabeller(consts.DataNodeType, spec.Name)

	srv := newServerConfigured(
		l,
		proxy,
		commonSpec,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-data-node.yson",
		func() ([]byte, error) {
			return cfgen.GetDataNodeConfig(spec)
		},
		cfgen,
		consts.DataNodeMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.DataNodeRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)
	return &RemoteDataNode{
		baseComponent: baseComponent{labeller: l},
		server:        srv,
		cfgen:         cfgen,
		spec:          &spec,
	}
}

func (n *RemoteDataNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
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

func (n *RemoteDataNode) GetType() consts.ComponentType { return consts.DataNodeType }

func (n *RemoteDataNode) Sync(ctx context.Context) (ComponentStatus, error) {
	return n.doSync(ctx, false)
}

func (n *RemoteDataNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}
