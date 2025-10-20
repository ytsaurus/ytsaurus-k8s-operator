package components

import (
	"context"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
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
	spec ytv1.RPCProxiesSpec,
) *RpcProxy {
	l := cfgen.GetComponentLabeller(consts.RpcProxyType, spec.Role)

	var ports []corev1.ContainerPort
	if cfgen.GetClusterFeatures().RPCProxyHavePublicAddress {
		ports = []corev1.ContainerPort{
			{
				Name:          consts.YTRPCPortName,
				ContainerPort: consts.RPCProxyInternalRPCPort,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          consts.YTPublicRPCPortName,
				ContainerPort: consts.RPCProxyPublicRPCPort,
				Protocol:      corev1.ProtocolTCP,
			},
		}
	} else {
		ports = []corev1.ContainerPort{
			{
				Name:          consts.YTRPCPortName,
				ContainerPort: consts.RPCProxyPublicRPCPort,
				Protocol:      corev1.ProtocolTCP,
			},
		}
	}

	srv := newServer(
		l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-proxy",
		"ytserver-rpc-proxy.yson",
		func() ([]byte, error) {
			return cfgen.GetRPCProxyConfig(spec)
		},
		consts.RPCProxyMonitoringPort,
		WithContainerPorts(ports...),
	)

	var balancingService *resources.RPCService = nil
	if spec.ServiceType != nil {
		balancingService = resources.NewRPCService(
			cfgen.GetRPCProxiesServiceName(spec.Role),
			[]corev1.ServicePort{
				{
					Name:       consts.YTRPCPortName,
					Port:       consts.RPCProxyPublicRPCPort,
					TargetPort: intstr.FromInt(consts.RPCProxyPublicRPCPort),
					NodePort:   ptr.Deref(spec.NodePort, 0),
				},
			},
			l,
			ytsaurus.APIProxy())
	}

	var tlsSecret *resources.TLSSecret
	if secret := spec.Transport.TLSSecret; secret != nil {
		tlsSecret = resources.NewTLSSecret(
			secret.Name,
			consts.RPCProxySecretVolumeName,
			consts.RPCProxySecretMountPoint)
	}

	return &RpcProxy{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
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

// needUpdate overrides the server's needUpdate to account for TLS secret volumes
// that are added after StatefulSet build
func (rp *RpcProxy) needUpdate() bool {
	return rp.server.needUpdateWithSpecCheck(rp.needStatefulSetSpecUpdate)
}

// needStatefulSetSpecUpdate checks if the StatefulSet spec has changed
func (rp *RpcProxy) needStatefulSetSpecUpdate() bool {
	// Rebuild the StatefulSet with TLS secret included
	desiredSpec := rp.server.rebuildStatefulSet().Spec
	rp.applyTlsSecret(&desiredSpec.Template.Spec)
	return rp.server.getStatefulSet().SpecChanged(desiredSpec)
}

// applyTlsSecret applies TLS secret volume and mount to the pod spec
func (rp *RpcProxy) applyTlsSecret(podSpec *corev1.PodSpec) {
	if rp.tlsSecret != nil {
		rp.tlsSecret.AddVolume(podSpec)
		rp.tlsSecret.AddVolumeMount(&podSpec.Containers[0])
	}
}

func (rp *RpcProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(rp.ytsaurus.GetClusterState()) && rp.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if rp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, rp.ytsaurus, rp, &rp.localComponent, rp.server, dry); status != nil {
			return *status, err
		}
	}

	masterStatus, err := rp.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, rp.master.GetFullName()), err
	}

	if rp.NeedSync() {
		if !dry {
			statefulSet := rp.server.buildStatefulSet()
			rp.applyTlsSecret(&statefulSet.Spec.Template.Spec)
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

func (rp *RpcProxy) Status(ctx context.Context) (ComponentStatus, error) {
	return rp.doSync(ctx, true)
}

func (rp *RpcProxy) Sync(ctx context.Context) error {
	_, err := rp.doSync(ctx, false)
	return err
}
