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
	serverComponentBase
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

	server := newServer(
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
	)

	return &httpProxy{
		serverComponentBase: serverComponentBase{
			componentBase: componentBase{
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

func (hp *httpProxy) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		hp.server,
		hp.balancingService,
	})
}

func (hp *httpProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if hp.ytsaurus.GetClusterState() == ytv1.ClusterStateRunning && hp.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if hp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating && hp.IsUpdating() {
		if hp.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			if !dry {
				err = hp.removePods(ctx)
			}
			return WaitingStatus(SyncStatusUpdating, "pods removal"), err
		}
	}

	if !(hp.master.Status(ctx).SyncStatus == SyncStatusReady) {
		return WaitingStatus(SyncStatusBlocked, hp.master.GetName()), err
	}

	if hp.server.needSync() {
		if !dry {
			err = hp.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !resources.Exists(hp.balancingService) {
		if !dry {
			s := hp.balancingService.Build()
			s.Spec.Type = hp.serviceType
			err = hp.balancingService.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, hp.balancingService.Name()), err
	}

	if !hp.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (hp *httpProxy) Status(ctx context.Context) ComponentStatus {
	status, err := hp.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (hp *httpProxy) Sync(ctx context.Context) error {
	_, err := hp.doSync(ctx, false)
	return err
}
