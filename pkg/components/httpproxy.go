package components

import (
	"context"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/yt"
	v1 "k8s.io/api/core/v1"
)

type httpProxy struct {
	ServerComponentBase
	serviceType v1.ServiceType

	master           Component
	balancingService *resources.HTTPService

	ytClient yt.Client
}

func NewHTTPProxy(
	cfgen *ytconfig.Generator,
	apiProxy *apiproxy.APIProxy,
	masterReconciler Component) Component {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: consts.YTComponentLabelHTTPProxy,
		ComponentName:  "HttpProxy",
	}

	server := NewServer(
		&labeller,
		apiProxy,
		&ytsaurus.Spec.HTTPProxies[0].InstanceSpec,
		"/usr/bin/ytserver-http-proxy",
		"ytserver-http-proxy.yson",
		cfgen.GetHTTPProxiesStatefulSetName(),
		cfgen.GetHTTPProxiesHeadlessServiceName(),
		cfgen.GetHTTPProxyConfig,
	)

	return &httpProxy{
		ServerComponentBase: ServerComponentBase{
			ComponentBase: ComponentBase{
				labeller: &labeller,
				apiProxy: apiProxy,
				cfgen:    cfgen,
			},
			server: server,
		},
		master:      masterReconciler,
		serviceType: ytsaurus.Spec.HTTPProxies[0].ServiceType,
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

	if r.apiProxy.GetClusterState() == ytv1.ClusterStateUpdating {
		if r.apiProxy.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
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
