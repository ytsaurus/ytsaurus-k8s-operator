package components

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func (s ComponentStatus) IsReady() bool {
	return s.SyncStatus == SyncStatusReady
}

// needSync2 is a copy of needSync but without ytsaurus status checking
// (server resources shouldn't know about parent ytsaurus component)
func (s *serverImpl) needSync2() bool {
	needReload, err := s.configHelper.NeedReload()
	if err != nil {
		needReload = false
	}
	return s.configHelper.NeedInit() ||
		needReload ||
		!s.exists() ||
		s.statefulSet.NeedSync(s.instanceSpec.InstanceCount)
}

type Component2 interface {
	Component
	Status2(context.Context) (ComponentStatus, error)
	Sync2(context.Context) error
}

type YtsaurusClient2 interface {
	YtsaurusClient
	Component2
	HandlePossibilityCheck(context.Context) (bool, string, error)
	EnableSafeMode(context.Context) error
	DisableSafeMode(context.Context) error
	IsSafeModeEnabled() bool
	SaveTableCells(ctx context.Context) error
	AreTableCellsSaved() bool
	RemoveTableCells(context.Context) error
	RecoverTableCells(context.Context) error
	AreTabletCellsRemoved(context.Context) (bool, error)
	AreTabletCellsRecovered(context.Context) (bool, error)
	StartBuildMasterSnapshots(ctx context.Context) error
	IsMasterReadOnly(context.Context) (bool, error)
}

var (
	EnableSafeModeCondition = metav1.Condition{
		Type:    consts.ConditionSafeModeEnabled,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Safe mode was enabled",
	}
)

func (d *Discovery) Status2(ctx context.Context) (ComponentStatus, error) {
	// exists but images or config is not up-to-date
	if d.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), nil
	}

	// configMap not exists
	// OR config is not up-to-date
	// OR server not exists
	// OR not enough pods in sts
	if d.server.needSync2() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), nil
	}

	// FIXME: possible leaking abstraction, we should only check if server ready or nor
	if !d.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusUpdating, "pods"), nil
	}

	return SimpleStatus(SyncStatusReady), nil
}
func (d *Discovery) Sync2(ctx context.Context) error {
	// TODO: we are dropping this remove pods thing, but should check if it works ok without it.
	// should we detect `pods are not ready` status and don't do sync
	// will it make things easier or more observable or faster?
	return d.server.Sync(ctx)
}

func (m *Master) Status2(ctx context.Context) (ComponentStatus, error) {
	if m.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate), nil
	}
	if m.server.needSync2() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), nil
	}
	if !m.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusUpdating, "pods"), nil
	}
	// TODO: refactor access to inner initjob
	if !resources.Exists(m.initJob.initJob) {
		return SimpleStatus(SyncStatusNeedLocalUpdate), nil
	}
	if !m.initJob.initJob.Completed() {
		return WaitingStatus(SyncStatusUpdating, "init-job"), nil
	}
	return SimpleStatus(SyncStatusReady), nil
}
func (m *Master) Sync2(ctx context.Context) error {
	err := m.doServerSync(ctx)
	if err != nil {
		return err
	}
	m.initJob.SetInitScript(m.createInitScript())
	_, err = m.initJob.Sync(ctx, false)
	return err
}

