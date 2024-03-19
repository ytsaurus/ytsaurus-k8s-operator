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
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelControllerAgent,
		ComponentName:  "ControllerAgent",
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

func (ca *ControllerAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, ca.server)
}

func (ca *ControllerAgent) Status(ctx context.Context) ComponentStatus {
	if ytv1.IsReadyToUpdateClusterState(ca.ytsaurus.GetClusterState()) && ca.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	if ca.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, ca.ytsaurus, ca, &ca.localComponent, ca.server, true)
		if status != nil {
			if err != nil {
				panic(err)
			}
			return *status
		}
	}

	if !IsRunningStatus(ca.master.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, ca.master.GetName())
	}

	if ca.NeedSync() {
		return WaitingStatus(SyncStatusPending, "components")
	}

	if !ca.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods")
	}

	return SimpleStatus(SyncStatusReady)
}

func (ca *ControllerAgent) Sync(ctx context.Context) error {
	var err error

	if ytv1.IsReadyToUpdateClusterState(ca.ytsaurus.GetClusterState()) && ca.server.needUpdate() {
		return nil
	}

	if ca.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, ca.ytsaurus, ca, &ca.localComponent, ca.server, false)
		if status != nil {
			return err
		}
	}

	if !IsRunningStatus(ca.master.Status(ctx).SyncStatus) {
		return nil
	}

	if ca.NeedSync() {
		err = ca.server.Sync(ctx)
		return err
	}

	if !ca.server.arePodsReady(ctx) {
		return nil
	}

	return nil
}
