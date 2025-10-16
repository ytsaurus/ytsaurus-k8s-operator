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

type CypressProxy struct {
	localServerComponent
	cfgen *ytconfig.Generator
}

func NewCypressProxy(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *CypressProxy {
	l := cfgen.GetComponentLabeller(consts.CypressProxyType, "")

	resource := ytsaurus.GetResource()
	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.CypressProxies.InstanceSpec,
		"/usr/bin/ytserver-cypress-proxy",
		"ytserver-cypress-proxy.yson",
		func() ([]byte, error) { return cfgen.GetCypressProxiesConfig(resource.Spec.CypressProxies) },
		consts.CypressProxyMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.CypressProxyRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &CypressProxy{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
	}
}

func (cyp *CypressProxy) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, cyp.server)
}

func (cyp *CypressProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(cyp.ytsaurus.GetClusterState()) && cyp.server.needUpdate() && canComponentBeUpdated(cyp.ytsaurus, cyp) {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if cyp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, cyp.ytsaurus, cyp, &cyp.localComponent, cyp.server, dry); status != nil {
			return *status, err
		}
	}

	if cyp.NeedSync() {
		if !dry {
			err = cyp.doServerSync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !cyp.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (cyp *CypressProxy) Status(ctx context.Context) (ComponentStatus, error) {
	return cyp.doSync(ctx, true)
}

func (cyp *CypressProxy) Sync(ctx context.Context) error {
	_, err := cyp.doSync(ctx, false)
	return err
}

func (cyp *CypressProxy) doServerSync(ctx context.Context) error {
	return cyp.server.Sync(ctx)
}
