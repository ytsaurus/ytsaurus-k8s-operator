package components

import (
	"context"

	"go.ytsaurus.tech/library/go/ptr"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type ControllerAgent struct {
	localServerComponent
	cfgen  *ytconfig.Generator
	master Component
}

func NewControllerAgent(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, master Component) *ControllerAgent {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:                 &resource.ObjectMeta,
		APIProxy:                   ytsaurus.APIProxy(),
		ComponentObjectsNamePrefix: consts.YTComponentLabelControllerAgent,
		ComponentFullName:          string(consts.ControllerAgentType),
	}

	if resource.Spec.ControllerAgents.InstanceSpec.MonitoringPort == nil {
		resource.Spec.ControllerAgents.InstanceSpec.MonitoringPort = ptr.Int32(consts.ControllerAgentMonitoringPort)
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
	)

	return &ControllerAgent{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               master,
	}
}

func (ca *ControllerAgent) IsUpdatable() bool {
	return true
}

func (ca *ControllerAgent) GetType() consts.ComponentType { return consts.ControllerAgentType }

func (ca *ControllerAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, ca.server)
}

func (ca *ControllerAgent) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(ca.ytsaurus.GetClusterState()) && ca.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if ca.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, ca.ytsaurus, ca, &ca.localComponent, ca.server, dry); status != nil {
			return *status, err
		}
	}

	masterStatus, err := ca.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, ca.master.GetName()), err
	}

	if ca.NeedSync() {
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

func (ca *ControllerAgent) Status(ctx context.Context) (ComponentStatus, error) {
	return ca.doSync(ctx, true)
}

func (ca *ControllerAgent) Sync(ctx context.Context) error {
	_, err := ca.doSync(ctx, false)
	return err
}
