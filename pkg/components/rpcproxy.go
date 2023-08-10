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
	ServerComponentBase

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

	server := NewServer(
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
		ServerComponentBase: ServerComponentBase{
			ComponentBase: ComponentBase{
				labeller: &l,
				ytsaurus: ytsaurus,
				cfgen:    cfgen,
			},
			server: server,
		},
		master:           masterReconciler,
		serviceType:      spec.ServiceType,
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

	if r.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if r.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			return SyncStatusUpdating, r.removePods(ctx, dry)
		}
	}

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
