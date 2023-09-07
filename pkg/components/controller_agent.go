package components

import (
	"context"
	"k8s.io/utils/strings/slices"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type controllerAgent struct {
	ServerComponentBase
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

	server := NewServer(
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
		ServerComponentBase: ServerComponentBase{
			server: server,
			ComponentBase: ComponentBase{
				labeller: &l,
				ytsaurus: ytsaurus,
				cfgen:    cfgen,
			},
		},
		master: master,
	}
}

func (ca *controllerAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		ca.server,
	})
}

func (ca *controllerAgent) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ca.ytsaurus.GetClusterState() == ytv1.ClusterStateRunning && ca.server.NeedUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if ca.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if ca.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			updatingComponents := ca.ytsaurus.GetLocalUpdatingComponents()
			if updatingComponents == nil || slices.Contains(updatingComponents, ca.GetName()) {
				return WaitingStatus(SyncStatusUpdating, "pods removal"), ca.removePods(ctx, dry)
			}
		}
	}

	if ca.master.Status(ctx).SyncStatus != SyncStatusReady {
		return WaitingStatus(SyncStatusBlocked, ca.master.GetName()), err
	}

	if ca.server.NeedSync() {
		if !dry {
			// TODO(psushin): there should be me more sophisticated logic for version updates.
			err = ca.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !ca.server.ArePodsReady(ctx) {
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
