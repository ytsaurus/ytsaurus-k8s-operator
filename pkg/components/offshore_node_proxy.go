package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type OffshoreNodeProxy struct {
	localServerComponent
	cfgen *ytconfig.Generator
}

func NewOffshoreNodeProxy(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
) *OffshoreNodeProxy {
	l := cfgen.GetComponentLabeller(consts.OffshoreNodeProxyType, "")

	resource := ytsaurus.GetResource()
	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.OffshoreNodeProxies.InstanceSpec,
		"/usr/bin/ytserver-offshore-node-proxy",
		"ytserver-offshore-node-proxy.yson",
		func() ([]byte, error) {
			return cfgen.GetOffshoreNodeProxiesConfig(resource.Spec.OffshoreNodeProxies)
		},
		consts.OffshoreNodeProxyMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.OffshoreNodeProxyRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &OffshoreNodeProxy{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
	}
}

func (p *OffshoreNodeProxy) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, p.server)
}

func (p *OffshoreNodeProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(p.ytsaurus.GetClusterState()) && p.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if p.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, p.ytsaurus, p, &p.localComponent, p.server, dry); status != nil {
			return *status, err
		}
	}

	if p.NeedSync() {
		if !dry {
			err = p.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !p.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (p *OffshoreNodeProxy) Status(ctx context.Context) (ComponentStatus, error) {
	return p.doSync(ctx, true)
}

func (p *OffshoreNodeProxy) Sync(ctx context.Context) error {
	_, err := p.doSync(ctx, false)
	return err
}
