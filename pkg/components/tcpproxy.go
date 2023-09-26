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

type tcpProxy struct {
	componentBase
	server server

	master Component

	serviceType      *v1.ServiceType
	balancingService *resources.TCPService
}

func NewTCPProxy(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	masterReconciler Component,
	spec ytv1.TCPProxiesSpec) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelTCPProxy, spec.Role),
		ComponentName:  cfgen.FormatComponentStringWithDefault("TcpProxy", spec.Role),
		MonitoringPort: consts.TCPProxyMonitoringPort,
	}

	server := newServer(
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
			&l,
			ytsaurus.APIProxy())
	}

	return &tcpProxy{
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

func (tp *tcpProxy) IsUpdatable() bool {
	return true
}

func (tp *tcpProxy) Fetch(ctx context.Context) error {
	fetchable := []resources.Fetchable{
		tp.server,
	}
	if tp.balancingService != nil {
		fetchable = append(fetchable, tp.balancingService)
	}
	return resources.Fetch(ctx, fetchable)
}

func (tp *tcpProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if tp.ytsaurus.GetClusterState() == ytv1.ClusterStateRunning && tp.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if tp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, tp.ytsaurus, tp, &tp.componentBase, tp.server, dry); status != nil {
			return *status, err
		}
	}

	if !(tp.master.Status(ctx).SyncStatus == SyncStatusReady) {
		return WaitingStatus(SyncStatusBlocked, tp.master.GetName()), err
	}

	if tp.server.needSync() {
		if !dry {
			err = tp.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if tp.balancingService != nil && !resources.Exists(tp.balancingService) {
		if !dry {
			s := tp.balancingService.Build()
			s.Spec.Type = *tp.serviceType
			err = tp.balancingService.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, tp.balancingService.Name()), err
	}

	if !tp.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (tp *tcpProxy) Status(ctx context.Context) ComponentStatus {
	status, err := tp.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (tp *tcpProxy) Sync(ctx context.Context) error {
	_, err := tp.doSync(ctx, false)
	return err
}
