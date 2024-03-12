package components

import (
	"context"
	"fmt"

	"go.ytsaurus.tech/yt/go/yt"
	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type HttpProxy struct {
	localServerComponent
	cfgen *ytconfig.Generator

	serviceType      corev1.ServiceType
	master           Component
	balancingService *resources.HTTPService

	role        string
	httpsSecret *resources.TLSSecret

	ytClient yt.Client
}

func NewHTTPProxy(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	masterReconciler Component,
	spec ytv1.HTTPProxiesSpec) *HttpProxy {

	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelHTTPProxy, spec.Role),
		ComponentName:  cfgen.FormatComponentStringWithDefault("HttpProxy", spec.Role),
		MonitoringPort: consts.HTTPProxyMonitoringPort,
	}

	srv := newServer(
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
		&l,
		ytsaurus.APIProxy())

	balancingService.SetHttpNodePort(spec.HttpNodePort)
	balancingService.SetHttpsNodePort(spec.HttpsNodePort)

	return &HttpProxy{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               masterReconciler,
		serviceType:          spec.ServiceType,
		role:                 spec.Role,
		httpsSecret:          httpsSecret,
		balancingService:     balancingService,
	}
}

func (hp *HttpProxy) IsUpdatable() bool {
	return true
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

	if !IsRunningStatus(hp.master.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, hp.master.GetName()), err
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

func (hp *HttpProxy) Status(ctx context.Context) ComponentStatus {
	if hp.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	if hp.NeedSync() {
		return WaitingStatus(SyncStatusPending, "components")
	}

	if !resources.Exists(hp.balancingService) {
		return WaitingStatus(SyncStatusPending, hp.balancingService.Name())
	}

	if !hp.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods")
	}

	return SimpleStatus(SyncStatusReady)
}

func (hp *HttpProxy) Sync(ctx context.Context) error {
	var err error

	if hp.server.needUpdate() {
		if err = removePods(ctx, hp.server, &hp.localComponent); err != nil {
			return fmt.Errorf("failed to remove pods: %w", err)
		}
	}

	if hp.NeedSync() {
		statefulSet := hp.server.buildStatefulSet()
		if hp.httpsSecret != nil {
			hp.httpsSecret.AddVolume(&statefulSet.Spec.Template.Spec)
			hp.httpsSecret.AddVolumeMount(&statefulSet.Spec.Template.Spec.Containers[0])
		}
		err = hp.server.Sync(ctx)
		if err != nil {
			return err
		}
	}

	if !resources.Exists(hp.balancingService) {
		s := hp.balancingService.Build()
		s.Spec.Type = hp.serviceType
		err = hp.balancingService.Sync(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}
