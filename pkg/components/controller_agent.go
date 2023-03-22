package components

import (
	"context"

	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/YTsaurus/yt-k8s-operator/pkg/resources"
	"github.com/YTsaurus/yt-k8s-operator/pkg/ytconfig"
)

type controllerAgent struct {
	server   *Server
	master   Component
	labeller *labeller.Labeller
}

func NewControllerAgent(cfgen *ytconfig.Generator, apiProxy *apiproxy.APIProxy, master Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: "yt-controller-agent",
		ComponentName:  "ControllerAgent",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.ControllerAgents.InstanceGroup,
		"/usr/bin/ytserver-controller-agent",
		"ytserver-controller-agent.yson",
		"ca",
		"controller-agents",
		false,
		cfgen.GetControllerAgentConfig,
	)

	return &controllerAgent{
		server:   server,
		master:   master,
		labeller: &labeller,
	}
}

func (ca *controllerAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		ca.server,
	})
}

func (ca *controllerAgent) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error
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
