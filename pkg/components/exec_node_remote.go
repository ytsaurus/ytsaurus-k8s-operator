package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type RemoteExecNode struct {
	remoteServerComponent
	baseExecNode
}

var _ RemoteServerComponent = &RemoteExecNode{}

func NewRemoteExecNodes(
	cfgen *ytconfig.NodeGenerator,
	nodes *ytv1.RemoteExecNodes,
	proxy apiproxy.APIProxy,
	spec ytv1.ExecNodesSpec,
	commonSpec ytv1.CommonSpec,
) *RemoteExecNode {
	l := labeller.Labeller{
		ObjectMeta:        &nodes.ObjectMeta,
		ComponentType:     consts.ExecNodeType,
		ComponentNamePart: spec.Name,
	}

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.To(int32(consts.ExecNodeMonitoringPort))
	}

	server := newServerConfigured(
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
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.ExecNodeRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	var sidecarConfig *ConfigHelper
	if spec.JobEnvironment != nil && spec.JobEnvironment.CRI != nil {
		sidecarConfig = NewConfigHelper(
			&l,
			proxy,
			l.GetSidecarConfigMapName(consts.JobsContainerName),
			commonSpec.ConfigOverrides,
			map[string]ytconfig.GeneratorDescriptor{
				consts.ContainerdConfigFileName: {
					F: func() ([]byte, error) {
						return cfgen.GetContainerdConfig(&spec)
					},
					Fmt: ytconfig.ConfigFormatToml,
				},
			})
	}

	return &RemoteExecNode{
		remoteServerComponent: newRemoteServerComponent(proxy, &l, server),
		baseExecNode: baseExecNode{
			server:        server,
			cfgen:         cfgen,
			spec:          &spec,
			sidecarConfig: sidecarConfig,
		},
	}
}

func (n *RemoteExecNode) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	server := n.remoteServerComponent.server
	if server.configNeedsReload() || server.needBuild() || server.needUpdate() {
		return n.doSyncBase(ctx, dry)
	}

	if !server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}
