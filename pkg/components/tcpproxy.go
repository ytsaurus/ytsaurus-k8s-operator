package components

import (
	"context"

	"go.ytsaurus.tech/library/go/ptr"
	v1 "k8s.io/api/core/v1"

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

	serviceType      *v1.ServiceType
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
		ComponentName:  cfgen.FormatComponentStringWithDefault("TcpProxy", spec.Role),
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

func (tp *TcpProxy) Fetch(ctx context.Context) error {
	fetchable := []resources.Fetchable{
		tp.server,
	}
	if tp.balancingService != nil {
		fetchable = append(fetchable, tp.balancingService)
	}
	return resources.Fetch(ctx, fetchable...)
}

func (tp *TcpProxy) Status(ctx context.Context) ComponentStatus {
	if ytv1.IsReadyToUpdateClusterState(tp.ytsaurus.GetClusterState()) && tp.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	if tp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, tp.ytsaurus, tp, &tp.localComponent, tp.server, true)
		if status != nil {
			if err != nil {
				panic(err)
			}
			return *status
		}
	}

	if !IsRunningStatus(tp.master.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, tp.master.GetName())
	}

	if tp.NeedSync() {
		return WaitingStatus(SyncStatusPending, "components")
	}

	if tp.balancingService != nil && !resources.Exists(tp.balancingService) {
		return WaitingStatus(SyncStatusPending, tp.balancingService.Name())
	}

	if !tp.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods")
	}

	return SimpleStatus(SyncStatusReady)
}

func (tp *TcpProxy) Sync(ctx context.Context) error {
	if ytv1.IsReadyToUpdateClusterState(tp.ytsaurus.GetClusterState()) && tp.server.needUpdate() {
		return nil
	}

	if tp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, tp.ytsaurus, tp, &tp.localComponent, tp.server, false)
		if status != nil {
			return err
		}
	}

	if !IsRunningStatus(tp.master.Status(ctx).SyncStatus) {
		return nil
	}

	if tp.NeedSync() {
		return tp.server.Sync(ctx)
	}

	if tp.balancingService != nil && !resources.Exists(tp.balancingService) {
		tp.balancingService.Build()
		return tp.balancingService.Sync(ctx)
	}
	return nil
}
