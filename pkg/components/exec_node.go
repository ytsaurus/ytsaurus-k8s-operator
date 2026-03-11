package components

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/intstr"

	corev1 "k8s.io/api/core/v1"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type ExecNode struct {
	baseExecNode

	master Component
}

func NewExecNode(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	spec ytv1.ExecNodesSpec,
	yc internalYtsaurusClient,
) *ExecNode {
	l := cfgen.GetComponentLabeller(consts.ExecNodeType, spec.Name)

	srv := newServer(
		l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		[]ConfigGenerator{{
			"ytserver-exec-node.yson",
			ConfigFormatYson,
			func() ([]byte, error) { return cfgen.GetExecNodeConfig(spec) },
		}},
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
		ytsaurus,
		criConfig,
		ytsaurus.GetCommonSpec().ConfigOverrides,
	)

	if criConfig.MonitoringPort != 0 {
		srv.addMonitoringPort(corev1.ServicePort{
			Name:       consts.CRIServiceMonitoringPortName,
			Protocol:   corev1.ProtocolTCP,
			Port:       criConfig.MonitoringPort,
			TargetPort: intstr.FromInt32(criConfig.MonitoringPort),
		})
	}

	return &ExecNode{
		baseExecNode: baseExecNode{
			serverComponent: newLocalServerComponent(l, ytsaurus, srv),

			cfgen:          cfgen,
			criConfig:      criConfig,
			spec:           &spec,
			sidecarConfig:  sidecarConfig,
			ytsaurusClient: yc,
		},
		master: master,
	}
}

func (n *ExecNode) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if n.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := dispatchComponentUpdate(ctx, n.ytsaurus, n, &n.component, n.server, dry); status != nil {
			return *status, err
		}
		// Rolling update completed, re-enable scheduler jobs on all remaining pods of this STS.
		if !dry {
			n.enableSchedulerJobs(ctx)
		}
	}

	if masterStatus := n.master.GetStatus(); !masterStatus.IsRunning() {
		return masterStatus.Blocker(), nil
	}

	if n.NeedSync() {
		return n.doSyncBase(ctx, dry)
	}

	if !n.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	return ComponentStatusReady(), err
}

func (n *ExecNode) UpdatePreCheck(ctx context.Context) ComponentStatus {
	if _, status := getYtClient(n.ytsaurusClient); status != nil {
		return *status
	}

	strategy := getComponentUpdateStrategy(n.ytsaurus, n.GetType(), n.GetShortName())
	if strategy == ytv1.ComponentUpdateModeTypeRollingUpdate {
		return n.drainExecNodeForRollingUpdate(ctx)
	}
	return ComponentStatusReady()
}

// drainExecNodeForRollingUpdate drains only the exec node pod that is about to be updated
// and re-enables scheduler jobs on already-updated pods.
func (n *ExecNode) drainExecNodeForRollingUpdate(ctx context.Context) ComponentStatus {
	ytClient, status := getYtClient(n.ytsaurusClient)
	if status != nil {
		return *status
	}

	sts, ok := n.server.getRollingUpdateStatus(ctx)
	if !ok || sts.partition == nil || *sts.partition == 0 {
		return ComponentStatusReady()
	}

	partition := int(*sts.partition)
	totalCount := int(sts.totalCount)

	// Drain the pod about to be updated (at partition-1).
	if s := n.drainExecNode(ctx, ytClient, partition-1); s.SyncStatus != SyncStatusReady {
		return s
	}

	// Re-enable scheduler jobs on already-updated pods (ordinals >= partition).
	for i := partition; i < totalCount; i++ {
		if err := n.setExecNodeDisableSchedulerJobs(ctx, ytClient, i, false); err != nil {
			return ComponentStatusBlocked("Failed to re-enable scheduler jobs for ordinal %d: %v", i, err)
		}
	}

	return ComponentStatusReady()
}

// enableSchedulerJobs called after rolling update completion to clean up the last drained pod.
func (n *ExecNode) enableSchedulerJobs(ctx context.Context) {
	ytClient, status := getYtClient(n.ytsaurusClient)
	if status != nil {
		return
	}

	totalCount := int(n.server.getReplicaCount())
	for i := range totalCount {
		if err := n.setExecNodeDisableSchedulerJobs(ctx, ytClient, i, false); err != nil {
			return
		}
	}
}

func (n *ExecNode) execNodeInstancePath(ordinal int) ypath.Path {
	instanceAddress := n.GetLabeller().GetInstanceAddressPort(ordinal, consts.ExecNodeRPCPort)
	return ypath.Path(fmt.Sprintf("%s/%s", consts.ComponentCypressPath(consts.ExecNodeType), instanceAddress))
}

// drainExecNode disables scheduler jobs on the exec node at the given ordinal
// and checks whether its user_slots have reached 0.
func (n *ExecNode) drainExecNode(ctx context.Context, ytClient yt.Client, ordinal int) ComponentStatus {
	instancePath := n.execNodeInstancePath(ordinal)

	var userSlots int
	if err := ytClient.GetNode(ctx, ypath.Path(fmt.Sprintf("%s/@resource_usage/user_slots", instancePath)), &userSlots, nil); err != nil {
		return ComponentStatusBlocked("Failed to get user_slots for %s: %v", instancePath, err)
	}

	if userSlots == 0 {
		return ComponentStatusReady()
	}

	if err := n.setExecNodeDisableSchedulerJobs(ctx, ytClient, ordinal, true); err != nil {
		return ComponentStatusBlocked("Failed to set %s for %s: %v", consts.DisableSchedulerJobsAttr, instancePath, err)
	}

	return ComponentStatusBlocked("Exec node %s still has %d user slots in use", instancePath, userSlots)
}

// setExecNodeDisableSchedulerJobs sets the disable_scheduler_jobs attribute on the exec node
// at the given ordinal.
func (n *ExecNode) setExecNodeDisableSchedulerJobs(ctx context.Context, ytClient yt.Client, ordinal int, value bool) error {
	return ytClient.SetNode(ctx, n.execNodeInstancePath(ordinal).Attr(consts.DisableSchedulerJobsAttr), value, nil)
}
