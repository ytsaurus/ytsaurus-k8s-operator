package components

import (
	"context"

	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"

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

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.To(int32(consts.HTTPProxyMonitoringPort))
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
		WithContainerPorts(
			corev1.ContainerPort{
				Name:          consts.YTRPCPortName,
				ContainerPort: consts.HTTPProxyRPCPort,
				Protocol:      corev1.ProtocolTCP,
			},
			corev1.ContainerPort{
				Name:          "http",
				ContainerPort: consts.HTTPProxyHTTPPort,
				Protocol:      corev1.ProtocolTCP,
			},
			corev1.ContainerPort{
				Name:          "https",
				ContainerPort: consts.HTTPProxyHTTPSPort,
				Protocol:      corev1.ProtocolTCP,
			},
		),
		WithCustomReadinessProbeEndpointPort(consts.HTTPProxyHTTPPort),
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

	balancingService.SetHttpNodePort(spec.HttpNodePort)
	balancingService.SetHttpsNodePort(spec.HttpsNodePort)

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

func (hp *HttpProxy) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(hp.ytsaurus.GetClusterState()) && hp.server.needUpdate() {
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
			if hp.httpsSecret != nil {
				hp.httpsSecret.AddVolume(&statefulSet.Spec.Template.Spec)
				hp.httpsSecret.AddVolumeMount(&statefulSet.Spec.Template.Spec.Containers[0])
			}
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
