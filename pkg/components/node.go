package components

import (
	"context"

	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/YTsaurus/yt-k8s-operator/pkg/resources"
	"github.com/YTsaurus/yt-k8s-operator/pkg/ytconfig"
)

type node struct {
	server   *Server
	master   Component
	labeller *labeller.Labeller
}

func NewDataNode(cfgen *ytconfig.Generator, apiProxy *apiproxy.APIProxy, master Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: "yt-data-node",
		ComponentName:  "DataNode",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.DataNodes.InstanceGroup,
		"/usr/bin/ytserver-node",
		"ytserver-data-node.yson",
		"dnd",
		"data-nodes",
		false,
		cfgen.GetDataNodeConfig,
	)

	return &node{
		server:   server,
		master:   master,
		labeller: &labeller,
	}
}

func NewExecNode(cfgen *ytconfig.Generator, apiProxy *apiproxy.APIProxy, master Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: "yt-exec-node",
		ComponentName:  "DataNode",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.ExecNodes.InstanceGroup,
		"/usr/bin/ytserver-node",
		"ytserver-exec-node.yson",
		"end",
		"exec-nodes",
		false,
		cfgen.GetExecNodeConfig,
	)

	return &node{
		server:   server,
		master:   master,
		labeller: &labeller,
	}
}

func (n *node) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		n.server,
	})
}

func (n *node) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error
	if !(n.master.Status(ctx) == SyncStatusReady) {
		return SyncStatusBlocked, err
	}

	if !n.server.IsInSync() {
		if !dry {
			err = n.server.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	if !n.server.ArePodsReady(ctx) {
		return SyncStatusBlocked, err
	}

	return SyncStatusReady, err
}

func (n *node) Status(ctx context.Context) SyncStatus {
	status, err := n.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (n *node) Sync(ctx context.Context) error {
	_, err := n.doSync(ctx, false)
	return err
}
