package components

import (
	"context"

	v1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type rpcProxy struct {
	ytsaurusServerComponent
	cfgen *ytconfig.Generator

	master Component

	serviceType      *v1.ServiceType
	balancingService *resources.RPCService
	tlsSecret        *resources.TLSSecret
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

	srv := newServer(
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

		balancingService.SetNodePort(spec.NodePort)
	}

	var tlsSecret *resources.TLSSecret
	if secret := spec.Transport.TLSSecret; secret != nil {
		tlsSecret = resources.NewTLSSecret(
			secret.Name,
			consts.RPCSecretVolumeName,
			consts.RPCSecretMountPoint)
	}

	return &rpcProxy{
		ytsaurusServerComponent: newYtsaurusServerComponent(&l, ytsaurus, srv),
		cfgen:                   cfgen,
		master:                  masterReconciler,
		serviceType:             spec.ServiceType,
		balancingService:        balancingService,
		tlsSecret:               tlsSecret,
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
	return resources.Fetch(ctx, fetchable...)
}

func (rp *rpcProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(rp.ytsaurus.GetClusterState()) && rp.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if rp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, rp.ytsaurus, rp, &rp.ytsaurusComponent, rp.server, dry); status != nil {
			return *status, err
		}
	}

	if !IsRunningStatus(rp.master.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, rp.master.GetName()), err
	}

	if rp.NeedSync() {
		if !dry {
			statefulSet := rp.server.buildStatefulSet()
			if secret := rp.tlsSecret; secret != nil {
				secret.AddVolume(&statefulSet.Spec.Template.Spec)
				secret.AddVolumeMount(&statefulSet.Spec.Template.Spec.Containers[0])
			}
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
