package components

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type internalYtsaurusClient interface {
	Component
	GetYtClient() yt.Client
}

type YtsaurusClient struct {
	localComponent
	cfgen     *ytconfig.Generator
	httpProxy Component

	initUserJob *InitJob

	secret   *resources.StringSecret
	ytClient yt.Client
}

func NewYtsaurusClient(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	httpProxy Component,
) *YtsaurusClient {
	l := cfgen.GetComponentLabeller(consts.YtsaurusClientType, "")
	resource := ytsaurus.GetResource()
	return &YtsaurusClient{
		localComponent: newLocalComponent(l, ytsaurus),
		cfgen:          cfgen,
		httpProxy:      httpProxy,
		initUserJob: NewInitJob(
			l,
			ytsaurus.APIProxy(),
			ytsaurus,
			ytsaurus.GetResource().Spec.ImagePullSecrets,
			"user",
			consts.ClientConfigFileName,
			resource.Spec.CoreImage,
			cfgen.GetNativeClientConfig,
			resource.Spec.Tolerations,
			resource.Spec.NodeSelector,
			resource.Spec.DNSConfig,
			&resource.Spec.CommonSpec,
		),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			l,
			ytsaurus.APIProxy()),
	}
}

func (yc *YtsaurusClient) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		yc.secret,
		yc.initUserJob,
		yc.httpProxy,
	)
}

func (yc *YtsaurusClient) createInitUserScript() string {
	token, _ := yc.secret.GetValue(consts.TokenSecretKey)
	initJob := initJobWithNativeDriverPrologue()
	return initJob + "\n" + strings.Join(createUserCommand(consts.YtsaurusOperatorUserName, "", token, true), "\n")
}

type TabletCellBundleHealth struct {
	Name   string `yson:",value" json:"name"`
	Health string `yson:"health,attr" json:"health"`
}

type MasterInfo struct {
	CellID    string   `yson:"cell_id" json:"cellId"`
	Addresses []string `yson:"addresses" json:"addresses"`
}

func getReadOnlyGetOptions() *yt.GetNodeOptions {
	return &yt.GetNodeOptions{
		TransactionOptions: &yt.TransactionOptions{
			SuppressUpstreamSync:               true,
			SuppressTransactionCoordinatorSync: true,
		},
	}
}

type MasterState string

const (
	MasterStateLeading   MasterState = "leading"
	MasterStateFollowing MasterState = "following"
)

type MasterHydra struct {
	ReadOnly             bool        `yson:"read_only"`
	LastSnapshotReadOnly bool        `yson:"last_snapshot_read_only"`
	Active               bool        `yson:"active"`
	State                MasterState `yson:"state"`
}

func (yc *YtsaurusClient) getAllMasters(ctx context.Context) ([]MasterInfo, error) {
	var primaryMaster MasterInfo
	err := yc.ytClient.GetNode(ctx, ypath.Path("//sys/@cluster_connection/primary_master"), &primaryMaster, getReadOnlyGetOptions())
	if err != nil {
		return nil, err
	}

	var secondaryMasters []MasterInfo
	if len(yc.ytsaurus.GetResource().Spec.SecondaryMasters) > 0 {
		err = yc.ytClient.GetNode(ctx, ypath.Path("//sys/@cluster_connection/secondary_masters"), &secondaryMasters, getReadOnlyGetOptions())
		if err != nil {
			return nil, err
		}
	}

	return append(secondaryMasters, primaryMaster), nil
}

func (yc *YtsaurusClient) getMasterHydra(ctx context.Context, path string) (MasterHydra, error) {
	var masterHydra MasterHydra
	err := yc.ytClient.GetNode(ctx, ypath.Path(path), &masterHydra, getReadOnlyGetOptions())
	return masterHydra, err
}

