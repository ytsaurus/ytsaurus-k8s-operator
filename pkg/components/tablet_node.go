package components

import (
	"context"
	"fmt"

	"go.ytsaurus.tech/library/go/ptr"
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

var (
	tnTabletCellsBackupStartedCond  = isTrue("TabletCellsBackupStarted")
	tnTabletCellsBackupFinishedCond = isTrue("TabletCellsBackupFinished")
	tnTabletCellsRecoveredCond      = isTrue("TabletCellsRecovered")
)

type ytsaurusClientForTabletNodes interface {
	GetYtClient() yt.Client

	GetTabletCells(context.Context) ([]ytv1.TabletCellBundleInfo, error)
	RemoveTabletCells(context.Context) error
	RecoverTableCells(context.Context, []ytv1.TabletCellBundleInfo) error
	AreTabletCellsRemoved(context.Context) (bool, error)
}

type TabletNode struct {
	localServerComponent
	cfgen *ytconfig.NodeGenerator

	ytsaurusClient ytsaurusClientForTabletNodes

	initBundlesCondition string
	spec                 ytv1.TabletNodesSpec
	doInitialization     bool
}

func NewTabletNode(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus *apiproxy.Ytsaurus,
	ytsaurusClient ytsaurusClientForTabletNodes,
	spec ytv1.TabletNodesSpec,
	doInitiailization bool,
) *TabletNode {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelTabletNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault(string(consts.TabletNodeType), spec.Name),
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

func (tn *TabletNode) GetType() consts.ComponentType { return consts.TabletNodeType }

func (tn *TabletNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
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

	//if tn.ytsaurusClient.Status(ctx).SyncStatus != SyncStatusReady {
	//	return WaitingStatus(SyncStatusBlocked, tn.ytsaurusClient.GetName()), err
	//}

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

func (tn *TabletNode) initializeBundles(ctx context.Context) error {
	ytClient := tn.ytsaurusClient.GetYtClient()

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
				return fmt.Errorf("Setting snapshot_primary_medium for `default` bundle failed: %w", err)
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

	return nil

	//tn.ytsaurus.SetStatusCondition(metav1.Condition{
	//	Type:    tn.initBundlesCondition,
	//	Status:  metav1.ConditionTrue,
	//	Reason:  "InitBundlesCompleted",
	//	Message: "Init bundles successfully completed",
	//})

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

func (tn *TabletNode) Status(ctx context.Context) (ComponentStatus, error) {
	if err := tn.Fetch(ctx); err != nil {
		return ComponentStatus{}, fmt.Errorf("failed to fetch component %s: %w", tn.GetName(), err)
	}

	st, msg, err := tn.getFlow().Status(ctx, tn.condManager)
	return ComponentStatus{
		SyncStatus: st,
		Message:    msg,
	}, err
}

func (tn *TabletNode) StatusOld(ctx context.Context) ComponentStatus {
	status, err := tn.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (tn *TabletNode) Sync(ctx context.Context) error {
	_, err := tn.getFlow().Run(ctx, tn.condManager)
	return err
}

func (tn *TabletNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, tn.server)
}

func (tn *TabletNode) getFlow() Step {
	name := tn.GetName()
	buildStartedCond := buildStarted(name)
	builtFinishedCond := buildFinished(name)
	initCond := initializationFinished(name)
	updateRequiredCond := updateRequired(name)
	rebuildStartedCond := rebuildStarted(name)
	rebuildFinishedCond := rebuildFinished(name)

	return StepComposite{
		Steps: []Step{
			StepRun{
				Name:               StepStartBuild,
				RunIfCondition:     not(buildStartedCond),
				RunFunc:            tn.server.Sync,
				OnSuccessCondition: buildStartedCond,
			},
			StepCheck{
				Name:               StepWaitBuildFinished,
				RunIfCondition:     not(builtFinishedCond),
				OnSuccessCondition: builtFinishedCond,
				RunFunc: func(ctx context.Context) (ok bool, err error) {
					diff, err := tn.server.hasDiff(ctx)
					return !diff, err
				},
			},
			StepRun{
				Name:               StepInitFinished,
				RunIfCondition:     not(initCond),
				OnSuccessCondition: initCond,
				RunFunc: func(ctx context.Context) error {
					return tn.initializeBundles(ctx)
				},
			},
			StepComposite{
				Name: StepUpdate,
				// Update should be run if either diff exists or updateRequired condition is set,
				// because a diff should disappear in the middle of the update, but it still need
				// to finish actions after the update (master exit read only, safe mode, etc.).
				StatusConditionFunc: func(ctx context.Context) (SyncStatus, string, error) {
					diff, err := tn.server.hasDiff(ctx)
					if err != nil {
						return "", "", err
					}
					if diff {
						if err = tn.condManager.SetCond(ctx, updateRequiredCond); err != nil {
							return "", "", err
						}
					}
					// Sync either if diff or is condition set
					// in the middle of update there will be no diff, so we need a condition.
					if diff || tn.condManager.IsSatisfied(updateRequiredCond) {
						return SyncStatusNeedSync, "", nil
					}
					return SyncStatusReady, "", nil
				},
				OnSuccessCondition: not(updateRequiredCond),
				OnSuccessFunc:      tn.cleanupAfterUpdate,
				Steps: []Step{
					StepRun{
						Name:               "SaveTabletCellBundles",
						RunIfCondition:     not(tnTabletCellsBackupStartedCond),
						OnSuccessCondition: tnTabletCellsBackupStartedCond,
						RunFunc: func(ctx context.Context) error {
							bundles, err := tn.ytsaurusClient.GetTabletCells(ctx)
							if err != nil {
								return err
							}
							if err = tn.storeTabletCellBundles(ctx, bundles); err != nil {
								return err
							}
							return tn.ytsaurusClient.RemoveTabletCells(ctx)
						},
					},
					StepCheck{
						Name:               "CheckTabletCellsRemoved",
						RunIfCondition:     not(tnTabletCellsBackupFinishedCond),
						OnSuccessCondition: tnTabletCellsBackupFinishedCond,
						RunFunc: func(ctx context.Context) (ok bool, err error) {
							return tn.ytsaurusClient.AreTabletCellsRemoved(ctx)
						},
					},
					StepRun{
						Name:               StepStartRebuild,
						RunIfCondition:     not(rebuildStartedCond),
						OnSuccessCondition: rebuildStartedCond,
						RunFunc:            tn.server.removePods,
					},
					StepCheck{
						Name:               StepWaitRebuildFinished,
						RunIfCondition:     not(rebuildFinishedCond),
						OnSuccessCondition: rebuildFinishedCond,
						RunFunc: func(ctx context.Context) (ok bool, err error) {
							diff, err := tn.server.hasDiff(ctx)
							return !diff, err
						},
					},
					StepRun{
						Name:               "RecoverTableCells",
						RunIfCondition:     not(tnTabletCellsRecoveredCond),
						OnSuccessCondition: tnTabletCellsRecoveredCond,
						RunFunc: func(ctx context.Context) error {
							bundles := tn.getStoredTabletCellBundles()
							return tn.ytsaurusClient.RecoverTableCells(ctx, bundles)
						},
					},
				},
			},
		},
	}
}

func (tn *TabletNode) cleanupAfterUpdate(ctx context.Context) error {
	for _, cond := range tn.getConditionsSetByUpdate() {
		if err := tn.condManager.SetCond(ctx, not(cond)); err != nil {
			return err
		}
	}
	return nil
}

func (tn *TabletNode) getConditionsSetByUpdate() []Condition {
	var result []Condition
	conds := []Condition{
		rebuildStarted(tn.GetName()),
		rebuildFinished(tn.GetName()),
		tnTabletCellsBackupStartedCond,
		tnTabletCellsBackupFinishedCond,
		tnTabletCellsRecoveredCond,
	}
	for _, cond := range conds {
		if tn.condManager.IsSatisfied(cond) {
			result = append(result, cond)
		}
	}
	return result
}

func (tn *TabletNode) storeTabletCellBundles(ctx context.Context, bundles []ytv1.TabletCellBundleInfo) error {
	return tn.stateManager.SetTabletCellBundles(ctx, bundles)
}

func (tn *TabletNode) getStoredTabletCellBundles() []ytv1.TabletCellBundleInfo {
	return tn.stateManager.GetTabletCellBundles()
}
