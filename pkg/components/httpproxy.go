package components

import (
	"context"

	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/YTsaurus/yt-k8s-operator/pkg/resources"
	"github.com/YTsaurus/yt-k8s-operator/pkg/ytconfig"
	v1 "k8s.io/api/core/v1"
)

type httpProxy struct {
	labeller    *labeller.Labeller
	server      *Server
	serviceType v1.ServiceType

	master           Component
	balancingService *resources.HTTPService
}

func NewHTTPProxy(
	cfgen *ytconfig.Generator,
	apiProxy *apiproxy.APIProxy,
	masterReconciler Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: "yt-http-proxy",
		ComponentName:  "HttpProxy",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.HTTPProxies.InstanceGroup,
		"/usr/bin/ytserver-http-proxy",
		"ytserver-http-proxy.yson",
		cfgen.GetHTTPProxiesStatefulSetName(),
		cfgen.GetHTTPProxiesHeadlessServiceName(),
		false,
		cfgen.GetHTTPProxyConfig,
	)

	return &httpProxy{
		server:      server,
		labeller:    &labeller,
		master:      masterReconciler,
		serviceType: ytsaurus.Spec.HTTPProxies.ServiceType,
		balancingService: resources.NewHTTPService(
			cfgen.GetHTTPProxiesServiceName(),
			&labeller,
			apiProxy),
	}
}

func (r *httpProxy) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		r.server,
		r.balancingService,
	})
}

func (r *httpProxy) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
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

	if !resources.Exists(r.balancingService) {
		if !dry {
			s := r.balancingService.Build()
			s.Spec.Type = r.serviceType
			err = r.balancingService.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	return SyncStatusReady, err
}

func (r *httpProxy) Status(ctx context.Context) SyncStatus {
	status, err := r.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (r *httpProxy) Sync(ctx context.Context) error {
	_, err := r.doSync(ctx, false)
	return err
}