func (yc *YtsaurusClient) handleUpdatingState(ctx context.Context) (ComponentStatus, error) {
	var err error

	switch yc.ytsaurus.GetUpdateState() {
	case ytv1.UpdateStatePossibilityCheck:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionHasPossibility) &&
			!yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionNoPossibility) {
			ok, msg, err := yc.HandlePossibilityCheck(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			if !ok {
				yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionNoPossibility,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: msg,
				})
				return SimpleStatus(SyncStatusUpdating), nil
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionHasPossibility,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: msg,
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForSafeModeEnabled:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSafeModeEnabled) {
			err := yc.EnableSafeMode(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionSafeModeEnabled,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Safe mode was enabled",
			})

			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForTabletCellsSaving:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsSaved) {
			tabletCellBundles, err := yc.GetTabletCells(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.GetResource().Status.UpdateStatus.TabletCellBundles = tabletCellBundles

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsSaved,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells were saved",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemovingStart:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemovingStarted) {
			err = yc.RemoveTabletCells(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsRemovingStarted,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells removing was started",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemoved:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemoved) {
			removed, err := yc.AreTabletCellsRemoved(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			if !removed {
				return SimpleStatus(SyncStatusUpdating), nil
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsRemoved,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells were removed",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForImaginaryChunksAbsence:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionRealChunksAttributeEnabled) {
			if err := yc.ensureRealChunkLocationsEnabled(ctx); err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}
			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionRealChunksAttributeEnabled,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Real chunk locations attribute is set to true",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionDataNodesWithImaginaryChunksAbsent) {
			haveImaginaryChunks, err := yc.areActiveDataNodesWithImaginaryChunksExist(ctx)
			if !haveImaginaryChunks {
				// Either no imaginary chunks been found
				// or imaginary chunk nodes pods were removed
				// and master doesn't have online data nodes with imaginary chunks.
				yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionDataNodesWithImaginaryChunksAbsent,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: "No active data nodes with imaginary chunks found",
				})
				return SimpleStatus(SyncStatusUpdating), err
			}

			if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionDataNodesNeedPodsRemoval) {
				// Requesting data nodes to remove pods.
				yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionDataNodesNeedPodsRemoval,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: "Some data nodes have imaginary chunks and need to be restarted",
				})
			}
			// Waiting for data nodes to remove pods and areOnlineDataNodesWithImaginaryChunksExist to return false
			// in the next reconciliations.
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForSnapshots:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSnaphotsSaved) {
			if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSnapshotsMonitoringInfoSaved) {
				monitoringPaths, err := yc.GetMasterMonitoringPaths(ctx)
				if err != nil {
					return SimpleStatus(SyncStatusUpdating), err
				}
				yc.ytsaurus.GetResource().Status.UpdateStatus.MasterMonitoringPaths = monitoringPaths

				yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionSnapshotsMonitoringInfoSaved,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: "Snapshots monitoring info saved",
				})
			}

			if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSnapshotsBuildingStarted) {
				dnds, err := yc.getDataNodesInfo(ctx)
				if err != nil {
					return SimpleStatus(SyncStatusUpdating), err
				}
				log.FromContext(ctx).Info("data nodes before snapshots building", "dataNodes", dnds)
				if err := yc.StartBuildMasterSnapshots(ctx, yc.ytsaurus.GetResource().Status.UpdateStatus.MasterMonitoringPaths); err != nil {
					return SimpleStatus(SyncStatusUpdating), err
				}

				yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionSnapshotsBuildingStarted,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: "Snapshots building started",
				})
			}

			built, err := yc.AreMasterSnapshotsBuilt(ctx, yc.ytsaurus.GetResource().Status.UpdateStatus.MasterMonitoringPaths)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			if !built {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionSnaphotsSaved,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Master snapshots were built",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForTabletCellsRecovery:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRecovered) {
			err = yc.RecoverTableCells(ctx, yc.ytsaurus.GetResource().Status.UpdateStatus.TabletCellBundles)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsRecovered,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells recovered",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForSafeModeDisabled:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSafeModeDisabled) {
			err := yc.DisableSafeMode(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionSafeModeDisabled,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Safe mode disabled",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}
	}

	return SimpleStatus(SyncStatusUpdating), err
}

