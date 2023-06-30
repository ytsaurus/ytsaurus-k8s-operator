package components

import (
	"context"
	"fmt"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ptr "k8s.io/utils/pointer"
	"strings"
	"time"
)

type YtsaurusClient interface {
	Component
	GetYtClient() yt.Client
}

type ytsaurusClient struct {
	ComponentBase
	httpProxy Component

	initUserJob *InitJob

	secret   *resources.StringSecret
	ytClient yt.Client
}

func NewYtsaurusClient(
	cfgen *ytconfig.Generator,
	apiProxy *apiproxy.APIProxy,
	httpProxy Component,
) YtsaurusClient {
	ytsaurus := apiProxy.Ytsaurus()
	labeller := labeller.Labeller{
		Ytsaurus:       ytsaurus,
		APIProxy:       apiProxy,
		ComponentLabel: consts.YTComponentLabelClient,
		ComponentName:  "YtsaurusClient",
	}

	return &ytsaurusClient{
		ComponentBase: ComponentBase{
			labeller: &labeller,
			apiProxy: apiProxy,
			cfgen:    cfgen,
		},
		httpProxy: httpProxy,
		initUserJob: NewInitJob(
			&labeller,
			apiProxy,
			"user",
			consts.ClientConfigFileName,
			cfgen.GetNativeClientConfig),
		secret: resources.NewStringSecret(
			labeller.GetSecretName(),
			&labeller,
			apiProxy),
	}
}

func (yc *ytsaurusClient) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, []resources.Fetchable{
		yc.secret,
		yc.initUserJob,
		yc.httpProxy,
	})
}

func (yc *ytsaurusClient) createInitUserScript() string {
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

func (yc *ytsaurusClient) getAllMasters(ctx context.Context) ([]MasterInfo, error) {
	var primaryMaster MasterInfo
	err := yc.ytClient.GetNode(ctx, ypath.Path("//sys/@cluster_connection/primary_master"), &primaryMaster, getReadOnlyGetOptions())
	if err != nil {
		return nil, err
	}

	var secondaryMasters []MasterInfo
	if len(yc.apiProxy.Ytsaurus().Spec.SecondaryMasters) > 0 {
		err = yc.ytClient.GetNode(ctx, ypath.Path("//sys/@cluster_connection/secondary_masters"), &secondaryMasters, getReadOnlyGetOptions())
		if err != nil {
			return nil, err
		}
	}

	return append(secondaryMasters, primaryMaster), nil
}

func (yc *ytsaurusClient) getMasterHydra(ctx context.Context, path string) (MasterHydra, error) {
	var masterHydra MasterHydra
	err := yc.ytClient.GetNode(ctx, ypath.Path(path), &masterHydra, getReadOnlyGetOptions())
	return masterHydra, err
}

func (yc *ytsaurusClient) startBuildMasterSnapshots(ctx context.Context) error {
	var err error

	allMastersReadOnly := true
	for _, monitoringPath := range yc.apiProxy.Ytsaurus().Status.UpdateStatus.MasterMonitoringPaths {
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
		WaitForSnapshotCompletion: ptr.Bool(false),
		SetReadOnly:               ptr.Bool(true),
	})

	return err
}

