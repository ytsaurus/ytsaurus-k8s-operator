package components

import (
	"context"

	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/YTsaurus/yt-k8s-operator/pkg/resources"
	"github.com/YTsaurus/yt-k8s-operator/pkg/ytconfig"
)

type rpcProxy struct {
	labeller *labeller.Labeller
	server   *Server
	master   Component
}

func NewRPCProxy(
	cfgen *ytconfig.Generator,
	apiProxy *apiproxy.APIProxy,
	masterReconciler Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: "yt-rpc-proxy",
		ComponentName:  "RpcProxy",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.HTTPProxies.InstanceGroup,
		"/usr/bin/ytserver-proxy",
		"ytserver-rpc-proxy.yson",
		cfgen.GetRPCProxiesStatefulSetName(),
		cfgen.GetRPCProxiesServiceName(),
		false,
		cfgen.GetRPCProxyConfig,
	)

	return &rpcProxy{
		server:   server,
		labeller: &labeller,
		master:   masterReconciler,
	}
}

func (r *rpcProxy) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		r.server,
	})
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
