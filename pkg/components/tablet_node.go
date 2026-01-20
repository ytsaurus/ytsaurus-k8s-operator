package components

import (
	"context"
	"fmt"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

const (
	SysBundle     string = "sys"
	DefaultBundle string = "default"
)

type TabletNode struct {
	localServerComponent
	cfgen *ytconfig.NodeGenerator

	ytsaurusClient internalYtsaurusClient

	initBundlesCondition string
	spec                 ytv1.TabletNodesSpec
	doInitialization     bool
}

func NewTabletNode(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus *apiproxy.Ytsaurus,
	ytsaurusClient internalYtsaurusClient,
	spec ytv1.TabletNodesSpec,
	doInitiailization bool,
) *TabletNode {
	l := cfgen.GetComponentLabeller(consts.TabletNodeType, spec.Name)

	srv := newServer(
		l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-tablet-node.yson",
		func() ([]byte, error) {
			return cfgen.GetTabletNodeConfig(spec)
		},
		consts.TabletNodeMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.TabletNodeRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &TabletNode{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
		initBundlesCondition: "bundlesTabletNodeInitCompleted",
		ytsaurusClient:       ytsaurusClient,
		spec:                 spec,
		doInitialization:     doInitiailization,
	}
}

func (tn *TabletNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(tn.ytsaurus.GetClusterState()) && tn.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedUpdate), err
	}

	if tn.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(tn.ytsaurus, tn) {
			if status, err := handleBulkUpdatingClusterState(ctx, tn.ytsaurus, tn, &tn.localComponent, tn.server, dry); status != nil {
				return *status, err
			}
		} else {
			return ComponentStatusReadyAfter("Not updating component"), nil
		}
	}

	if tn.NeedSync() {
		if !dry {
			err = tn.server.Sync(ctx)
		}

		return ComponentStatusWaitingFor("components"), err
	}

	if !tn.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	if !tn.doInitialization || tn.ytsaurus.IsStatusConditionTrue(tn.initBundlesCondition) {
		return ComponentStatusReady(), err
	}

	ytClientStatus, err := tn.ytsaurusClient.Status(ctx)
	if err != nil {
		return ytClientStatus, err
	}
	if ytClientStatus.SyncStatus != SyncStatusReady {
		return ComponentStatusBlockedBy(tn.ytsaurusClient.GetFullName()), err
	}

	if !dry && tn.doInitialization {
		tabletBundleStatus, err := tn.initBundles(ctx)
		if err != nil {
			return tabletBundleStatus, err
		}
	}

	return ComponentStatusWaitingFor(fmt.Sprintf("setting %s condition", tn.initBundlesCondition)), err
}

func (tn *TabletNode) getBundleBootstrap(bundle string) *ytv1.BundleBootstrapSpec {
	resource := tn.ytsaurus.GetResource()
	if resource.Spec.Bootstrap == nil || resource.Spec.Bootstrap.TabletCellBundles == nil {
		return nil
	}

	if bundle == SysBundle {
		return resource.Spec.Bootstrap.TabletCellBundles.Sys
	}

	if bundle == DefaultBundle {
		return resource.Spec.Bootstrap.TabletCellBundles.Default
	}

	return nil
}

func (tn *TabletNode) getBundleOptions(bundle string) map[string]any {
	options := map[string]any{}

	if bundle == SysBundle {
		options["changelog_account"] = "sys"
		options["snapshot_account"] = "sys"
	}

	bootstrap := tn.getBundleBootstrap(bundle)
	if bootstrap != nil {
		if bootstrap.ChangelogPrimaryMedium != nil {
			options["changelog_primary_medium"] = *bootstrap.ChangelogPrimaryMedium
		}
		if bootstrap.SnapshotPrimaryMedium != nil {
			options["snapshot_primary_medium"] = *bootstrap.SnapshotPrimaryMedium
		}
	}

	if tn.cfgen.GetMaxReplicationFactor() < 3 {
		options["changelog_replication_factor"] = 1
		options["changelog_read_quorum"] = 1
		options["changelog_write_quorum"] = 1
		options["snapshot_replication_factor"] = 1
	}

	return options
}

