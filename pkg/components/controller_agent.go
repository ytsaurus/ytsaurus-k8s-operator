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

type ControllerAgent struct {
	localServerComponent
	cfgen  *ytconfig.Generator
	master Component
}

func NewControllerAgent(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, master Component) *ControllerAgent {
	resource := ytsaurus.Resource()
	l := labeller.Labeller{
		ObjectMeta:    &resource.ObjectMeta,
		ComponentType: consts.ControllerAgentType,
	}

	if resource.Spec.ControllerAgents.InstanceSpec.MonitoringPort == nil {
		resource.Spec.ControllerAgents.InstanceSpec.MonitoringPort = ptr.To(int32(consts.ControllerAgentMonitoringPort))
	}

	srv := newServer(
		&l,
		ytsaurus,
		&resource.Spec.ControllerAgents.InstanceSpec,
		"/usr/bin/ytserver-controller-agent",
		"ytserver-controller-agent.yson",
		"ca",
		"controller-agents",
		func() ([]byte, error) { return cfgen.GetControllerAgentConfig(resource.Spec.ControllerAgents) },
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.ControllerAgentRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &ControllerAgent{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               master,
	}
}

func (ca *ControllerAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, ca.server)
}

func (ca *ControllerAgent) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(ca.ytsaurus.GetClusterState()) && ca.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if ca.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, ca, dry); status != nil {
			return *status, err
		}
	}

	if status, err := checkComponentDependency(ctx, ca.master); status != nil {
		return *status, err
	}

	if ServerNeedSync(ca.server, ca.ytsaurus) {
		if !dry {
			err = ca.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !ca.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}
