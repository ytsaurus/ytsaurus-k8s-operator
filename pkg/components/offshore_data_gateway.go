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

type OffshoreDataGateway struct {
	server server
	cfgen  *ytconfig.NodeGenerator
	spec   *ytv1.OffshoreDataGatewaySpec
	baseComponent
}

func NewOffshoreDataGateways(
	cfgen *ytconfig.NodeGenerator,
	proxy apiproxy.APIProxy,
	spec ytv1.OffshoreDataGatewaySpec,
	commonSpec ytv1.CommonSpec,
) *OffshoreDataGateway {
	l := cfgen.GetComponentLabeller(consts.OffshoreDataGatewayType, "")

	srv := newServerConfigured(
		l,
		proxy,
		&commonSpec,
		&ytv1.PodSpec{},
		&spec.InstanceSpec,
		"/usr/bin/ytserver-offshore-data-gateway",
		"ytserver-offshore-data-gateway.yson",
		func() ([]byte, error) {
			return cfgen.GetOffshoreDataGatewaysConfig(spec)
		},
		consts.OffshoreDataGatewayMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.OffshoreDataGatewayRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)
	return &OffshoreDataGateway{
		baseComponent: baseComponent{labeller: l},
		server:        srv,
		cfgen:         cfgen,
		spec:          &spec,
	}
}

func (p *OffshoreDataGateway) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if p.server.needSync() || p.server.needUpdate() {
		if !dry {
			err = p.server.Sync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if !p.server.arePodsReady(ctx) {
		return ComponentStatusWaitingFor("pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (p *OffshoreDataGateway) GetType() consts.ComponentType {
	return consts.OffshoreDataGatewayType
}

func (p *OffshoreDataGateway) Sync(ctx context.Context) (ComponentStatus, error) {
	return p.doSync(ctx, false)
}

func (p *OffshoreDataGateway) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, p.server)
}
