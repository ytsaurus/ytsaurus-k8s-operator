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
	"k8s.io/utils/strings/slices"
)

type httpProxy struct {
	ServerComponentBase
	serviceType v1.ServiceType

	master           Component
	balancingService *resources.HTTPService

	role string

	ytClient yt.Client
}

func NewHTTPProxy(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	masterReconciler Component,
	spec ytv1.HTTPProxiesSpec) Component {

	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelHTTPProxy, spec.Role),
		ComponentName:  cfgen.FormatComponentStringWithDefault("HttpProxy", spec.Role),
		MonitoringPort: consts.HTTPProxyMonitoringPort,
	}

	server := NewServer(
		&l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-http-proxy",
		"ytserver-http-proxy.yson",
		cfgen.GetHTTPProxiesStatefulSetName(spec.Role),
		cfgen.GetHTTPProxiesHeadlessServiceName(spec.Role),
		func() ([]byte, error) {
			return cfgen.GetHTTPProxyConfig(spec)
		},
		func(data []byte) (bool, error) {
			return cfgen.NeedHTTPProxyConfigReload(spec, data)
		},
	)

	return &httpProxy{
		ServerComponentBase: ServerComponentBase{
			ComponentBase: ComponentBase{
				labeller: &l,
				ytsaurus: ytsaurus,
				cfgen:    cfgen,
			},
			server: server,
		},
		master:      masterReconciler,
		serviceType: spec.ServiceType,
		role:        spec.Role,
		balancingService: resources.NewHTTPService(
			cfgen.GetHTTPProxiesServiceName(spec.Role),
			&l,
			ytsaurus.APIProxy()),
	}
}

func (r *httpProxy) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		r.server,
		r.balancingService,
	})
}

func (r *httpProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if r.ytsaurus.GetClusterState() == ytv1.ClusterStateRunning && r.server.NeedUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if r.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if r.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			updatingComponents := r.ytsaurus.GetLocalUpdatingComponents()
			if updatingComponents == nil || slices.Contains(updatingComponents, r.GetName()) {
				return WaitingStatus(SyncStatusUpdating, "pods removal"), r.removePods(ctx, dry)
			}
		}
	}

	if !(r.master.Status(ctx).SyncStatus == SyncStatusReady) {
		return WaitingStatus(SyncStatusBlocked, r.master.GetName()), err
	}

	if r.server.NeedSync() {
		if !dry {
			err = r.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !resources.Exists(r.balancingService) {
		if !dry {
			s := r.balancingService.Build()
			s.Spec.Type = r.serviceType
			err = r.balancingService.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, r.balancingService.Name()), err
	}

	if !r.server.ArePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (r *httpProxy) Status(ctx context.Context) ComponentStatus {
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
