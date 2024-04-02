package components

import (
	"context"

	"go.ytsaurus.tech/library/go/ptr"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
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
	resource := ytsaurus.GetResource()

	l := labeller.NewMultiComponentLabeller(
		&resource.ObjectMeta,
		consts.ExecNodeType,
		consts.YTComponentLabelExecNode,
		spec.Name,
		ytsaurus.GetCommonSpec().ExtraPodAnnotations,
	)

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.Int32(consts.ExecNodeMonitoringPort)
	}

	srv := newServer(
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
	)

	var sidecarConfig *ConfigHelper
	if spec.JobEnvironment != nil && spec.JobEnvironment.CRI != nil {
		sidecarConfig = NewConfigHelper(
			&l,
			ytsaurus.APIProxy(),
			l.GetSidecarConfigMapName(consts.JobsContainerName),
			ytsaurus.GetResource().Spec.ConfigOverrides,
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
		localComponent: newLocalComponent(&l, ytsaurus),
		baseExecNode: baseExecNode{
			server:        srv,
			cfgen:         cfgen,
			spec:          &spec,
			sidecarConfig: sidecarConfig,
		},
		master: master,
	}
}

func (n *ExecNode) IsUpdatable() bool {
	return true
}

func (n *ExecNode) GetType() consts.ComponentType { return consts.ExecNodeType }

func (n *ExecNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(n.ytsaurus.GetClusterState()) && n.server.needUpdate() {
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
		return WaitingStatus(SyncStatusBlocked, n.master.GetName()), err
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
