package components

import (
	"context"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type controllerAgent struct {
	ytsaurusServerComponent
	cfgen  *ytconfig.Generator
	master Component
}

func NewControllerAgent(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, master Component) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelControllerAgent,
		ComponentName:  "ControllerAgent",
		MonitoringPort: consts.ControllerAgentMonitoringPort,
	}

	srv := newServer(
		&l,
		ytsaurus,
		&resource.Spec.ControllerAgents.InstanceSpec,
		"/usr/bin/ytserver-controller-agent",
		"ytserver-controller-agent.yson",
		"ca",
		"controller-agents",
		cfgen.GetControllerAgentConfig,
	)

	return &controllerAgent{
		ytsaurusServerComponent: newYtsaurusServerComponent(&l, ytsaurus, srv),
		cfgen:                   cfgen,
		master:                  master,
	}
}

func (ca *controllerAgent) IsUpdatable() bool {
	return true
}

func (ca *controllerAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, ca.server)
}

func (ca *controllerAgent) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(ca.ytsaurus.GetClusterState()) && ca.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if ca.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, ca.ytsaurus, ca, &ca.ytsaurusComponent, ca.server, dry); status != nil {
			return *status, err
		}
	}

	if !IsRunningStatus(ca.master.Status(ctx).SyncStatus) {
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

func (ca *controllerAgent) Status(ctx context.Context) ComponentStatus {
	status, err := ca.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (ca *controllerAgent) Sync(ctx context.Context) error {
	_, err := ca.doSync(ctx, false)
	return err
}
