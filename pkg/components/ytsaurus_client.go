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
	ptr "k8s.io/utils/pointer"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type YtsaurusClient interface {
	Component2
	GetYtClient() yt.Client
	HandlePossibilityCheck(context.Context) (bool, string, error)
	EnableSafeMode(context.Context) error
	DisableSafeMode(context.Context) error
	IsSafeModeEnabled(context.Context) (bool, error)
	SaveTableCellsAndUpdateState(ctx context.Context) error
	IsTableCellsSaved() bool
	RemoveTableCells(context.Context) error
	RecoverTableCells(context.Context) error
	AreTabletCellsRemoved(context.Context) (bool, error)
	AreTabletCellsRecovered(context.Context) (bool, error)
	IsMasterReadOnly(context.Context) (bool, error)
}

type ytsaurusClient struct {
	componentBase
	httpProxy Component

	initUserJob *InitJob

	secret   *resources.StringSecret
	ytClient yt.Client
}

func NewYtsaurusClient(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	httpProxy Component,
) YtsaurusClient {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelClient,
		ComponentName:  "YtsaurusClient",
		Annotations:    resource.Spec.ExtraPodAnnotations,
	}

	return &ytsaurusClient{
		componentBase: componentBase{
			labeller: &l,
			ytsaurus: ytsaurus,
			cfgen:    cfgen,
		},
		httpProxy: httpProxy,
		initUserJob: NewInitJob(
			&l,
			ytsaurus.APIProxy(),
			ytsaurus,
			ytsaurus.GetResource().Spec.ImagePullSecrets,
			"user",
			consts.ClientConfigFileName,
			resource.Spec.CoreImage,
			cfgen.GetNativeClientConfig),
		secret: resources.NewStringSecret(
			l.GetSecretName(),
			&l,
			ytsaurus.APIProxy()),
	}
}

func (yc *ytsaurusClient) IsUpdatable() bool {
	return false
}

func (yc *ytsaurusClient) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx,
		yc.secret,
		yc.initUserJob,
		yc.httpProxy,
	)
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
	if len(yc.ytsaurus.GetResource().Spec.SecondaryMasters) > 0 {
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
	for _, monitoringPath := range yc.ytsaurus.GetResource().Status.UpdateStatus.MasterMonitoringPaths {
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

func (yc *ytsaurusClient) handleUpdatingState(ctx context.Context) (ComponentStatus, error) {
	var err error

	switch yc.ytsaurus.GetUpdateState() {
	case ytv1.UpdateStatePossibilityCheck:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionHasPossibility) &&
			!yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionNoPossibility) {
			_, _, err = yc.HandlePossibilityCheck(ctx)
			return SimpleStatus(SyncStatusUpdating), err
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
			return SimpleStatus(SyncStatusUpdating), yc.saveTableCells(ctx)
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemovingStart:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemovingStarted) {
			return SimpleStatus(SyncStatusUpdating), yc.RemoveTableCells(ctx)
		}

	case ytv1.UpdateStateWaitingForTabletCellsRemoved:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemoved) {
			removed, err := yc.AreTabletCellsRemoved(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}
			if removed {
				yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionTabletCellsRemoved,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: "Tablet cells were removed",
				})
			}
			return SimpleStatus(SyncStatusUpdating), nil
		}

	case ytv1.UpdateStateWaitingForSnapshots:
		if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSnaphotsSaved) {
			if !yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSnapshotsMonitoringInfoSaved) {

				var monitoringPaths []string
				mastersInfo, err := yc.getAllMasters(ctx)
				if err != nil {
					return SimpleStatus(SyncStatusUpdating), err
				}

				for _, masterInfo := range mastersInfo {
					for _, address := range masterInfo.Addresses {
						monitoringPath := fmt.Sprintf("//sys/cluster_masters/%s/orchid/monitoring/hydra", address)
						monitoringPaths = append(monitoringPaths, monitoringPath)
					}
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
				if err = yc.startBuildMasterSnapshots(ctx); err != nil {
					return SimpleStatus(SyncStatusUpdating), err
				}

				yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
					Type:    consts.ConditionSnapshotsBuildingStarted,
					Status:  metav1.ConditionTrue,
					Reason:  "Update",
					Message: "Snapshots building started",
				})
			}

			for _, monitoringPath := range yc.ytsaurus.GetResource().Status.UpdateStatus.MasterMonitoringPaths {
				var masterHydra MasterHydra
				err = yc.ytClient.GetNode(ctx, ypath.Path(monitoringPath), &masterHydra, getReadOnlyGetOptions())
				if err != nil {
					return SimpleStatus(SyncStatusUpdating), err
				}

				if !masterHydra.LastSnapshotReadOnly {
					return SimpleStatus(SyncStatusUpdating), err
				}
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
			err = yc.RecoverTableCells(ctx)
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
			err = yc.DisableSafeMode(ctx)
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

func (yc *ytsaurusClient) getToken() string {
	token, _ := yc.secret.GetValue(consts.TokenSecretKey)
	return token
}

func (yc *ytsaurusClient) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error
	if !IsRunningStatus(yc.httpProxy.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, yc.httpProxy.GetName()), err
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
			proxy = yc.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)
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

func (yc *ytsaurusClient) Status(ctx context.Context) ComponentStatus {
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
