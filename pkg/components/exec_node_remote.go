package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
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
	l := cfgen.GetComponentLabeller(consts.ExecNodeType, spec.Name)

	srv := newServerConfigured(
		l,
		proxy,
		commonSpec,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-exec-node.yson",
		func() ([]byte, error) {
			return cfgen.GetExecNodeConfig(spec)
		},
		cfgen,
		consts.ExecNodeMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.ExecNodeRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	var sidecarConfig *ConfigHelper
	if spec.JobEnvironment != nil && spec.JobEnvironment.CRI != nil {
		sidecarConfig = NewConfigHelper(
			l,
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
		baseComponent: baseComponent{labeller: l},
		baseExecNode: baseExecNode{
			server:        srv,
			cfgen:         cfgen,
			spec:          &spec,
			sidecarConfig: sidecarConfig,
		},
	}
}

func (n *RemoteExecNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if n.server.needSync() || n.server.needUpdate() || n.sidecarConfigNeedsReload() {
		return n.doSyncBase(ctx, dry)
	}

	if !n.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (n *RemoteExecNode) GetType() consts.ComponentType { return consts.ExecNodeType }

func (n *RemoteExecNode) Sync(ctx context.Context) (ComponentStatus, error) {
	return n.doSync(ctx, false)
}
