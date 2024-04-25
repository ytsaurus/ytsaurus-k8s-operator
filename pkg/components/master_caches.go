package components

import (
	"context"

	"go.ytsaurus.tech/library/go/ptr"
	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type MasterCache struct {
	localServerComponent
	cfgen *ytconfig.Generator
}

func NewMasterCache(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *MasterCache {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelMasterCache,
		ComponentName:  string(consts.MasterCacheType),
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}

	if resource.Spec.MasterCaches.InstanceSpec.MonitoringPort == nil {
		resource.Spec.MasterCaches.InstanceSpec.MonitoringPort = ptr.Int32(consts.MasterCachesMonitoringPort)
	}

	srv := newServer(
		&l,
		ytsaurus,
		&resource.Spec.MasterCaches.InstanceSpec,
		"/usr/bin/ytserver-master-cache",
		"ytserver-master-cache.yson",
		cfgen.GetMasterCachesStatefulSetName(),
		cfgen.GetMasterCachesServiceName(),
		func() ([]byte, error) { return cfgen.GetMasterCachesConfig(resource.Spec.MasterCaches) },
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.MasterCachesRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &MasterCache{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
	}
}

func (mc *MasterCache) IsUpdatable() bool {
	return true
}

func (mc *MasterCache) GetType() consts.ComponentType { return consts.MasterCacheType }

func (mc *MasterCache) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, mc.server)
}

func (mc *MasterCache) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(mc.ytsaurus.GetClusterState()) && mc.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if mc.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, mc.ytsaurus, mc, &mc.localComponent, mc.server, dry); status != nil {
			return *status, err
		}
	}

	if mc.NeedSync() {
		if !dry {
			err = mc.doServerSync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !mc.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (mc *MasterCache) Status(ctx context.Context) (ComponentStatus, error) {
	return mc.doSync(ctx, true)
}

func (mc *MasterCache) Sync(ctx context.Context) error {
	_, err := mc.doSync(ctx, false)
	return err
}

func (mc *MasterCache) doServerSync(ctx context.Context) error {
	statefulSet := mc.server.buildStatefulSet()
	masterCachesSpec := mc.ytsaurus.GetResource().Spec.MasterCaches
	if len(masterCachesSpec.HostAddresses) != 0 {
		AddAffinity(statefulSet, mc.getHostAddressLabel(), masterCachesSpec.HostAddresses)
	}

	return mc.server.Sync(ctx)
}

func (mc *MasterCache) getHostAddressLabel() string {
	masterCachesSpec := mc.ytsaurus.GetResource().Spec.MasterCaches
	if masterCachesSpec.HostAddressLabel != "" {
		return masterCachesSpec.HostAddressLabel
	}
	return defaultHostAddressLabel
}
