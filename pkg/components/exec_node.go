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

type ExecNode struct {
	localServerComponent
	baseExecNode
	master Component
}

var _ LocalServerComponent = &ExecNode{}

func NewExecNode(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	spec ytv1.ExecNodesSpec,
) *ExecNode {
	resource := ytsaurus.Resource()
	l := labeller.Labeller{
		ObjectMeta:        &resource.ObjectMeta,
		ComponentType:     consts.ExecNodeType,
		ComponentNamePart: spec.Name,
	}

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.To(int32(consts.ExecNodeMonitoringPort))
	}

	server := newServer(
		&l,
		ytsaurus,
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
			ytsaurus.APIProxy(),
			l.GetSidecarConfigMapName(consts.JobsContainerName),
			ytsaurus.Resource().Spec.ConfigOverrides,
			map[string]ytconfig.GeneratorDescriptor{
				consts.ContainerdConfigFileName: {
					F: func() ([]byte, error) {
						return cfgen.GetContainerdConfig(&spec)
					},
					Fmt: ytconfig.ConfigFormatToml,
				},
			})
	}

	return &ExecNode{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, server),
		baseExecNode: baseExecNode{
			server:        server,
			cfgen:         cfgen,
			spec:          &spec,
			sidecarConfig: sidecarConfig,
		},
		master: master,
	}
}

func (n *ExecNode) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(n.ytsaurus.GetClusterState()) && n.localServerComponent.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if n.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, n, dry); status != nil {
			return *status, err
		}
	}

	if status, err := checkComponentDependency(ctx, n.master); status != nil {
		return *status, err
	}

	server := n.localServerComponent.server
	if ServerNeedSync(server, n.ytsaurus) {
		return n.doSyncBase(ctx, dry)
	}

	if !server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}
