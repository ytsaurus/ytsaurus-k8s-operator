package components

import (
	"context"
	"fmt"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

const SysBundle string = "sys"
const DefaultBundle string = "default"

type tabletNode struct {
	localServerComponent
	cfgen *ytconfig.NodeGenerator

	ytsaurusClient YtsaurusClient

	initBundlesCondition string
	spec                 ytv1.TabletNodesSpec
	doInitialization     bool
}

func NewTabletNode(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus *apiproxy.Ytsaurus,
	ytsaurusClient YtsaurusClient,
	spec ytv1.TabletNodesSpec,
	doInitiailization bool,
) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelTabletNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault("TabletNode", spec.Name),
		MonitoringPort: consts.TabletNodeMonitoringPort,
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

	return &tabletNode{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		initBundlesCondition: "bundlesTabletNodeInitCompleted",
		ytsaurusClient:       ytsaurusClient,
		spec:                 spec,
		doInitialization:     doInitiailization,
	}
}

func (tn *tabletNode) IsUpdatable() bool {
	return true
}

func (tn *tabletNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error
	logger := log.FromContext(ctx)

	if ytv1.IsReadyToUpdateClusterState(tn.ytsaurus.GetClusterState()) && tn.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate), err
	}

	if tn.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, tn.ytsaurus, tn, &tn.localComponent, tn.server, dry); status != nil {
			return *status, err
		}
	}

	if tn.NeedSync() {
		if !dry {
			err = tn.server.Sync(ctx)
		}

		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !tn.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	if !tn.doInitialization || tn.ytsaurus.IsStatusConditionTrue(tn.initBundlesCondition) {
		return SimpleStatus(SyncStatusReady), err
	}

	if tn.ytsaurusClient.Status(ctx).SyncStatus != SyncStatusReady {
		return WaitingStatus(SyncStatusBlocked, tn.ytsaurusClient.GetName()), err
	}

	ytClient := tn.ytsaurusClient.GetYtClient()

	if !dry {
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
						logger.Error(err, "Creating tablet_cell_bundle failed")
						return WaitingStatus(SyncStatusPending, "tablet_cell_bundle creation"), err
					}
				}
			} else {
				return WaitingStatus(SyncStatusPending, "tablet_cell_bundle creation"), err
			}

			defaultBundleBootstrap := tn.getBundleBootstrap(DefaultBundle)
			if defaultBundleBootstrap != nil {
				path := ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s", DefaultBundle))
				if defaultBundleBootstrap.ChangelogPrimaryMedium != nil {
					err := ytClient.SetNode(ctx, path.Attr("options/changelog_primary_medium"), *defaultBundleBootstrap.ChangelogPrimaryMedium, nil)
					if err != nil {
						logger.Error(err, "Setting changelog_primary_medium for `default` bundle failed")
						return WaitingStatus(SyncStatusPending, "setting changelog_primary_medium"), err
					}
				}

				if defaultBundleBootstrap.SnapshotPrimaryMedium != nil {
					err := ytClient.SetNode(ctx, path.Attr("options/snapshot_primary_medium"), *defaultBundleBootstrap.SnapshotPrimaryMedium, nil)
					if err != nil {
						logger.Error(err, "Setting snapshot_primary_medium for `default` bundle failed")
						return WaitingStatus(SyncStatusPending, "setting snapshot_primary_medium"), err
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
					return WaitingStatus(SyncStatusPending, "tablet cells creation"), err
				}
			}

			tn.ytsaurus.SetStatusCondition(metav1.Condition{
				Type:    tn.initBundlesCondition,
				Status:  metav1.ConditionTrue,
				Reason:  "InitBundlesCompleted",
				Message: "Init bundles successfully completed",
			})
		}
	}

	return WaitingStatus(SyncStatusPending, fmt.Sprintf("setting %s condition", tn.initBundlesCondition)), err
}

func (tn *tabletNode) getBundleBootstrap(bundle string) *ytv1.BundleBootstrapSpec {
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

func (tn *tabletNode) Status(ctx context.Context) ComponentStatus {
	status, err := tn.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (tn *tabletNode) Sync(ctx context.Context) error {
	_, err := tn.doSync(ctx, false)
	return err
}

func (tn *tabletNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, tn.server)
}
