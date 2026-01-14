package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

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
		consts.ExecNodeMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.ExecNodeRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	criConfig := ytconfig.NewCRIConfigGenerator(&spec)
	sidecarConfig := NewJobsSidecarConfig(
		l,
		proxy,
		criConfig,
		commonSpec.ConfigOverrides,
	)

	if criConfig.MonitoringPort != 0 {
		srv.addMonitoringPort(corev1.ServicePort{
			Name:       consts.CRIServiceMonitoringPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       criConfig.MonitoringPort,
			TargetPort: intstr.FromInt32(criConfig.MonitoringPort),
		})
	}

	return &RemoteExecNode{
		baseComponent: baseComponent{labeller: l},
		baseExecNode: baseExecNode{
			server:        srv,
			cfgen:         cfgen,
			criConfig:     criConfig,
			spec:          &spec,
			sidecarConfig: sidecarConfig,
		},
	}
}

func (n *RemoteExecNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	needsSync, err := n.server.needSync()
	if err != nil {
		return ComponentStatusWaitingFor("components"), err
	}

	needsUpdate, err := n.server.needUpdate()
	if err != nil {
		return ComponentStatusWaitingFor("components"), err
	}

	if needsSync || needsUpdate || n.sidecarConfigNeedsReload() {
		return n.doSyncBase(ctx, dry)
	}

	if !n.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	return ComponentStatusReady(), err
}

func (n *RemoteExecNode) GetType() consts.ComponentType { return consts.ExecNodeType }

func (n *RemoteExecNode) Sync(ctx context.Context) (ComponentStatus, error) {
	return n.doSync(ctx, false)
}