func (hp *HTTPProxy) Status2(ctx context.Context) (ComponentStatus, error) {
	if hp.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), nil
	}

	// FIXME: stopped checking master running status:
	// we either run hps after master in flow or hps don't need working master to be
	// deployed correctly (though they might not be considered healthy,
	// I suppose such check could another step or some other component dependency)

	if hp.server.needSync2() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), nil
	}

	if !resources.Exists(hp.balancingService) {
		return WaitingStatus(SyncStatusNeedLocalUpdate, hp.balancingService.Name()), nil
	}

	if !hp.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusUpdating, "pods"), nil
	}

	return SimpleStatus(SyncStatusReady), nil
}
func (hp *HTTPProxy) Sync2(ctx context.Context) error {
	statefulSet := hp.server.buildStatefulSet()
	if hp.httpsSecret != nil {
		hp.httpsSecret.AddVolume(&statefulSet.Spec.Template.Spec)
		hp.httpsSecret.AddVolumeMount(&statefulSet.Spec.Template.Spec.Containers[0])
	}
	err := hp.server.Sync(ctx)
	if err != nil {
		return err
	}

	if !resources.Exists(hp.balancingService) {
		s := hp.balancingService.Build()
		s.Spec.Type = hp.serviceType
		err = hp.balancingService.Sync(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *DataNode) Status2(ctx context.Context) (ComponentStatus, error) {
	if n.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate), nil
	}
	// TODO: we don't checking master running status anymore
	if n.server.needSync2() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), nil
	}
	if !n.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusUpdating, "pods"), nil
	}
	return SimpleStatus(SyncStatusReady), nil
}
func (n *DataNode) Sync2(ctx context.Context) error {
	return n.server.Sync(ctx)
}

