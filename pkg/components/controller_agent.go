package components

import (
	"context"
	v1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
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

func NewControllerAgent(cfgen *ytconfig.Generator, apiProxy *apiproxy.APIProxy, master Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: consts.YTComponentLabelControllerAgent,
		ComponentName:  "ControllerAgent",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.ControllerAgents.InstanceSpec,
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
				labeller: &labeller,
				apiProxy: apiProxy,
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

func (ca *controllerAgent) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error
	if ca.apiProxy.GetClusterState() == v1.ClusterStateUpdating {
		if ca.apiProxy.GetUpdateState() == v1.UpdateStateWaitingForPodsRemoval {
			return SyncStatusUpdating, ca.removePods(ctx, dry)
		}
	}

	if ca.master.Status(ctx) != SyncStatusReady {
		return SyncStatusBlocked, err
	}

	if !ca.server.IsInSync() {
		if !dry {
			// TODO(psushin): there should be me more sophisticated logic for version updates.
			err = ca.server.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	if !ca.server.ArePodsReady(ctx) {
		return SyncStatusBlocked, err
	}

	return SyncStatusReady, err
}

func (ca *controllerAgent) Status(ctx context.Context) SyncStatus {
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
