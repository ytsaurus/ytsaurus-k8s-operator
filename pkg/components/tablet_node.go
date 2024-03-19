package components

import (
	"context"
	"fmt"

	"go.ytsaurus.tech/library/go/ptr"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

const SysBundle string = "sys"
const DefaultBundle string = "default"

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
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelTabletNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault("TabletNode", spec.Name),
	}

	if spec.InstanceSpec.MonitoringPort == nil {
		spec.InstanceSpec.MonitoringPort = ptr.Int32(consts.TabletNodeMonitoringPort)
	}

	srv := newServer(
		&l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-tablet-node.yson",
		cfgen.GetTabletNodesStatefulSetName(spec.Name),
		cfgen.GetTabletNodesServiceName(spec.Name),
		func() ([]byte, error) {
			return cfgen.GetTabletNodeConfig(spec)
		},
	)

	return &TabletNode{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		initBundlesCondition: "bundlesTabletNodeInitCompleted",
		ytsaurusClient:       ytsaurusClient,
		spec:                 spec,
		doInitialization:     doInitiailization,
	}
}

func (tn *TabletNode) IsUpdatable() bool {
	return true
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

func (tn *TabletNode) Status(ctx context.Context) ComponentStatus {
	if ytv1.IsReadyToUpdateClusterState(tn.ytsaurus.GetClusterState()) && tn.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate)
	}

	if tn.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, tn.ytsaurus, tn, &tn.localComponent, tn.server, true)
		if status != nil {
			if err != nil {
				panic(err)
			}
			return *status
		}
	}

	if tn.NeedSync() {
		return WaitingStatus(SyncStatusPending, "components")
	}

	if !tn.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods")
	}

	if !tn.doInitialization || tn.ytsaurus.IsStatusConditionTrue(tn.initBundlesCondition) {
		return SimpleStatus(SyncStatusReady)
	}

	if tn.ytsaurusClient.Status(ctx).SyncStatus != SyncStatusReady {
		return WaitingStatus(SyncStatusBlocked, tn.ytsaurusClient.GetName())
	}

	return WaitingStatus(SyncStatusPending, fmt.Sprintf("setting %s condition", tn.initBundlesCondition))
}

func (tn *TabletNode) Sync(ctx context.Context) error {
	if ytv1.IsReadyToUpdateClusterState(tn.ytsaurus.GetClusterState()) && tn.server.needUpdate() {
		return nil
	}

	if tn.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		status, err := handleUpdatingClusterState(ctx, tn.ytsaurus, tn, &tn.localComponent, tn.server, false)
		if status != nil {
			return err
		}
	}

	if tn.NeedSync() {
		return tn.server.Sync(ctx)
	}

	if !tn.server.arePodsReady(ctx) {
		return nil
	}

	if !tn.doInitialization || tn.ytsaurus.IsStatusConditionTrue(tn.initBundlesCondition) {
		return nil
	}

	if tn.ytsaurusClient.Status(ctx).SyncStatus != SyncStatusReady {
		return nil
	}

	ytClient := tn.ytsaurusClient.GetYtClient()

	// TODO: refactor it
	if tn.doInitialization {
		if exists, err := ytClient.NodeExists(ctx, ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s", SysBundle)), nil); err == nil {
			if !exists {
				options := map[string]string{
					"changelog_account": "sys",
					"snapshot_account":  "sys",
				}

				bootstrap := tn.getBundleBootstrap(SysBundle)
				if bootstrap != nil {
					if bootstrap.ChangelogPrimaryMedium != nil {
						options["changelog_primary_medium"] = *bootstrap.ChangelogPrimaryMedium
					}
					if bootstrap.SnapshotPrimaryMedium != nil {
						options["snapshot_primary_medium"] = *bootstrap.SnapshotPrimaryMedium
					}
				}

				_, err = ytClient.CreateObject(ctx, yt.NodeTabletCellBundle, &yt.CreateObjectOptions{
					Attributes: map[string]interface{}{
						"name":    SysBundle,
						"options": options,
					},
				})

				if err != nil {
					return fmt.Errorf("creating tablet_cell_bundle failed: %w", err)
				}
			}
		} else {
			return err
		}

		defaultBundleBootstrap := tn.getBundleBootstrap(DefaultBundle)
		if defaultBundleBootstrap != nil {
			path := ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s", DefaultBundle))
			if defaultBundleBootstrap.ChangelogPrimaryMedium != nil {
				err := ytClient.SetNode(ctx, path.Attr("options/changelog_primary_medium"), *defaultBundleBootstrap.ChangelogPrimaryMedium, nil)
				if err != nil {
					return fmt.Errorf("setting changelog_primary_medium for `default` bundle failed: %w", err)

				}
			}

			if defaultBundleBootstrap.SnapshotPrimaryMedium != nil {
				err := ytClient.SetNode(ctx, path.Attr("options/snapshot_primary_medium"), *defaultBundleBootstrap.SnapshotPrimaryMedium, nil)
				if err != nil {
					return fmt.Errorf("setting snapshot_primary_medium for `default` bundle failed: %w", err)
				}
			}
		}

		for _, bundle := range []string{DefaultBundle, SysBundle} {
			tabletCellCount := 1
			bootstrap := tn.getBundleBootstrap(bundle)
			if bootstrap != nil {
				tabletCellCount = bootstrap.TabletCellCount
			}
			err := CreateTabletCells(ctx, ytClient, bundle, tabletCellCount)
			if err != nil {
				return err
			}
		}

		tn.ytsaurus.SetStatusCondition(metav1.Condition{
			Type:    tn.initBundlesCondition,
			Status:  metav1.ConditionTrue,
			Reason:  "InitBundlesCompleted",
			Message: "Init bundles successfully completed",
		})
	}

	return nil
}

func (tn *TabletNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, tn.server)
}
