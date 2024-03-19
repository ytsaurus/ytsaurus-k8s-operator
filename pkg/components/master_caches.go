package components

import (
	"context"

	"go.ytsaurus.tech/library/go/ptr"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type masterCache struct {
	localServerComponent
	cfgen *ytconfig.Generator
}

func NewMasterCache(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelMasterCache,
		ComponentName:  "MasterCache",
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
	)

	return &masterCache{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
	}
}

func (mc *masterCache) IsUpdatable() bool {
	return true
}

func (mc *masterCache) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, mc.server)
}

func (mc *masterCache) Status(ctx context.Context) ComponentStatus {
	if ytv1.IsReadyToUpdateClusterState(mc.ytsaurus.GetClusterState()) && mc.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	if mc.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, mc.ytsaurus, mc, &mc.localComponent, mc.server, true)
		if status != nil {
			if err != nil {
				panic(err)
			}
			return *status
		}
	}

	if mc.NeedSync() {
		return WaitingStatus(SyncStatusPending, "components")
	}

	if !mc.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods")
	}

	return SimpleStatus(SyncStatusReady)
}

func (mc *masterCache) Sync(ctx context.Context) error {
	if ytv1.IsReadyToUpdateClusterState(mc.ytsaurus.GetClusterState()) && mc.server.needUpdate() {
		return nil
	}

	if mc.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, mc.ytsaurus, mc, &mc.localComponent, mc.server, false)
		if status != nil {
			return err
		}
	}

	if mc.NeedSync() {
		return mc.doServerSync(ctx)
	}

	if !mc.server.arePodsReady(ctx) {
		return nil
	}

	return nil
}

func (mc *masterCache) doServerSync(ctx context.Context) error {
	statefulSet := mc.server.buildStatefulSet()
	masterCachesSpec := mc.ytsaurus.GetResource().Spec.MasterCaches
	if len(masterCachesSpec.HostAddresses) != 0 {
		AddAffinity(statefulSet, mc.getHostAddressLabel(), masterCachesSpec.HostAddresses)
	}

	return mc.server.Sync(ctx)
}

func (mc *masterCache) getHostAddressLabel() string {
	masterCachesSpec := mc.ytsaurus.GetResource().Spec.MasterCaches
	if masterCachesSpec.HostAddressLabel != "" {
		return masterCachesSpec.HostAddressLabel
	}
	return defaultHostAddressLabel
}
