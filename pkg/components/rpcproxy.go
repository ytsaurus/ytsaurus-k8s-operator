package components

import (
	"context"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	v1 "k8s.io/api/core/v1"
)

type rpcProxy struct {
	componentBase
	server server

	master Component

	serviceType      *v1.ServiceType
	balancingService *resources.RPCService
}

func NewRPCProxy(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	masterReconciler Component,
	spec ytv1.RPCProxiesSpec) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelRPCProxy, spec.Role),
		ComponentName:  cfgen.FormatComponentStringWithDefault("RpcProxy", spec.Role),
		MonitoringPort: consts.RPCProxyMonitoringPort,
	}

	server := newServer(
		&l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-proxy",
		"ytserver-rpc-proxy.yson",
		cfgen.GetRPCProxiesStatefulSetName(spec.Role),
		cfgen.GetRPCProxiesHeadlessServiceName(spec.Role),
		func() ([]byte, error) {
			return cfgen.GetRPCProxyConfig(spec)
		},
	)

	var balancingService *resources.RPCService = nil
	if spec.ServiceType != nil {
		balancingService = resources.NewRPCService(
			cfgen.GetRPCProxiesServiceName(spec.Role),
			&l,
			ytsaurus.APIProxy())
	}

	return &rpcProxy{
		componentBase: componentBase{
			labeller: &l,
			ytsaurus: ytsaurus,
			cfgen:    cfgen,
		},
		server:           server,
		master:           masterReconciler,
		serviceType:      spec.ServiceType,
		balancingService: balancingService,
	}
}

func (rp *rpcProxy) IsUpdatable() bool {
	return true
}

func (rp *rpcProxy) Fetch(ctx context.Context) error {
	fetchable := []resources.Fetchable{
		rp.server,
	}
	if rp.balancingService != nil {
		fetchable = append(fetchable, rp.balancingService)
	}
	return resources.Fetch(ctx, fetchable)
}

func (rp *rpcProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if rp.ytsaurus.GetClusterState() == ytv1.ClusterStateRunning && rp.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if rp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, rp.ytsaurus, rp, &rp.componentBase, rp.server, dry); status != nil {
			return *status, err
		}
	}

	if !(rp.master.Status(ctx).SyncStatus == SyncStatusReady) {
		return WaitingStatus(SyncStatusBlocked, rp.master.GetName()), err
	}

	if rp.server.needSync() {
		if !dry {
			err = rp.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if rp.balancingService != nil && !resources.Exists(rp.balancingService) {
		if !dry {
			s := rp.balancingService.Build()
			s.Spec.Type = *rp.serviceType
			err = rp.balancingService.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, rp.balancingService.Name()), err
	}

	if !rp.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (rp *rpcProxy) Status(ctx context.Context) ComponentStatus {
	status, err := rp.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (rp *rpcProxy) Sync(ctx context.Context) error {
	_, err := rp.doSync(ctx, false)
	return err
}