func (yc *YtsaurusClient) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error
	hpStatus, err := yc.httpProxy.Status(ctx)
	if err != nil {
		return hpStatus, err
	}
	if !IsRunningStatus(hpStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, yc.httpProxy.GetFullName()), err
	}

	if yc.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			s := yc.secret.Build()
			s.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = yc.secret.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, yc.secret.Name()), err
	}

	if !dry {
		yc.initUserJob.SetInitScript(yc.createInitUserScript())
	}
	status, err := yc.initUserJob.Sync(ctx, dry)
	if err != nil || status.SyncStatus != SyncStatusReady {
		return status, err
	}

	if yc.ytClient == nil {
		token, _ := yc.secret.GetValue(consts.TokenSecretKey)
		timeout := time.Second * 10
		proxy, ok := os.LookupEnv("YTOP_PROXY")
		disableProxyDiscovery := true
		if !ok {
			proxy = yc.cfgen.GetHTTPProxiesAddress(&yc.ytsaurus.GetResource().Spec, consts.DefaultHTTPProxyRole)
			disableProxyDiscovery = false
		}
		yc.ytClient, err = ythttp.NewClient(&yt.Config{
			Proxy:                 proxy,
			Token:                 token,
			LightRequestTimeout:   &timeout,
			DisableProxyDiscovery: disableProxyDiscovery,
		})

		if err != nil {
			return WaitingStatus(SyncStatusPending, "ytClient init"), err
		}
	}

	if yc.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if yc.ytsaurus.GetResource().Status.UpdateStatus.State == ytv1.UpdateStateImpossibleToStart {
			return SimpleStatus(SyncStatusReady), err
		}
		if dry {
			return SimpleStatus(SyncStatusUpdating), err
		}
		return yc.handleUpdatingState(ctx)
	}

	return SimpleStatus(SyncStatusReady), err
}

func (yc *YtsaurusClient) Status(ctx context.Context) (ComponentStatus, error) {
	return yc.doSync(ctx, true)
}

func (yc *YtsaurusClient) Sync(ctx context.Context) error {
	_, err := yc.doSync(ctx, false)
	return err
}

func (yc *YtsaurusClient) GetYtClient() yt.Client {
	return yc.ytClient
}

func (yc *YtsaurusClient) HandlePossibilityCheck(ctx context.Context) (ok bool, msg string, err error) {
	// Check tablet cell bundles.
	notGoodBundles, err := GetNotGoodTabletCellBundles(ctx, yc.ytClient)
	if err != nil {
		return
	}

	if len(notGoodBundles) > 0 {
		msg = fmt.Sprintf("Tablet cell bundles (%v) aren't in 'good' health", notGoodBundles)
		return false, msg, nil
	}

	// Check LVC.
	lvcCount := 0
	err = yc.ytClient.GetNode(ctx, ypath.Path("//sys/lost_vital_chunks/@count"), &lvcCount, nil)
	if err != nil {
		return
	}

	if lvcCount > 0 {
		msg = fmt.Sprintf("There are lost vital chunks: %v", lvcCount)
		return false, msg, nil
	}

	// Check QMC.
	qmcCount := 0
	err = yc.ytClient.GetNode(ctx, ypath.Path("//sys/quorum_missing_chunks/@count"), &qmcCount, nil)
	if err != nil {
		return
	}

	if qmcCount > 0 {
		msg = fmt.Sprintf("There are quorum missing chunks: %v", qmcCount)
		return false, msg, nil
	}

	// Check masters.
	primaryMasterAddresses := make([]string, 0)
	err = yc.ytClient.ListNode(ctx, ypath.Path("//sys/primary_masters"), &primaryMasterAddresses, nil)
	if err != nil {
		return
	}

	leadingPrimaryMasterCount := 0
	followingPrimaryMasterCount := 0

	for _, primaryMasterAddress := range primaryMasterAddresses {
		var hydra MasterHydra
		err = yc.ytClient.GetNode(
			ctx,
			ypath.Path(fmt.Sprintf("//sys/primary_masters/%v/orchid/monitoring/hydra", primaryMasterAddress)),
			&hydra,
			nil)
		if err != nil {
			return
		}

		if !hydra.Active {
			msg = fmt.Sprintf("There is a non-active master: %v", primaryMasterAddresses)
			return false, msg, nil
		}

		switch hydra.State {
		case MasterStateLeading:
			leadingPrimaryMasterCount += 1
		case MasterStateFollowing:
			followingPrimaryMasterCount += 1
		}
	}

	if !(leadingPrimaryMasterCount == 1 && followingPrimaryMasterCount+1 == len(primaryMasterAddresses)) {
		msg = "There is no leader or some peer is not active"
		return false, msg, nil
	}

	return true, "Update is possible", nil
}

