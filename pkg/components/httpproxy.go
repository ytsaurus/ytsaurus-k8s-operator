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
	serverComponent

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
		ytsaurus)

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
		serverComponent:  newLocalServerComponent(l, ytsaurus, srv),
		cfgen:            cfgen,
		master:           masterReconciler,
		serviceType:      spec.ServiceType,
		role:             spec.Role,
		httpsSecret:      httpsSecret,
		balancingService: balancingService,
	}
}

func (hp *HttpProxy) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		hp.server,
		hp.balancingService,
	)
}

func (hp *HttpProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if hp.ytsaurus.IsReadyToUpdate() && hp.NeedUpdate() {
		return SimpleStatus(SyncStatusNeedUpdate), err
	}

	if hp.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(hp.ytsaurus, hp) {
			switch getComponentUpdateStrategy(hp.ytsaurus, consts.HttpProxyType, hp.GetShortName()) {
			case ytv1.ComponentUpdateModeTypeOnDelete:
				if status, err := handleOnDeleteUpdatingClusterState(ctx, hp.ytsaurus, hp, &hp.component, hp.server, dry); status != nil {
					return *status, err
				}
			default:
				if status, err := handleBulkUpdatingClusterState(ctx, hp.ytsaurus, hp, &hp.component, hp.server, dry); status != nil {
					return *status, err
				}
			}

			if hp.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation {
				return ComponentStatusReady(), err
			}
		} else {
			return ComponentStatusReadyAfter("Not updating component"), nil
		}
	}

	masterStatus, err := hp.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !masterStatus.IsRunning() {
		return ComponentStatusBlockedBy(hp.master.GetFullName()), err
	}

	if hp.NeedSync() {
		if !dry {
			statefulSet := hp.server.buildStatefulSet()
			if hp.httpsSecret != nil {
				hp.httpsSecret.AddVolume(&statefulSet.Spec.Template.Spec)
				hp.httpsSecret.AddVolumeMount(&statefulSet.Spec.Template.Spec.Containers[0])
			}
			err = hp.server.Sync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if !hp.balancingService.Exists() {
		if !dry {
			s := hp.balancingService.Build()
			s.Spec.Type = hp.serviceType
			err = hp.balancingService.Sync(ctx)
		}
		return ComponentStatusWaitingFor(hp.balancingService.Name()), err
	}

	if !hp.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	return ComponentStatusReady(), err
}

func (hp *HttpProxy) Status(ctx context.Context) (ComponentStatus, error) {
	return hp.doSync(ctx, true)
}

func (hp *HttpProxy) Sync(ctx context.Context) error {
	_, err := hp.doSync(ctx, false)
	return err
}
