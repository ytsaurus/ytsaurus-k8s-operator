package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type RemoteDataNode struct {
	remoteServerComponent
	cfgen *ytconfig.NodeGenerator
	spec  *ytv1.DataNodesSpec
}

var _ RemoteServerComponent = &RemoteDataNode{}

func NewRemoteDataNodes(
	cfgen *ytconfig.NodeGenerator,
	nodes *ytv1.RemoteDataNodes,
	apiProxy apiproxy.APIProxy,
	spec ytv1.DataNodesSpec,
	commonSpec ytv1.CommonSpec,
) *RemoteDataNode {
	l := labeller.Labeller{
		ObjectMeta:        &nodes.ObjectMeta,
		ComponentType:     consts.DataNodeType,
		ComponentNamePart: spec.Name,
	}

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.To(int32(consts.DataNodeMonitoringPort))
	}

	server := newServerConfigured(
		&l,
		apiProxy,
		commonSpec,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-data-node.yson",
		cfgen.GetDataNodesStatefulSetName(spec.Name),
		cfgen.GetDataNodesServiceName(spec.Name),
		func() ([]byte, error) {
			return cfgen.GetDataNodeConfig(spec)
		},
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.DataNodeRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)
	return &RemoteDataNode{
		remoteServerComponent: newRemoteServerComponent(apiProxy, &l, server),
		cfgen:                 cfgen,
		spec:                  &spec,
	}
}

func (n *RemoteDataNode) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if n.server.configNeedsReload() || n.server.needBuild() || n.server.needUpdate() {
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

func (n *RemoteDataNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}
