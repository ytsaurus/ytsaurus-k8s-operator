package components

import (
	"context"

	"k8s.io/apimachinery/pkg/util/intstr"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type ExecNode struct {
	baseExecNode
	localComponent
	master Component
}

func NewExecNode(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	spec ytv1.ExecNodesSpec,
) *ExecNode {
	l := cfgen.GetComponentLabeller(consts.ExecNodeType, spec.Name)

	srv := newServer(
		l,
		ytsaurus,
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

	var sidecarConfig *ConfigMapBuilder
	if spec.JobEnvironment != nil && spec.JobEnvironment.CRI != nil {
		sidecarConfig = NewConfigMapBuilder(
			l,
			ytsaurus.APIProxy(),
			l.GetSidecarConfigMapName(consts.JobsContainerName),
			ytsaurus.GetResource().Spec.ConfigOverrides,
		)

		sidecarConfig.AddGenerator(
			consts.ContainerdConfigFileName,
			ConfigFormatToml,
			func() ([]byte, error) {
				return cfgen.GetContainerdConfig(&spec)
			},
		)

		if port := ytconfig.GetCRIServiceMonitoringPort(&spec); port != 0 {
			srv.addMonitoringPort(corev1.ServicePort{
				Name:       consts.CRIServiceMonitoringPortName,
				Protocol:   corev1.ProtocolTCP,
				Port:       port,
				TargetPort: intstr.FromInt32(port),
			})
		}
	}

	return &ExecNode{
		localComponent: newLocalComponent(l, ytsaurus),
		baseExecNode: baseExecNode{
			server:        srv,
			cfgen:         cfgen,
			spec:          &spec,
			sidecarConfig: sidecarConfig,
		},
		master: master,
	}
}

func (n *ExecNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(n.ytsaurus.GetClusterState()) && (n.server.needUpdate() || n.sidecarConfigNeedsReload()) {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if n.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, n.ytsaurus, n, &n.localComponent, n.server, dry); status != nil {
			return *status, err
		}
	}

	masterStatus, err := n.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, n.master.GetFullName()), err
	}

	if LocalServerNeedSync(n.server, n.ytsaurus) {
		return n.doSyncBase(ctx, dry)
	}

	if !n.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (n *ExecNode) Status(ctx context.Context) (ComponentStatus, error) {
	return n.doSync(ctx, true)
}

func (n *ExecNode) Sync(ctx context.Context) error {
	_, err := n.doSync(ctx, false)
	return err
}
