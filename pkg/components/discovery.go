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

type Discovery struct {
	localServerComponent
	cfgen *ytconfig.Generator
}

func NewDiscovery(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *Discovery {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelDiscovery,
		ComponentName:  "Discovery",
	}

	if resource.Spec.Discovery.InstanceSpec.MonitoringPort == nil {
		resource.Spec.Discovery.InstanceSpec.MonitoringPort = ptr.Int32(consts.DiscoveryMonitoringPort)
	}

	srv := newServer(
		&l,
		ytsaurus,
		&resource.Spec.Discovery.InstanceSpec,
		"/usr/bin/ytserver-discovery",
		"ytserver-discovery.yson",
		cfgen.GetDiscoveryStatefulSetName(),
		cfgen.GetDiscoveryServiceName(),
		func() ([]byte, error) {
			return cfgen.GetDiscoveryConfig(&resource.Spec.Discovery)
		},
	)

	return &Discovery{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
	}
}

func (d *Discovery) IsUpdatable() bool {
	return true
}

func (d *Discovery) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, d.server)
}

func (d *Discovery) Status(ctx context.Context) ComponentStatus {
	if ytv1.IsReadyToUpdateClusterState(d.ytsaurus.GetClusterState()) && d.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	if d.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, d.ytsaurus, d, &d.localComponent, d.server, true)
		if status != nil {
			if err != nil {
				panic(err)
			}
			return *status
		}
	}

	if d.NeedSync() {
		return WaitingStatus(SyncStatusPending, "components")
	}

	if !d.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods")
	}

	return SimpleStatus(SyncStatusReady)
}

func (d *Discovery) Sync(ctx context.Context) error {
	var err error

	if ytv1.IsReadyToUpdateClusterState(d.ytsaurus.GetClusterState()) && d.server.needUpdate() {
		return nil
	}

	if d.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, d.ytsaurus, d, &d.localComponent, d.server, false)
		if status != nil {
			return err
		}
	}

	if d.NeedSync() {
		err = d.server.Sync(ctx)
		return err
	}

	if !d.server.arePodsReady(ctx) {
		return nil
	}

	return nil
}
