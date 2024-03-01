package components

import (
	"context"
	"fmt"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

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
func (yc *ytsaurusClient) SaveTableCells(ctx context.Context) error {
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
	if err = yc.ytsaurus.APIProxy().UpdateStatus(ctx); err != nil {
		return fmt.Errorf("failed to update status after SaveMasterMonitoringPaths: %w", err)
	}
	return nil
}
func (yc *ytsaurusClient) RemoveTableCells(ctx context.Context) error {
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
	if err = yc.ytsaurus.APIProxy().UpdateStatus(ctx); err != nil {
		return fmt.Errorf("failed to update status after SaveMasterMonitoringPaths: %w", err)
	}
	return nil
}
func (yc *ytsaurusClient) SaveMasterMonitoringPaths(ctx context.Context) error {
	var monitoringPaths []string
	mastersInfo, err := yc.getAllMasters(ctx)
	if err != nil {
		return err
	}
	for _, masterInfo := range mastersInfo {
		for _, address := range masterInfo.Addresses {
			monitoringPath := fmt.Sprintf("//sys/cluster_masters/%s/orchid/monitoring/hydra", address)
			monitoringPaths = append(monitoringPaths, monitoringPath)
		}
	}
	yc.ytsaurus.GetResource().Status.UpdateStatus.MasterMonitoringPaths = monitoringPaths
	if err = yc.ytsaurus.APIProxy().UpdateStatus(ctx); err != nil {
		return fmt.Errorf("failed to update status after SaveMasterMonitoringPaths: %w", err)
	}
	return nil
}
func (yc *ytsaurusClient) StartBuildingMasterSnapshots(ctx context.Context) error {
	return yc.startBuildMasterSnapshots(ctx)
}
func (yc *ytsaurusClient) AreMasterSnapshotsBuilt(ctx context.Context) (bool, error) {
	for _, monitoringPath := range yc.ytsaurus.GetResource().Status.UpdateStatus.MasterMonitoringPaths {
		var masterHydra MasterHydra
		if err := yc.ytClient.GetNode(ctx, ypath.Path(monitoringPath), &masterHydra, getReadOnlyGetOptions()); err != nil {
			return false, err
		}

		if !masterHydra.LastSnapshotReadOnly {
			return false, nil
		}
	}
	return true, nil
}
func (yc *ytsaurusClient) ClearUpdateStatus(ctx context.Context) error {
	return yc.ytsaurus.ClearUpdateStatus(ctx)
}
