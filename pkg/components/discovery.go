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

type discovery struct {
	ServerComponentBase
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

	server := NewServer(
		&l,
		ytsaurus,
		&resource.Spec.Discovery.InstanceSpec,
		"/usr/bin/ytserver-discovery",
		"ytserver-discovery.yson",
		cfgen.GetDiscoveryStatefulSetName(),
		cfgen.GetDiscoveryServiceName(),
		cfgen.GetDiscoveryConfig,
		cfgen.NeedDiscoveryConfigReload,
	)

	return &discovery{
		ServerComponentBase: ServerComponentBase{
			ComponentBase: ComponentBase{
				labeller: &l,
				ytsaurus: ytsaurus,
				cfgen:    cfgen,
			},
			server: server,
		},
	}
}

func (d *discovery) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		d.server,
	})
}

func (d *discovery) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error

	if d.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if d.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			updatingComponents := d.ytsaurus.GetLocalUpdatingComponents()
			if updatingComponents == nil || slices.Contains(updatingComponents, d.GetName()) {
				return SyncStatusUpdating, d.removePods(ctx, dry)
			}
		}
	}

	if d.server.NeedSync() {
		if !dry {
			err = d.server.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	if !d.server.ArePodsReady(ctx) {
		return SyncStatusBlocked, err
	}

	return SyncStatusReady, err
}

func (d *discovery) Status(ctx context.Context) SyncStatus {
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
