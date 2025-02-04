package components

import (
	"context"

	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type TcpProxy struct {
	localServerComponent
	cfgen *ytconfig.Generator

	master Component

	serviceType      *corev1.ServiceType
	balancingService *resources.TCPService
}

func NewTCPProxy(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, masterReconciler Component, spec ytv1.TCPProxiesSpec) *TcpProxy {
	l := cfgen.GetComponentLabeller(consts.TcpProxyType, spec.Role)

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.To(int32(consts.TCPProxyMonitoringPort))
	}

	srv := newServer(
		l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-tcp-proxy",
		"ytserver-tcp-proxy.yson",
		func() ([]byte, error) {
			return cfgen.GetTCPProxyConfig(spec)
		},
	)

	var balancingService *resources.TCPService = nil
	if spec.ServiceType != nil {
		balancingService = resources.NewTCPService(
			cfgen.GetTCPProxiesServiceName(spec.Role),
			*spec.ServiceType,
			spec.PortCount,
			spec.MinPort,
			l,
			ytsaurus.APIProxy())
	}

	return &TcpProxy{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               masterReconciler,
		serviceType:          spec.ServiceType,
		balancingService:     balancingService,
	}
}

func (tp *TcpProxy) Fetch(ctx context.Context) error {
	fetchable := []resources.Fetchable{
		tp.server,
	}
	if tp.balancingService != nil {
		fetchable = append(fetchable, tp.balancingService)
	}
	return resources.Fetch(ctx, fetchable...)
}

func (tp *TcpProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(tp.ytsaurus.GetClusterState()) && tp.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if tp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, tp.ytsaurus, tp, &tp.localComponent, tp.server, dry); status != nil {
			return *status, err
		}
	}

	tpStatus, err := tp.master.Status(ctx)
	if err != nil {
		return tpStatus, err
	}
	if !IsRunningStatus(tpStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, tp.master.GetName()), err
	}

	if tp.NeedSync() {
		if !dry {
			err = tp.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if tp.balancingService != nil && !resources.Exists(tp.balancingService) {
		if !dry {
			tp.balancingService.Build()
			err = tp.balancingService.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, tp.balancingService.Name()), err
	}

	if !tp.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (tp *TcpProxy) Status(ctx context.Context) (ComponentStatus, error) {
	return tp.doSync(ctx, true)
}

func (tp *TcpProxy) Sync(ctx context.Context) error {
	_, err := tp.doSync(ctx, false)
	return err
}
