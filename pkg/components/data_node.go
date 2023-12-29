package components

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type dataNode struct {
	componentBase
	cfgen  *ytconfig.NodeGenerator
	server server
	master masterComponent
}

type masterComponent interface {
	GetName() string
	Status(ctx context.Context) ComponentStatus
}

func NewDataNode(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus *apiproxy.Ytsaurus,
	master masterComponent,
	spec ytv1.DataNodesSpec,
) Component {
	return NewDataNodeConfigured(
		cfgen,
		ytsaurus,
		&ytsaurus.GetResource().ObjectMeta,
		ytsaurus.APIProxy(),
		&ytsaurus.GetResource().Spec.ConfigurationSpec,
		master,
		spec,
	)
}

func NewDataNodeConfigured(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus ytsaurusResourceStateManager,
	objectMeta *metav1.ObjectMeta,
	proxy apiproxy.APIProxy,
	configSpec *ytv1.ConfigurationSpec,
	master masterComponent,
	spec ytv1.DataNodesSpec,
) Component {
	l := labeller.Labeller{
		ObjectMeta:     objectMeta,
		APIProxy:       proxy,
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelDataNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault("DataNode", spec.Name),
		MonitoringPort: consts.DataNodeMonitoringPort,
	}

	srv := newServerConfigured(
		&l,
		configSpec,
		proxy,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-data-node.yson",
		cfgen.GetDataNodesStatefulSetName(spec.Name),
		cfgen.GetDataNodesServiceName(spec.Name),
		func() ([]byte, error) {
			return cfgen.GetDataNodeConfig(spec)
		},
	)

	return &dataNode{
		componentBase: componentBase{
			labeller:             &l,
			ytsaurusStateManager: ytsaurus,
		},
		cfgen:  cfgen,
		server: srv,
		master: master,
	}
}

func (n *dataNode) IsUpdatable() bool {
	return true
}

func (n *dataNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}

func (n *dataNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(n.ytsaurusStateManager.GetClusterState()) && n.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate), err
	}

	if n.ytsaurusStateManager.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, n.ytsaurusStateManager, n, &n.componentBase, n.server, dry); status != nil {
			return *status, err
		}
	}

	if !IsRunningStatus(n.master.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, n.master.GetName()), err
	}

	if n.server.needSync() {
		if !dry {
			err = n.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !n.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (n *dataNode) Status(ctx context.Context) ComponentStatus {
	status, err := n.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (n *dataNode) Sync(ctx context.Context) error {
	_, err := n.doSync(ctx, false)
	return err
}
