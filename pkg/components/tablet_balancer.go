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

type TabletBalancer struct {
	serverComponent

	cfgen *ytconfig.Generator
}

func NewTabletBalancer(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
) *TabletBalancer {
	l := cfgen.GetComponentLabeller(consts.TabletBalancerType, "")

	resource := ytsaurus.GetResource()
	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.TabletBalancer.InstanceSpec,
		"/usr/bin/ytserver-tablet-balancer",
		"ytserver-tablet-balancer.yson",
		func() ([]byte, error) { return cfgen.GetTabletBalancerConfig(resource.Spec.TabletBalancer) },
		consts.TabletBalancerMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.TabletBalancerRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &TabletBalancer{
		serverComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:           cfgen,
	}
}

func (tb *TabletBalancer) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, tb.server)
}

func (tb *TabletBalancer) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if tb.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, tb.ytsaurus, tb, &tb.component, tb.server, dry); status != nil {
			return *status, err
		}
	}

	if tb.NeedSync() {
		if !dry {
			err = tb.doServerSync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if !tb.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (tb *TabletBalancer) Status(ctx context.Context) (ComponentStatus, error) {
	return tb.doSync(ctx, true)
}

func (tb *TabletBalancer) Sync(ctx context.Context) error {
	_, err := tb.doSync(ctx, false)
	return err
}

func (tb *TabletBalancer) doServerSync(ctx context.Context) error {
	return tb.server.Sync(ctx)
}