// Safe mode actions.

func (yc *YtsaurusClient) EnableSafeMode(ctx context.Context) error {
	return yc.ytClient.SetNode(ctx, ypath.Path("//sys/@enable_safe_mode"), true, nil)
}
func (yc *YtsaurusClient) DisableSafeMode(ctx context.Context) error {
	return yc.ytClient.SetNode(ctx, ypath.Path("//sys/@enable_safe_mode"), false, nil)
}

// Tablet cells actions.

func (yc *YtsaurusClient) GetTabletCells(ctx context.Context) ([]ytv1.TabletCellBundleInfo, error) {
	var tabletCellBundles []ytv1.TabletCellBundleInfo
	err := yc.ytClient.ListNode(
		ctx,
		ypath.Path("//sys/tablet_cell_bundles"),
		&tabletCellBundles,
		&yt.ListNodeOptions{Attributes: []string{"tablet_cell_count"}},
	)

	if err != nil {
		return nil, err
	}
	return tabletCellBundles, nil
}
func (yc *YtsaurusClient) RemoveTabletCells(ctx context.Context) error {
	var tabletCells []string
	err := yc.ytClient.ListNode(
		ctx,
		ypath.Path("//sys/tablet_cells"),
		&tabletCells,
		nil)

	if err != nil {
		return err
	}

	for _, tabletCell := range tabletCells {
		err = yc.ytClient.RemoveNode(
			ctx,
			ypath.Path(fmt.Sprintf("//sys/tablet_cells/%s", tabletCell)),
			nil)
		if err != nil {
			return err
		}
	}
	return nil
}
func (yc *YtsaurusClient) AreTabletCellsRemoved(ctx context.Context) (bool, error) {
	var tabletCells []string
	err := yc.ytClient.ListNode(
		ctx,
		ypath.Path("//sys/tablet_cells"),
		&tabletCells,
		nil)

	if err != nil {
		return false, err
	}

	if len(tabletCells) != 0 {
		return false, err
	}
	return true, nil
}
func (yc *YtsaurusClient) RecoverTableCells(ctx context.Context, bundles []ytv1.TabletCellBundleInfo) error {
	for _, bundle := range bundles {
		err := CreateTabletCells(ctx, yc.ytClient, bundle.Name, bundle.TabletCellCount)
		if err != nil {
			return err
		}
	}
	return nil
}

// Master actions.

func (yc *YtsaurusClient) GetMasterMonitoringPaths(ctx context.Context) ([]string, error) {
	var monitoringPaths []string
	mastersInfo, err := yc.getAllMasters(ctx)
	if err != nil {
		return nil, err
	}

	for _, masterInfo := range mastersInfo {
		for _, address := range masterInfo.Addresses {
			monitoringPath := fmt.Sprintf("//sys/cluster_masters/%s/orchid/monitoring/hydra", address)
			monitoringPaths = append(monitoringPaths, monitoringPath)
		}
	}
	return monitoringPaths, nil
}
func (yc *YtsaurusClient) StartBuildMasterSnapshots(ctx context.Context, monitoringPaths []string) error {
	var err error

	allMastersReadOnly := true
	for _, monitoringPath := range monitoringPaths {
		masterHydra, err := yc.getMasterHydra(ctx, monitoringPath)
		if err != nil {
			return err
		}
		if !masterHydra.ReadOnly {
			allMastersReadOnly = false
			break
		}
	}

	if allMastersReadOnly {
		// build_master_snapshot was called before, do nothing.
		return nil
	}

	_, err = yc.ytClient.BuildMasterSnapshots(ctx, &yt.BuildMasterSnapshotsOptions{
		WaitForSnapshotCompletion: ptr.To(false),
		SetReadOnly:               ptr.To(true),
	})

	return err
}
func (yc *YtsaurusClient) AreMasterSnapshotsBuilt(ctx context.Context, monitoringPaths []string) (bool, error) {
	for _, monitoringPath := range monitoringPaths {
		var masterHydra MasterHydra
		err := yc.ytClient.GetNode(ctx, ypath.Path(monitoringPath), &masterHydra, getReadOnlyGetOptions())
		if err != nil {
			return false, err
		}

		if !masterHydra.LastSnapshotReadOnly {
			return false, nil
		}
	}
	return true, nil
}

