package controllers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

var (
	SafeModeEnabledCondition = metav1.Condition{
		Type:    consts.ConditionSafeModeEnabled,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Safe mode was enabled",
	}
	TabletCellsSavedCondition = metav1.Condition{
		Type:    consts.ConditionTabletCellsSaved,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Tablet cells were saved",
	}
	TabletCellsRemovedCondition = metav1.Condition{
		Type:    consts.ConditionTabletCellsRemoved,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Tablet cells were removed",
	}
	SnapshotsMonitoringInfoSavedCondition = metav1.Condition{
		Type:    consts.ConditionSnapshotsMonitoringInfoSaved,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Snapshots monitoring info saved",
	}
	SnapshotsBuildingStartedCondition = metav1.Condition{
		Type:    consts.ConditionSnapshotsBuildingStarted,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Snapshots building started",
	}
	MasterSnapshotsBuiltCondition = metav1.Condition{
		Type:    consts.ConditionSnaphotsSaved,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Master snapshots were built",
	}

	MasterExitedReadOnlyCondition = metav1.Condition{
		Type:    consts.ConditionMasterExitedReadOnly,
		Status:  metav1.ConditionTrue,
		Reason:  "MasterExitedReadOnly",
		Message: "Masters exited read-only state",
	}

	TabletCellsRecoveredCondition = metav1.Condition{
		Type:    consts.ConditionTabletCellsRecovered,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Tablet cells recovered",
	}
	SafeModeDisabledCondition = metav1.Condition{
		Type:    consts.ConditionSafeModeDisabled,
		Status:  metav1.ConditionTrue,
		Reason:  "Update",
		Message: "Safe mode disabled",
	}

	// TODO: PodsRemovingCondition?
	// TODO: qt/qa/scheduler conditions
)
