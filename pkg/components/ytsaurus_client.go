package components

import (
	"context"
	"fmt"
	v1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
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
	})
}

func (yc *ytsaurusClient) createInitUserScript() string {
	token, _ := yc.secret.GetValue(consts.TokenSecretKey)
	initJob := initJobWithNativeDriverPrologue()
	return initJob + "\n" + strings.Join(createUserCommand(consts.YtsaurusOperatorUserName, "", token, true), "\n")
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
		yc.ytClient, err = ythttp.NewClient(&yt.Config{
			Proxy: yc.cfgen.GetHTTPProxiesAddress(),
			Token: token,
		})

		if err != nil {
			return SyncStatusPending, err
		}
	}

	if yc.apiProxy.GetClusterState() == v1.ClusterStateUpdating {
		if dry {
			return SyncStatusUpdating, err
		}

		if yc.apiProxy.GetUpdateState() == v1.UpdateStateWaitingForSafeModeEnabled &&
			!yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionSafeModeEnabled) {

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

		if yc.apiProxy.GetUpdateState() == v1.UpdateStateWaitingForTabletCellsSaving &&
			!yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsSaved) {

			var tabletCellBundles []v1.TabletCellBundleInfo
			err := yc.ytClient.ListNode(
				ctx,
				ypath.Path("//sys/tablet_cell_bundles"),
				&tabletCellBundles,
				&yt.ListNodeOptions{Attributes: []string{"tablet_cell_count"}})

			if err != nil {
				return SyncStatusUpdating, err
			}

			yc.apiProxy.Ytsaurus().Status.SavedTabletCellBundles = tabletCellBundles
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

		if yc.apiProxy.GetUpdateState() == v1.UpdateStateWaitingForTabletCellsRemovingStart &&
			!yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemovingStarted) {

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

		if yc.apiProxy.GetUpdateState() == v1.UpdateStateWaitingForTabletCellsRemoved &&
			!yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRemoved) {

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

		if yc.apiProxy.GetUpdateState() == v1.UpdateStateWaitingForSnapshots &&
			!yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionSnaphotsSaved) {

			// TODO: call BuildMasterSnapshots and wait until finished

			err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionSnaphotsSaved,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Master snapshots were built",
			})
			return SyncStatusUpdating, err
		}

		if yc.apiProxy.GetUpdateState() == v1.UpdateStateWaitingForTabletCellsRecovery &&
			!yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsRecovered) {

			// TODO: recreate tablet cells

			err = yc.apiProxy.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTabletCellsRecovered,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Tablet cells recovered",
			})
			return SyncStatusUpdating, err
		}

		if yc.apiProxy.GetUpdateState() == v1.UpdateStateWaitingForSafeModeDisabled &&
			!yc.apiProxy.IsUpdateStatusConditionTrue(consts.ConditionSafeModeDisabled) {

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
		return SyncStatusUpdating, err
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
