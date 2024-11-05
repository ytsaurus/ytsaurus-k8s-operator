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

type MasterCache struct {
	localServerComponent
	cfgen *ytconfig.Generator
}

func NewMasterCache(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *MasterCache {
	resource := ytsaurus.Resource()
	l := labeller.Labeller{
		ObjectMeta:    &resource.ObjectMeta,
		ComponentType: consts.MasterCacheType,
		Annotations:   resource.Spec.ExtraPodAnnotations,
	}

	if resource.Spec.MasterCaches.InstanceSpec.MonitoringPort == nil {
		resource.Spec.MasterCaches.InstanceSpec.MonitoringPort = ptr.To(int32(consts.MasterCachesMonitoringPort))
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

func (mc *MasterCache) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, mc.server)
}

func (mc *MasterCache) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(mc.ytsaurus.GetClusterState()) && mc.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if mc.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, mc, dry); status != nil {
			return *status, err
		}
	}

	if ServerNeedSync(mc.server, mc.ytsaurus) {
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

func (mc *MasterCache) doServerSync(ctx context.Context) error {
	statefulSet := mc.server.buildStatefulSet()
	masterCachesSpec := mc.ytsaurus.Resource().Spec.MasterCaches
	if len(masterCachesSpec.HostAddresses) != 0 {
		AddAffinity(statefulSet, mc.getHostAddressLabel(), masterCachesSpec.HostAddresses)
	}

	return mc.server.Sync(ctx)
}

func (mc *MasterCache) getHostAddressLabel() string {
	masterCachesSpec := mc.ytsaurus.Resource().Spec.MasterCaches
	if masterCachesSpec.HostAddressLabel != "" {
		return masterCachesSpec.HostAddressLabel
	}
	return defaultHostAddressLabel
}
