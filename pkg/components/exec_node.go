package components

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	corev1 "k8s.io/api/core/v1"

	"go.ytsaurus.tech/yt/go/ypath"

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
		// Rolling update completed, re-enable scheduler jobs on all pods of this STS that were drained.
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
func (n *ExecNode) drainExecNodeForRollingUpdate(ctx context.Context) ComponentStatus {
	sts, ok := n.server.getRollingUpdateStatus(ctx)
	if !ok || sts.partition == nil || *sts.partition == 0 {
		return ComponentStatusReady()
	}

	podNameToDrain := fmt.Sprintf("%s-%d", n.GetLabeller().GetServerStatefulSetName(), *sts.partition-1)

	return n.setDisableSchedulerJobs(ctx, podNameToDrain)
}

func (n *ExecNode) enableSchedulerJobs(ctx context.Context) {
	// Empty podNameToDrain means re-enable all instances without draining.
	_ = n.setDisableSchedulerJobs(ctx, "")
}

// setDisableSchedulerJobs iterates over exec node instances in Cypress.
// If podNameToDrain is non-empty, the matching instance is drained (disable_scheduler_jobs=true,
// wait for user_slots=0)
func (n *ExecNode) setDisableSchedulerJobs(ctx context.Context, podNameToDrain string) ComponentStatus {
	ytClient, status := getYtClient(n.ytsaurusClient)
	if status != nil {
		return *status
	}
	stsName := n.GetLabeller().GetServerStatefulSetName()
	cypressPath := consts.ComponentCypressPath(consts.ExecNodeType)

	var instances []string
	if err := ytClient.ListNode(ctx, ypath.Path(cypressPath), &instances, nil); err != nil {
		return ComponentStatusBlocked("Failed to list exec node instances: %v", err)
	}

	for _, instance := range instances {
		instancePath := fmt.Sprintf("%s/%s", cypressPath, instance)

		if podNameToDrain != "" && strings.HasPrefix(instance, podNameToDrain+".") {
			var userSlots int
			if err := ytClient.GetNode(ctx, ypath.Path(fmt.Sprintf("%s/@resource_usage/user_slots", instancePath)), &userSlots, nil); err != nil {
				return ComponentStatusBlocked("Failed to get user_slots for %s: %v", instance, err)
			}

			if userSlots == 0 {
				continue
			}

			if err := ytClient.SetNode(ctx, ypath.Path(instancePath).Attr(consts.DisableSchedulerJobsAttr), true, nil); err != nil {
				return ComponentStatusBlocked("Failed to set %s for %s: %v", consts.DisableSchedulerJobsAttr, instance, err)
			}

			return ComponentStatusBlocked("Exec node %s still has %d user slots in use", instance, userSlots)
		}

		// Re-enable scheduler jobs on other pods of this StatefulSet.
		if strings.HasPrefix(instance, stsName+"-") {
			if err := ytClient.SetNode(ctx, ypath.Path(instancePath).Attr(consts.DisableSchedulerJobsAttr), false, nil); err != nil {
				return ComponentStatusBlocked("Failed to set %s for %s: %v", consts.DisableSchedulerJobsAttr, instance, err)
			}
		}
	}

	return ComponentStatusReady()
}
