package components

import (
	"context"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	v1 "k8s.io/api/core/v1"
)

type rpcProxy struct {
	ComponentBase
	server Server

	master Component

	serviceType      *v1.ServiceType
	balancingService *resources.RPCService
}

func NewRPCProxy(
	cfgen *ytconfig.Generator,
	apiProxy *apiproxy.APIProxy,
	masterReconciler Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: consts.YTComponentLabelRPCProxy,
		ComponentName:  "RpcProxy",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.RPCProxies.InstanceGroup,
		"/usr/bin/ytserver-proxy",
		"ytserver-rpc-proxy.yson",
		cfgen.GetRPCProxiesStatefulSetName(),
		cfgen.GetRPCProxiesHeadlessServiceName(),
		cfgen.GetRPCProxyConfig,
	)

	var balancingService *resources.RPCService = nil
	if ytsaurus.Spec.RPCProxies.ServiceType != nil {
		balancingService = resources.NewRPCService(
			cfgen.GetRPCProxiesServiceName(),
			&labeller,
			apiProxy)
	}

	return &rpcProxy{
		ComponentBase: ComponentBase{
			labeller: &labeller,
			apiProxy: apiProxy,
			cfgen:    cfgen,
		},
		server:           server,
		master:           masterReconciler,
		serviceType:      ytsaurus.Spec.RPCProxies.ServiceType,
		balancingService: balancingService,
	}
}

func (r *rpcProxy) Fetch(ctx context.Context) error {
	fetchable := []resources.Fetchable{
		r.server,
	}
	if r.balancingService != nil {
		fetchable = append(fetchable, r.balancingService)
	}
	return resources.Fetch(ctx, fetchable)
}

func (r *rpcProxy) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error
	if !(r.master.Status(ctx) == SyncStatusReady) {
		return SyncStatusBlocked, err
	}

	if !r.server.IsInSync() {
		if !dry {
			err = r.server.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	if r.balancingService != nil && !resources.Exists(r.balancingService) {
		if !dry {
			s := r.balancingService.Build()
			s.Spec.Type = *r.serviceType
			err = r.balancingService.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	return SyncStatusReady, err
}

func (r *rpcProxy) Status(ctx context.Context) SyncStatus {
	status, err := r.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (r *rpcProxy) Sync(ctx context.Context) error {
	_, err := r.doSync(ctx, false)
	return err
}