func (yc *YtsaurusClient) ensureRealChunkLocationsEnabled(ctx context.Context) error {
	logger := log.FromContext(ctx)

	realChunkLocationPath := "//sys/@config/node_tracker/enable_real_chunk_locations"

	pathExists, err := yc.ytClient.NodeExists(ctx, ypath.Path(realChunkLocationPath), nil)
	if err != nil {
		return fmt.Errorf("failed to check if %s exists: %w", realChunkLocationPath, err)
	}
	if !pathExists {
		logger.Info(fmt.Sprintf("%s doesn't exist, this is the case for 24.2+ versions, nothing to do", realChunkLocationPath))
		return nil
	}

	var isChunkLocationsEnabled bool
	err = yc.ytClient.GetNode(ctx, ypath.Path(realChunkLocationPath), &isChunkLocationsEnabled, nil)
	if err != nil {
		return fmt.Errorf("failed to get %s: %w", realChunkLocationPath, err)
	}

	logger.Info(fmt.Sprintf("enable_real_chunk_locations is %t", isChunkLocationsEnabled))

	if isChunkLocationsEnabled {
		return nil
	}

	err = yc.ytClient.SetNode(ctx, ypath.Path(realChunkLocationPath), true, nil)
	if err != nil {
		return fmt.Errorf("failed to set %s: %w", realChunkLocationPath, err)
	}
	logger.Info("enable_real_chunk_locations is set to true")
	return nil
}

type DataNodeMeta struct {
	Name               string `yson:",value"`
	UseImaginaryChunks bool   `yson:"use_imaginary_chunk_locations,attr"`
	State              string `yson:"state,attr"`
}

func (yc *YtsaurusClient) getDataNodesInfo(ctx context.Context) ([]DataNodeMeta, error) {
	var dndMeta []DataNodeMeta
	err := yc.ytClient.ListNode(ctx, ypath.Path("//sys/data_nodes"), &dndMeta, &yt.ListNodeOptions{
		Attributes: []string{
			"use_imaginary_chunk_locations",
			"state",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list data_nodes: %w", err)
	}
	return dndMeta, nil
}

func (yc *YtsaurusClient) areActiveDataNodesWithImaginaryChunksExist(ctx context.Context) (bool, error) {
	dndMeta, err := yc.getDataNodesInfo(ctx)
	if err != nil {
		return false, err
	}

	var activeNodesWithImaginaryChunks []DataNodeMeta
	for _, dnd := range dndMeta {
		if dnd.UseImaginaryChunks && dnd.State != "offline" {
			activeNodesWithImaginaryChunks = append(activeNodesWithImaginaryChunks, dnd)
		}
	}
	logger := log.FromContext(ctx)
	if len(activeNodesWithImaginaryChunks) == 0 {
		logger.Info("There are 0 active nodes with imaginary chunk locations")
		return false, nil
	}
	logger.Info(
		fmt.Sprintf("There are %d active nodes with imaginary chunk locations", len(activeNodesWithImaginaryChunks)),
		"sample", activeNodesWithImaginaryChunks[:1],
	)
	return true, nil
}