func (yc *ytsaurusClient) handleUpdatingState(ctx context.Context) (SyncStatus, error) {
	var err error

	switch yc.apiProxy.GetUpdateState() {
	case ytv1.UpdateStatePossibilityCheck:
		if !yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionHasPossibility) &&
			!yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionNoPossibility) {

			// Check tablet cell bundles.
			notGoodBundles, err := GetNotGoodTabletCellBundles(ctx, yc.ytClient)

			if err != nil {
				return SyncStatusUpdating, err
			}

			if len(notGoodBundles) > 0 {
				err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionNoPossibility,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: fmt.Sprintf("Tablet cell bundles (%v) aren't in 'good' health", notGoodBundles),
				})
				return SyncStatusUpdating, err
			}

			// Check LVC.
			lvcCount := 0
			err = yc.ytClient.GetNode(ctx, ypath.Path("//sys/lost_vital_chunks/@count"), &lvcCount, nil)
			if err != nil {
				return SyncStatusUpdating, err
			}

			if lvcCount > 0 {
				err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionNoPossibility,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: fmt.Sprintf("There are lost vital chunks: %v", lvcCount),
				})
				return SyncStatusUpdating, err
			}

			// Check QMC.
			qmcCount := 0
			err = yc.ytClient.GetNode(ctx, ypath.Path("//sys/quorum_missing_chunks/@count"), &qmcCount, nil)
			if err != nil {
				return SyncStatusUpdating, err
			}

			if qmcCount > 0 {
				err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionNoPossibility,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: fmt.Sprintf("There are quorum missing chunks: %v", qmcCount),
				})
				return SyncStatusUpdating, err
			}

			// Check masters.
			primaryMasterAddresses := make([]string, 0)
			err = yc.ytClient.ListNode(ctx, ypath.Path("//sys/primary_masters"), &primaryMasterAddresses, nil)
			if err != nil {
				return SyncStatusUpdating, err
			}

			leadingPrimaryMasterCount := 0
			followingPrimaryMasterCount := 0

			for _, primaryMasterAddress := range primaryMasterAddresses {
				var hydra MasterHydra
				err := yc.ytClient.GetNode(
					ctx,
					ypath.Path(fmt.Sprintf("//sys/primary_masters/%v/orchid/monitoring/hydra", primaryMasterAddress)),
					&hydra,
					nil)
				if err != nil {
					return SyncStatusUpdating, err
				}

				if !hydra.Active {
					err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
						Type:    consts.ConditionNoPossibility,
						Status:  metav1.ConditionTrue,
						Reason:  "Update",
						Message: fmt.Sprintf("There is a non-active master: %v", primaryMasterAddresses),
					})
					return SyncStatusUpdating, err
				}

				switch hydra.State {
				case MasterStateLeading:
					leadingPrimaryMasterCount += 1
				case MasterStateFollowing:
					followingPrimaryMasterCount += 1
				}
			}

			if !(leadingPrimaryMasterCount == 1 && followingPrimaryMasterCount+1 == len(primaryMasterAddresses)) {
				err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionNoPossibility,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: fmt.Sprintf("There is no leader or some peer is not active"),
				})
				return SyncStatusUpdating, err
			}

			err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionHasPossibility,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Update is possible",
			})
			return SyncStatusUpdating, err
		}

	case ytv1.UpdateStateWaitingForSafeModeEnabled:
		if !yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionSafeModeEnabled) {
			err := yc.ytClient.SetNode(ctx, ypath.Path("//sys/@enable_safe_mode"), true, nil)
			if err != nil {
				return SyncStatusUpdating, err
			}

			err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionSafeModeEnabled,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Safe mode was enabled",
			})

			return SyncStatusUpdating, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsSaving:
		if !yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsSaved) {
			var tabletCellBundles []ytv1.TabletCellBundleInfo
			err := yc.ytClient.ListNode(
				ctx,
				ypath.Path("//sys/tablet_cell_bundles"),
				&tabletCellBundles,
				&yt.ListNodeOptions{Attributes: []string{"tablet_cell_count"}})

			if err != nil {
				return SyncStatusUpdating, err
			}

			yc.apiProxy.Ytsaurus().Status.UpdateStatus.TabletCellBundles = tabletCellBundles
			err = yc.apiProxy.UpdateStatus(ctx)

			if err != nil {
				return SyncStatusUpdating, err
			}

			err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsSaved,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells were saved",
			})
			return SyncStatusUpdating, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemovingStart:
		if !yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemovingStarted) {

			var tabletCells []string
			err := yc.ytClient.ListNode(
				ctx,
				ypath.Path("//sys/tablet_cells"),
				&tabletCells,
				nil)

			if err != nil {
				return SyncStatusUpdating, err
			}

			for _, tabletCell := range tabletCells {
				err := yc.ytClient.RemoveNode(
					ctx,
					ypath.Path(fmt.Sprintf("//sys/tablet_cells/%s", tabletCell)),
					nil)
				if err != nil {
					return SyncStatusUpdating, err
				}
			}

			err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsRemovingStarted,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells removing was started",
			})
			return SyncStatusUpdating, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemoved:
		if !yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemoved) {
			var tabletCells []string
			err := yc.ytClient.ListNode(
				ctx,
				ypath.Path("//sys/tablet_cells"),
				&tabletCells,
				nil)

			if err != nil {
				return SyncStatusUpdating, err
			}

			if len(tabletCells) != 0 {
				return SyncStatusUpdating, err
			}

			err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsRemoved,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells were removed",
			})
			return SyncStatusUpdating, err
		}

	case ytv1.UpdateStateWaitingForSnapshots:
		if !yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionSnaphotsSaved) {
			if !yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionSnapshotsMonitoringInfoSaved) {

				var monitoringPaths []string
				mastersInfo, err := yc.getAllMasters(ctx)
				if err != nil {
					return SyncStatusUpdating, err
				}

				for _, masterInfo := range mastersInfo {
					for _, address := range masterInfo.Addresses {
						monitoringPath := fmt.Sprintf("//sys/cluster_masters/%s/orchid/monitoring/hydra", address)
						monitoringPaths = append(monitoringPaths, monitoringPath)
					}
				}

				yc.apiProxy.Ytsaurus().Status.UpdateStatus.MasterMonitoringPaths = monitoringPaths
				err = yc.apiProxy.UpdateStatus(ctx)

				if err != nil {
					return SyncStatusUpdating, err
				}

				err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionSnapshotsMonitoringInfoSaved,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: "Snapshots monitoring info saved",
				})
			}

			if !yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionSnapshotsBuildingStarted) {
				if err = yc.startBuildMasterSnapshots(ctx); err != nil {
					return SyncStatusUpdating, err
				}

				err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionSnapshotsBuildingStarted,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: "Snapshots building started",
				})
			}

			for _, monitoringPath := range yc.apiProxy.Ytsaurus().Status.UpdateStatus.MasterMonitoringPaths {
				var masterHydra MasterHydra
				err = yc.ytClient.GetNode(ctx, ypath.Path(monitoringPath), &masterHydra, getReadOnlyGetOptions())
				if err != nil {
					return SyncStatusUpdating, err
				}

				if !masterHydra.LastSnapshotReadOnly {
					return SyncStatusUpdating, err
				}
			}

			err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionSnaphotsSaved,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Master snapshots were built",
			})
			return SyncStatusUpdating, err
		}

	case ytv1.UpdateStateWaitingForTabletCellsRecovery:
		if !yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRecovered) {

			for _, bundle := range yc.apiProxy.Ytsaurus().Status.UpdateStatus.TabletCellBundles {
				err = CreateTabletCells(ctx, yc.ytClient, bundle.Name, bundle.TabletCellCount)
				if err != nil {
					return SyncStatusUpdating, err
				}
			}

			err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsRecovered,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells recovered",
			})
			return SyncStatusUpdating, err
		}

	case ytv1.UpdateStateWaitingForSafeModeDisabled:
		if !yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionSafeModeDisabled) {
			err := yc.ytClient.SetNode(ctx, ypath.Path("//sys/@enable_safe_mode"), false, nil)
			if err != nil {
				return SyncStatusUpdating, err
			}

			err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionSafeModeDisabled,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Safe mode disabled",
			})
			return SyncStatusUpdating, err
		}
	}

	return SyncStatusUpdating, err
}

