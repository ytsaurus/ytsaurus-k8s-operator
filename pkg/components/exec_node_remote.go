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
		ComponentType:  consts.YTComponentLabelExecNode,
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelExecNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault(string(consts.ExecNodeType), spec.Name),
	}

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.To(int32(consts.ExecNodeMonitoringPort))
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
		baseComponent: baseComponent{labeller: &l},
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

	if n.server.needSync() || n.server.needUpdate() {
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
