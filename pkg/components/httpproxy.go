package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type HttpProxy struct {
	localServerComponent
	cfgen *ytconfig.Generator

	serviceType      corev1.ServiceType
	master           Component
	balancingService *resources.HTTPService

	role        string
	httpsSecret *resources.TLSSecret
}

func NewHTTPProxy(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	masterReconciler Component,
	spec ytv1.HTTPProxiesSpec,
) *HttpProxy {
	l := cfgen.GetComponentLabeller(consts.HttpProxyType, spec.Role)
	containerPorts := []corev1.ContainerPort{
		{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.HTTPProxyRPCPort,
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          consts.HTTPPortName,
			ContainerPort: ptr.Deref(spec.HttpPort, consts.HTTPProxyHTTPPort),
			Protocol:      corev1.ProtocolTCP,
		},
		{
			Name:          consts.HTTPSPortName,
			ContainerPort: ptr.Deref(spec.HttpsPort, consts.HTTPProxyHTTPSPort),
			Protocol:      corev1.ProtocolTCP,
		},
	}
	var chytProxy *ytv1.CHYTProxySpec
	if ytsaurus.GetClusterFeatures().HTTPProxyHaveChytAddress {
		chytProxy = ptr.To(ptr.Deref(spec.ChytProxy, ytv1.CHYTProxySpec{}))
	}

	if chytProxy != nil {
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          consts.CHYTHttpProxyName,
			ContainerPort: ptr.Deref(chytProxy.HttpPort, int32(consts.HTTPProxyChytHttpPort)),
			Protocol:      corev1.ProtocolTCP,
		})
		containerPorts = append(containerPorts, corev1.ContainerPort{
			Name:          consts.CHYTHttpsProxyName,
			ContainerPort: ptr.Deref(chytProxy.HttpsPort, int32(consts.HTTPProxyChytHttpsPort)),
			Protocol:      corev1.ProtocolTCP,
		})
	}

	srv := newServer(
		l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-http-proxy",
		"ytserver-http-proxy.yson",
		func() ([]byte, error) {
			return cfgen.GetHTTPProxyConfig(spec)
		},
		consts.HTTPProxyMonitoringPort,
		WithContainerPorts(containerPorts...),
		WithCustomReadinessProbeEndpointPort(ptr.Deref(spec.HttpPort, consts.HTTPProxyHTTPPort)),
		WithCustomReadinessProbeEndpointPath("/ping"),
	)

	var httpsSecret *resources.TLSSecret
	if spec.Transport.HTTPSSecret != nil {
		httpsSecret = resources.NewTLSSecret(
			spec.Transport.HTTPSSecret.Name,
			consts.HTTPSSecretVolumeName,
			consts.HTTPSSecretMountPoint)
	}

	balancingService := resources.NewHTTPService(
		cfgen.GetHTTPProxiesServiceName(spec.Role),
		&spec.Transport,
		l,
		ytsaurus.APIProxy())

	balancingService.SetHttpPort(spec.HttpPort)
	balancingService.SetHttpsPort(spec.HttpsPort)
	balancingService.SetHttpNodePort(spec.HttpNodePort)
	balancingService.SetHttpsNodePort(spec.HttpsNodePort)
	if chytProxy != nil {
		balancingService.SetChytProxyHttpPort(ptr.To(ptr.Deref(chytProxy.HttpPort, int32(consts.HTTPProxyChytHttpPort))))
		balancingService.SetChytProxyHttpNodePort(chytProxy.HttpNodePort)
		balancingService.SetChytProxyHttpsPort(ptr.To(ptr.Deref(chytProxy.HttpsPort, int32(consts.HTTPProxyChytHttpsPort))))
		balancingService.SetChytProxyHttpsNodePort(chytProxy.HttpsNodePort)
	}

	return &HttpProxy{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               masterReconciler,
		serviceType:          spec.ServiceType,
		role:                 spec.Role,
		httpsSecret:          httpsSecret,
		balancingService:     balancingService,
	}
}

func (hp *HttpProxy) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		hp.server,
		hp.balancingService,
	)
}

// needUpdate overrides the server's needUpdate to account for HTTPS secret volumes
// that are added after StatefulSet build
func (hp *HttpProxy) needUpdate() bool {
	return hp.server.needUpdateWithSpecCheck(hp.needStatefulSetSpecUpdate)
}

// needStatefulSetSpecUpdate checks if the StatefulSet spec has changed
func (hp *HttpProxy) needStatefulSetSpecUpdate() bool {
	// Rebuild the StatefulSet with HTTPS secret included
	desiredSpec := hp.server.rebuildStatefulSet().Spec
	hp.applyHttpsSecret(&desiredSpec.Template.Spec)
	return hp.server.getStatefulSet().SpecChanged(desiredSpec)
}

// applyHttpsSecret applies HTTPS secret volume and mount to the pod spec
func (hp *HttpProxy) applyHttpsSecret(podSpec *corev1.PodSpec) {
	if hp.httpsSecret != nil {
		hp.httpsSecret.AddVolume(podSpec)
		hp.httpsSecret.AddVolumeMount(&podSpec.Containers[0])
	}
}

func (hp *HttpProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(hp.ytsaurus.GetClusterState()) && hp.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if hp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, hp.ytsaurus, hp, &hp.localComponent, hp.server, dry); status != nil {
			return *status, err
		}
	}

	masterStatus, err := hp.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !IsRunningStatus(masterStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, hp.master.GetFullName()), err
	}

	if hp.NeedSync() {
		if !dry {
			statefulSet := hp.server.buildStatefulSet()
			hp.applyHttpsSecret(&statefulSet.Spec.Template.Spec)
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

func (hp *HttpProxy) Status(ctx context.Context) (ComponentStatus, error) {
	return hp.doSync(ctx, true)
}

func (hp *HttpProxy) Sync(ctx context.Context) error {
	_, err := hp.doSync(ctx, false)
	return err
}
