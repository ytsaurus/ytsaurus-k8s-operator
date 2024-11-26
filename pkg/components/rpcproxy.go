package components

import (
	"context"

	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type RpcProxy struct {
	localServerComponent
	cfgen *ytconfig.Generator

	master Component

	serviceType      *corev1.ServiceType
	balancingService *resources.RPCService
	tlsSecret        *resources.TLSSecret
}

func NewRPCProxy(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	masterReconciler Component,
	spec ytv1.RPCProxiesSpec) *RpcProxy {
	resource := ytsaurus.Resource()
	l := labeller.Labeller{
		ObjectMeta:        &resource.ObjectMeta,
		ComponentType:     consts.RpcProxyType,
		ComponentNamePart: spec.Role,
	}

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.To(int32(consts.RPCProxyMonitoringPort))
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
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.RPCProxyRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
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

	return &RpcProxy{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               masterReconciler,
		serviceType:          spec.ServiceType,
		balancingService:     balancingService,
		tlsSecret:            tlsSecret,
	}
}

func (rp *RpcProxy) Fetch(ctx context.Context) error {
	fetchable := []resources.Fetchable{
		rp.server,
	}
	if rp.balancingService != nil {
		fetchable = append(fetchable, rp.balancingService)
	}
	return resources.Fetch(ctx, fetchable...)
}

func (rp *RpcProxy) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(rp.ytsaurus.GetClusterState()) && rp.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if rp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, rp, dry); status != nil {
			return *status, err
		}
	}

	if status, err := checkComponentDependency(ctx, rp.master); status != nil {
		return *status, err
	}

	if ServerNeedSync(rp.server, rp.ytsaurus) {
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