func (yc *ytsaurusClient) doSync(ctx context.Context, dry bool) (SyncStatus, error) {
	var err error
	if !(yc.httpProxy.Status(ctx) == SyncStatusReady) {
		return SyncStatusBlocked, err
	}

	if yc.secret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			s := yc.secret.Build()
			s.StringData = map[string]string{
				consts.TokenSecretKey: ytconfig.RandString(30),
			}
			err = yc.secret.Sync(ctx)
		}
		return SyncStatusPending, err
	}

	if !dry {
		yc.initUserJob.SetInitScript(yc.createInitUserScript())
	}
	status, err := yc.initUserJob.Sync(ctx, dry)
	if err != nil || status != SyncStatusReady {
		return status, err
	}

	if yc.ytClient == nil {
		token, _ := yc.secret.GetValue(consts.TokenSecretKey)
		timeout := time.Second * 10
		yc.ytClient, err = ythttp.NewClient(&yt.Config{
			Proxy:               yc.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole),
			Token:               token,
			LightRequestTimeout: &timeout,
		})

		if err != nil {
			return SyncStatusPending, err
		}
	}

	if yc.apiProxy.GetClusterState() == ytv1.ClusterStateUpdating {
		if dry {
			return SyncStatusUpdating, err
		}
		return yc.handleUpdatingState(ctx)
	}

	return SyncStatusReady, err
}

func (yc *ytsaurusClient) Status(ctx context.Context) SyncStatus {
	status, err := yc.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (yc *ytsaurusClient) Sync(ctx context.Context) error {
	_, err := yc.doSync(ctx, false)
	return err
}

func (yc *ytsaurusClient) GetYtClient() yt.Client {
	return yc.ytClient
}
