package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type MasterCache struct {
	localServerComponent
	cfgen *ytconfig.Generator
}

func NewMasterCache(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *MasterCache {
	l := cfgen.GetComponentLabeller(consts.MasterCacheType, "")

	resource := ytsaurus.GetResource()
	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.MasterCaches.InstanceSpec,
		"/usr/bin/ytserver-master-cache",
		"ytserver-master-cache.yson",
		func() ([]byte, error) { return cfgen.GetMasterCachesConfig(resource.Spec.MasterCaches) },
		consts.MasterCachesMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.MasterCachesRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &MasterCache{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
	}
}

func (mc *MasterCache) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, mc.server)
}

func (mc *MasterCache) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(mc.ytsaurus.GetClusterState()) && mc.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedUpdate), err
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
		return ComponentStatusWaitingFor("components"), err
	}

	if !mc.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	return ComponentStatusReady(), err
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

func (mc *MasterCache) HasCustomUpdateState() bool {
	return false
}
