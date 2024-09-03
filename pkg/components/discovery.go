package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type Discovery struct {
	localServerComponent
	cfgen *ytconfig.Generator
}

func NewDiscovery(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *Discovery {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:    &resource.ObjectMeta,
		APIProxy:      ytsaurus.APIProxy(),
		ComponentType: consts.DiscoveryType,
	}

	if resource.Spec.Discovery.InstanceSpec.MonitoringPort == nil {
		resource.Spec.Discovery.InstanceSpec.MonitoringPort = ptr.To(int32(consts.DiscoveryMonitoringPort))
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
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.DiscoveryRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &Discovery{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
	}
}

func (d *Discovery) IsUpdatable() bool {
	return true
}

func (d *Discovery) GetType() consts.ComponentType { return consts.DiscoveryType }

func (d *Discovery) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, d.server)
}

func (d *Discovery) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(d.ytsaurus.GetClusterState()) && d.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if d.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, d.ytsaurus, d, &d.localComponent, d.server, dry); status != nil {
			return *status, err
		}
	}

	if d.NeedSync() {
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

func (d *Discovery) Status(ctx context.Context) (ComponentStatus, error) {
	return d.doSync(ctx, true)
}

func (d *Discovery) Sync(ctx context.Context) error {
	_, err := d.doSync(ctx, false)
	return err
}
