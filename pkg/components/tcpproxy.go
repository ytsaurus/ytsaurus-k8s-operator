package components

import (
	"context"

	"go.ytsaurus.tech/library/go/ptr"
	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type TcpProxy struct {
	localServerComponent
	cfgen *ytconfig.Generator

	master Component

	serviceType      *corev1.ServiceType
	balancingService *resources.TCPService
}

func NewTCPProxy(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	masterReconciler Component,
	spec ytv1.TCPProxiesSpec) *TcpProxy {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelTCPProxy, spec.Role),
		ComponentName:  cfgen.FormatComponentStringWithDefault(string(consts.TcpProxyType), spec.Role),
	}

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.Int32(consts.TCPProxyMonitoringPort)
	}

	srv := newServer(
		&l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-tcp-proxy",
		"ytserver-tcp-proxy.yson",
		cfgen.GetTCPProxiesStatefulSetName(spec.Role),
		cfgen.GetTCPProxiesHeadlessServiceName(spec.Role),
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
			&l,
			ytsaurus.APIProxy())
	}

	return &TcpProxy{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               masterReconciler,
		serviceType:          spec.ServiceType,
		balancingService:     balancingService,
	}
}

func (tp *TcpProxy) IsUpdatable() bool {
	return true
}

func (tp *TcpProxy) GetType() consts.ComponentType { return consts.TcpProxyType }

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

func (tp *TcpProxy) doServerSync(ctx context.Context) error {
	err := tp.server.Sync(ctx)
	if err != nil {
		return err
	}
	tp.balancingService.Build()
	return tp.balancingService.Sync(ctx)
}

func (tp *TcpProxy) serverInSync(ctx context.Context) (bool, error) {
	srvInSync, err := tp.server.inSync(ctx)
	if err != nil {
		return false, err
	}
	balancerExists := resources.Exists(tp.balancingService)
	return srvInSync && balancerExists, nil
}

func (tp *TcpProxy) Status(ctx context.Context) (ComponentStatus, error) {
	return flowToStatus(ctx, tp, tp.getFlow(), tp.condManager)
}

func (tp *TcpProxy) Sync(ctx context.Context) error {
	return flowToSync(ctx, tp.getFlow(), tp.condManager)
}