func (yc *ytsaurusClient) Status2(_ context.Context) (ComponentStatus, error) {
	// FIXME: stopped checking hp running status:
	// though but maybe we still should?
	// It depends on if hps could be deployed before master or not
	// maybe we need some action-step for healthcheck before ytsaurus client

	// FIXME: Why this not in New?
	var err error
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
			return ComponentStatus{}, err
		}
	}

	if yc.secret.NeedSync(consts.TokenSecretKey, "") {
		return WaitingStatus(SyncStatusNeedLocalUpdate, yc.secret.Name()), nil
	}

	// TODO: understand how check for initUserJob runs
	// we need to differentiate when it needed to be run and when it already ran

	return SimpleStatus(SyncStatusReady), nil
}
func (yc *ytsaurusClient) Sync2(ctx context.Context) error {
	var err error
	if yc.secret.NeedSync(consts.TokenSecretKey, "") {
		s := yc.secret.Build()
		s.StringData = map[string]string{
			consts.TokenSecretKey: ytconfig.RandString(30),
		}
		err = yc.secret.Sync(ctx)
		if err != nil {
			return err
		}
	}

	// todo run init job if necessary

	return nil
}
func (yc *ytsaurusClient) HandlePossibilityCheck(ctx context.Context) (ok bool, msg string, err error) {
	if !yc.ytsaurus.GetResource().Spec.EnableFullUpdate {
		msg = "Full update is not enabled"
		yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    consts.ConditionNoPossibility,
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: msg,
		})
		return false, msg, nil
	}

	// Check tablet cell bundles.
	notGoodBundles, err := GetNotGoodTabletCellBundles(ctx, yc.ytClient)

	if err != nil {
		return
	}

	if len(notGoodBundles) > 0 {
		msg = fmt.Sprintf("Tablet cell bundles (%v) aren't in 'good' health", notGoodBundles)
		yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    consts.ConditionNoPossibility,
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: msg,
		})
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
		yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    consts.ConditionNoPossibility,
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: msg,
		})
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
		yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    consts.ConditionNoPossibility,
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: msg,
		})
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
			yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionNoPossibility,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: msg,
			})
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
		msg = fmt.Sprintf("There is no leader or some peer is not active")
		yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    consts.ConditionNoPossibility,
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: msg,
		})
		return false, msg, nil
	}

	msg = "Update is possible"
	yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionHasPossibility,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: msg,
	})
	return true, "", nil
}
func (yc *ytsaurusClient) EnableSafeMode(ctx context.Context) error {
	return yc.ytClient.SetNode(ctx, ypath.Path("//sys/@enable_safe_mode"), true, nil)
	//if err != nil {
	//	return err
	//}
	//yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
	//	Type:    consts.ConditionSafeModeEnabled,
	//	Status:  metav1.ConditionTrue,
	//	Reason:  "Update",
	//	Message: "Safe mode was enabled",
	//})
	//return nil
}
func (yc *ytsaurusClient) DisableSafeMode(ctx context.Context) error {
	return yc.ytClient.SetNode(ctx, ypath.Path("//sys/@enable_safe_mode"), false, nil)
}
func (yc *ytsaurusClient) IsSafeModeEnabled() bool {
	return yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionSafeModeEnabled)
	//enableSafeModePath := "//sys/@enable_safe_mode"
	//exists, err := yc.ytClient.NodeExists(ctx, ypath.Path(enableSafeModePath), nil)
	//if err != nil {
	//	return false, err
	//}
	//if !exists {
	//	return false, nil
	//}
	//
	//var isEnabled bool
	//err = yc.ytClient.GetNode(ctx, ypath.Path(enableSafeModePath), &isEnabled, nil)
	//return isEnabled, err
}
func (yc *ytsaurusClient) saveTableCells(ctx context.Context) error {
	var tabletCellBundles []ytv1.TabletCellBundleInfo
	err := yc.ytClient.ListNode(
		ctx,
		ypath.Path("//sys/tablet_cell_bundles"),
		&tabletCellBundles,
		&yt.ListNodeOptions{Attributes: []string{"tablet_cell_count"}})

	if err != nil {
		return err
	}

	yc.ytsaurus.GetResource().Status.UpdateStatus.TabletCellBundles = tabletCellBundles

	yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionTabletCellsSaved,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Tablet cells were saved",
	})
	return nil
}
func (yc *ytsaurusClient) SaveTableCells(ctx context.Context) error {
	return yc.saveTableCells(ctx)
}
func (yc *ytsaurusClient) AreTableCellsSaved() bool {
	return yc.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTabletCellsSaved)
	// FIXME: is it a good check? What if we have 0 table cells for example?
	//return len(yc.ytsaurus.GetResource().Status.UpdateStatus.TabletCellBundles) != 0
}
func (yc *ytsaurusClient) RemoveTableCells(ctx context.Context) error {
	// FIXME: this needs locking or it can't be two reconciler loops in the same time?
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

	yc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    consts.ConditionTabletCellsRemovingStarted,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Tablet cells removing was started",
	})
	return nil
}
func (yc *ytsaurusClient) AreTabletCellsRemoved(ctx context.Context) (bool, error) {
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
func (yc *ytsaurusClient) AreTabletCellsRecovered(ctx context.Context) (bool, error) {
	// TODO: what if we have no table cells
	if len(yc.ytsaurus.GetResource().Status.UpdateStatus.TabletCellBundles) != 0 {
		return false, nil
	}

	var tabletCells []string
	err := yc.ytClient.ListNode(
		ctx,
		ypath.Path("//sys/tablet_cells"),
		&tabletCells,
		nil)

	if err != nil {
		return false, err
	}

	if len(tabletCells) == 0 {
		return false, err
	}
	return true, nil
}
func (yc *ytsaurusClient) RecoverTableCells(ctx context.Context) error {
	var err error
	for _, bundle := range yc.ytsaurus.GetResource().Status.UpdateStatus.TabletCellBundles {
		err = CreateTabletCells(ctx, yc.ytClient, bundle.Name, bundle.TabletCellCount)
		if err != nil {
			return err
		}
	}
	yc.ytsaurus.GetResource().Status.UpdateStatus.TabletCellBundles = make([]ytv1.TabletCellBundleInfo, 0)
	return yc.ytsaurus.SaveUpdateStatus(ctx)
}
func (yc *ytsaurusClient) IsMasterReadOnly(ctx context.Context) (bool, error) {
	var isReadOnly bool
	// FIXME: can read only be checked that way?
	err := yc.ytClient.GetNode(ctx, ypath.Path("//sys/@hydra_read_only"), &isReadOnly, nil)
	return isReadOnly, err
}