func (tn *TabletNode) initBundles(ctx context.Context) (ComponentStatus, error) {
	ytClient := tn.ytsaurusClient.GetYtClient()
	logger := log.FromContext(ctx)

	sysBundleExists, err := ytClient.NodeExists(ctx, ypath.Path("//sys/tablet_cell_bundles").Child(SysBundle), nil)
	if err != nil {
		return ComponentStatusWaitingFor("tablet_cell_bundle creation"), err
	}
	if !sysBundleExists {
		options := tn.getBundleOptions(SysBundle)
		_, err = ytClient.CreateObject(ctx, yt.NodeTabletCellBundle, &yt.CreateObjectOptions{
			Attributes: map[string]interface{}{
				"name":    SysBundle,
				"options": options,
			},
		})

		if err != nil {
			logger.Error(err, "Creating tablet_cell_bundle failed")
			return ComponentStatusWaitingFor("tablet_cell_bundle creation"), err
		}
	}

	{
		options := tn.getBundleOptions(DefaultBundle)
		if len(options) != 0 {
			path := ypath.Path("//sys/tablet_cell_bundles").Child(DefaultBundle)
			bundleOptions := map[string]any{}
			err = ytClient.GetNode(ctx, path.Attr("options"), &bundleOptions, nil)
			if err != nil {
				logger.Error(err, "Getting options for `default` bundle failed")
				return ComponentStatusWaitingFor("getting default bundle options"), err
			}
			for option, value := range options {
				bundleOptions[option] = value
			}
			err = ytClient.SetNode(ctx, path.Attr("options"), bundleOptions, nil)
			if err != nil {
				logger.Error(err, "Setting options for `default` bundle failed", "options", options)
				return ComponentStatusWaitingFor("setting default bundle options"), err
			}
		}
	}

	for _, bundle := range []string{DefaultBundle, SysBundle} {
		bootstrap := tn.getBundleBootstrap(bundle)
		if bootstrap != nil && bootstrap.NodeTagFilter != nil {
			// If user don't specify filter in bundle bootstrap (or not specify bootstrap at all),
			// he can expect that we unset previously set filter. However, if filter was set manually
			// not using bootstrap, it will cause unexpected filter unset. So we never unset filters here,
			// assuming that unsets are done only by user.
			path := ypath.Path("//sys/tablet_cell_bundles").Child(bundle).Attr("node_tag_filter")
			err = ytClient.SetNode(ctx, path, *bootstrap.NodeTagFilter, nil)
			if err != nil {
				logger.Error(err, "Setting node tag filter for bundle failed",
					"bundle", bundle,
					"nodeTagFilter", *bootstrap.NodeTagFilter,
				)
				return ComponentStatusWaitingFor(fmt.Sprintf("setting bundle %q node tag filter", bundle)), err
			}
		}
	}

	for _, bundle := range []string{DefaultBundle, SysBundle} {
		tabletCellCount := 1
		bootstrap := tn.getBundleBootstrap(bundle)
		if bootstrap != nil {
			tabletCellCount = bootstrap.TabletCellCount
		}
		err = CreateTabletCells(ctx, ytClient, bundle, tabletCellCount)
		if err != nil {
			return ComponentStatusWaitingFor("tablet cells creation"), err
		}
	}

	tn.ytsaurus.SetStatusCondition(metav1.Condition{
		Type:    tn.initBundlesCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "InitBundlesCompleted",
		Message: "Init bundles successfully completed",
	})

	return ComponentStatusReady(), nil
}

func (tn *TabletNode) Status(ctx context.Context) (ComponentStatus, error) {
	return tn.doSync(ctx, true)
}

func (tn *TabletNode) Sync(ctx context.Context) error {
	_, err := tn.doSync(ctx, false)
	return err
}

func (tn *TabletNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, tn.server)
}
