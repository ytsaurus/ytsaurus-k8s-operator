package components

import (
	"context"
	"fmt"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type node struct {
	ServerComponentBase
	master Component
}

func NewDataNode(
	cfgen *ytconfig.Generator,
	apiProxy *apiproxy.APIProxy,
	master Component,
	spec ytv1.DataNodesSpec,
) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: fmt.Sprintf("%s-%s", consts.YTComponentLabelDataNode, spec.Name),
		ComponentName:  fmt.Sprintf("DataNode-%s", spec.Name),
		MonitoringPort: consts.NodeMonitoringPort,
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-data-node.yson",
		cfgen.GetDataNodesStatefulSetName(spec.Name),
		cfgen.GetDataNodesServiceName(spec.Name),
		func() ([]byte, error) {
			return cfgen.GetDataNodeConfig(spec)
		},
	)

	return &node{
		ServerComponentBase: ServerComponentBase{
			ComponentBase: ComponentBase{
				labeller: &labeller,
				apiProxy: apiProxy,
				cfgen:    cfgen,
			},
			server: server,
		},
		master: master,
	}
}

func NewExecNode(
	cfgen *ytconfig.Generator,
	apiProxy *apiproxy.APIProxy,
	master Component,
	spec ytv1.ExecNodesSpec,
) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: fmt.Sprintf("%s-%s", consts.YTComponentLabelExecNode, spec.Name),
		ComponentName:  fmt.Sprintf("ExecNode-%v", spec.Name),
		MonitoringPort: consts.NodeMonitoringPort,
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-exec-node.yson",
		cfgen.GetExecNodesStatefulSetName(spec.Name),
		cfgen.GetExecNodesServiceName(spec.Name),
		func() ([]byte, error) {
			return cfgen.GetExecNodeConfig(spec)
		},
	)

	return &node{
		ServerComponentBase: ServerComponentBase{
			ComponentBase: ComponentBase{
				labeller: &labeller,
				apiProxy: apiProxy,
				cfgen:    cfgen,
			},
			server: server,
		},
		master: master,
	}
}

func (n *node) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		n.server,
	})
}

func (n *node) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error

	if n.apiProxy.GetClusterState() == ytv1.ClusterStateUpdating {
		if n.apiProxy.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			return SyncStatusUpdating, n.removePods(ctx, dry)
		}
	}

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
