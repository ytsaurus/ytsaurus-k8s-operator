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

type discovery struct {
	componentBase
	server server
}

func NewDiscovery(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelDiscovery,
		ComponentName:  "Discovery",
		MonitoringPort: consts.DiscoveryMonitoringPort,
	}

	server := newServer(
		&l,
		ytsaurus,
		&resource.Spec.Discovery.InstanceSpec,
		"/usr/bin/ytserver-discovery",
		"ytserver-discovery.yson",
		cfgen.GetDiscoveryStatefulSetName(),
		cfgen.GetDiscoveryServiceName(),
		cfgen.GetDiscoveryConfig,
	)

	return &discovery{
		componentBase: componentBase{
			labeller: &l,
			ytsaurus: ytsaurus,
			cfgen:    cfgen,
		},
		server: server,
	}
}

func (d *discovery) IsUpdatable() bool {
	return true
}

func (d *discovery) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		d.server,
	})
}

func (d *discovery) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if d.ytsaurus.GetClusterState() == ytv1.ClusterStateRunning && d.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if d.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating && IsUpdatingComponent(d.ytsaurus, d) {
		if d.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			if !dry {
				err = removePods(ctx, d.server, &d.componentBase)
			}
			return WaitingStatus(SyncStatusUpdating, "pods removal"), err
		}
	}

	if d.server.needSync() {
		if !dry {
			err = d.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !d.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (d *discovery) Status(ctx context.Context) ComponentStatus {
	status, err := d.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (d *discovery) Sync(ctx context.Context) error {
	_, err := d.doSync(ctx, false)
	return err
}
