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

type RemoteOffshoreNodeProxy struct {
	server server
	cfgen  *ytconfig.NodeGenerator
	spec   *ytv1.OffshoreNodeProxiesSpec
	baseComponent
}

func NewRemoteOffshoreNodeProxies(
	cfgen *ytconfig.NodeGenerator,
	nodes *ytv1.RemoteOffshoreNodeProxies,
	proxy apiproxy.APIProxy,
	spec ytv1.OffshoreNodeProxiesSpec,
	commonSpec ytv1.CommonSpec,
) *RemoteOffshoreNodeProxy {
	l := cfgen.GetComponentLabeller(consts.RemoteOffshoreNodeProxyType, "")

	srv := newServerConfigured(
		l,
		proxy,
		commonSpec,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-offshore-node-proxy",
		"ytserver-offshore-node-proxy.yson",
		func() ([]byte, error) {
			return cfgen.GetOffshoreNodeProxiesConfig(spec)
		},
		consts.OffshoreNodeProxyMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.OffshoreNodeProxyRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)
	return &RemoteOffshoreNodeProxy{
		baseComponent: baseComponent{labeller: l},
		server:        srv,
		cfgen:         cfgen,
		spec:          &spec,
	}
}

func (p *RemoteOffshoreNodeProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if p.server.needSync() || p.server.needUpdate() {
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

func (p *RemoteOffshoreNodeProxy) GetType() consts.ComponentType {
	return consts.RemoteOffshoreNodeProxyType
}

func (p *RemoteOffshoreNodeProxy) Sync(ctx context.Context) (ComponentStatus, error) {
	return p.doSync(ctx, false)
}

func (p *RemoteOffshoreNodeProxy) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, p.server)
}
