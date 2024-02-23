package components

import (
	"context"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

type masterCache struct {
	componentBase
	server  server
	initJob *InitJob
}

func NewMasterCache(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelMasterCache,
		ComponentName:  "MasterCache",
		MonitoringPort: consts.MasterCachesMonitoringPort,
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}

	server := newServer(
		&l,
		ytsaurus,
		&resource.Spec.MasterCaches.InstanceSpec,
		"/usr/bin/ytserver-master-cache",
		"ytserver-master-cache.yson",
		cfgen.GetMasterCachesStatefulSetName(),
		cfgen.GetMasterCachesServiceName(),
		cfgen.GetMasterCachesConfig,
	)

	initJob := NewInitJob(
		&l,
		ytsaurus.APIProxy(),
		ytsaurus,
		resource.Spec.ImagePullSecrets,
		"default",
		consts.ClientConfigFileName,
		resource.Spec.CoreImage,
		cfgen.GetNativeClientConfig)

	return &masterCache{
		componentBase: componentBase{
			labeller: &l,
			ytsaurus: ytsaurus,
			cfgen:    cfgen,
		},
		server:  server,
		initJob: initJob,
	}
}

func (mc *masterCache) IsUpdatable() bool {
	return true
}

func (mc *masterCache) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, mc.server)
}

func (mc *masterCache) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(mc.ytsaurus.GetClusterState()) && mc.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate), err
	}

	if mc.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, mc.ytsaurus, mc, &mc.componentBase, mc.server, dry); status != nil {
			return *status, err
		}
	}

	if mc.server.needSync() {
		if !dry {
			err = mc.doServerSync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !mc.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return mc.initJob.Sync(ctx, dry)
}

func (mc *masterCache) Status(ctx context.Context) ComponentStatus {
	status, err := mc.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (mc *masterCache) Sync(ctx context.Context) error {
	_, err := mc.doSync(ctx, false)
	return err
}

func (mc *masterCache) doServerSync(ctx context.Context) error {
	statefulSet := mc.server.buildStatefulSet()
	mc.addAffinity(statefulSet)
	return mc.server.Sync(ctx)
}

func (mc *masterCache) addAffinity(statefulSet *appsv1.StatefulSet) {
	masterCachesSpec := mc.ytsaurus.GetResource().Spec.MasterCaches
	if len(masterCachesSpec.HostAddresses) == 0 {
		return
	}

	affinity := &corev1.Affinity{}
	if statefulSet.Spec.Template.Spec.Affinity != nil {
		affinity = statefulSet.Spec.Template.Spec.Affinity
	}

	nodeAffinity := &corev1.NodeAffinity{}
	if affinity.NodeAffinity != nil {
		nodeAffinity = affinity.NodeAffinity
	}

	selector := &corev1.NodeSelector{}
	if nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		selector = nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	}

	nodeHostnameLabel := mc.getHostAddressLabel()
	selector.NodeSelectorTerms = append(selector.NodeSelectorTerms, corev1.NodeSelectorTerm{
		MatchExpressions: []corev1.NodeSelectorRequirement{
			{
				Key:      nodeHostnameLabel,
				Operator: corev1.NodeSelectorOpIn,
				Values:   masterCachesSpec.HostAddresses,
			},
		},
	})
	nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = selector
	affinity.NodeAffinity = nodeAffinity
	statefulSet.Spec.Template.Spec.Affinity = affinity
}

func (mc *masterCache) getHostAddressLabel() string {
	masterCachesSpec := mc.ytsaurus.GetResource().Spec.MasterCaches
	if masterCachesSpec.HostAddressLabel != "" {
		return masterCachesSpec.HostAddressLabel
	}
	return defaultHostAddressLabel
}
