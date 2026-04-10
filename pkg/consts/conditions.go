package consts

// Status conditions
const (
	ConditionOperatorVersion              = "OperatorVersion"
	ConditionClusterMaintenance           = "ClusterMaintenance"
	ConditionMastersQuorumCheck           = "MastersQuorumCheck"
	ConditionLostVitalChunksCheck         = "LostVitalChunksCheck"
	ConditionQuorumMissingChunksCheck     = "QuorumMissingChunksCheck"
	ConditionTabletCellBundlesHealthCheck = "TabletCellBundlesHealthCheck"
	ConditionReadyToWork                  = "ReadyToWork"
	ConditionImageHeaterReady             = "ImageHeaterReady"
	ConditionImageHeaterComplete          = "ImageHeaterComplete"
	ConditionTimbertruckUserInitialized   = "TimbertruckUserInitialized"
)

// Both status and update conditions
const (
	ConditionTimbertruckPrepared     = "TimbertruckPrepared"
	ConditionCypressPatchApplied     = "CypressPatchApplied"
	ConditionMasterCellsRegistration = "MasterCellsRegistration"
	ConditionMasterCellsSettlement   = "MasterCellsSettlement"
)

// Component status conditions suffixes
const (
	ConditionReady            = "Ready"
	ConditionInitJobCompleted = "InitJobCompleted"
	ConidtionCellSettled      = "-settled"
)

// Component update phase condition suffixes
const (
	ConditionBulkUpdateModeStarted  = "BulkUpdateModeStarted"
	ConditionWaitingOnDeleteUpdate  = "WaitingOnDeleteUpdate"
	ConditionRollingBudgetExhausted = "RollingBudgetExhausted"
	ConditionPreChecksRunning       = "PreChecksRunning"
	ConditionScalingDown            = "ScalingDown"
	ConditionScalingUp              = "ScalingUp"
	ConditionPodsUpdated            = "PodsUpdated"
	ConditionPodsRemoved            = "PodsRemoved"
	ConditionPodsRemovingStarted    = "PodsRemovingStarted"

	// Suffix for update state itself
	ConditionUpdateStateComplete = "Complete"
)

// Update conditions
const (
	ConditionHasPossibility                  = "HasPossibility"
	ConditionSafeModeEnabled                 = "SafeModeEnabled"
	ConditionTabletCellsSaved                = "TabletCellsSaved"
	ConditionTabletCellsRemovingStarted      = "TabletCellsRemovingStarted"
	ConditionTabletCellsRemoved              = "TabletCellsRemoved"
	ConditionSnaphotsSaved                   = "SnaphotsSaved"
	ConditionTabletCellsRecovered            = "TabletCellsRecovered"
	ConditionSidecarsPreparedForInitializing = "SidecarsPreparedForInitializing" // TODO: Remove, it holds alignment.
	ConditionQTStateUpdated                  = "QTStateUpdated"
	ConditionQTStatePreparedForUpdating      = "QTStatePreparedForUpdating"
	ConditionQAStateUpdated                  = "QAStateUpdated"
	ConditionQAStatePreparedForUpdating      = "QAStatePreparedForUpdating"
	ConditionSafeModeDisabled                = "SafeModeDisabled"
)

// Conditions below are for migration from imaginary chunks to real chunks for 24.2
// https://github.com/ytsaurus/ytsaurus-k8s-operator/issues/396
const (
	// ConditionRealChunksAttributeEnabled is set by client component when
	// it ensures that sys/@config/node_tracker/enable_real_chunk_locations == %true.
	ConditionRealChunksAttributeEnabled = "RealChunksAttributeEnabled"

	// ConditionDataNodesNeedPodsRemoval is set by client component when it detects that
	// some nodes have imaginary chunks and need to be restarted to remove them.
	ConditionDataNodesNeedPodsRemoval = "DataNodesNeedPodsRemoval"

	// ConditionDataNodesWithImaginaryChunksAbsent is set by client component when
	// it ensures that there are no active data nodes with imaginary chunks exists, so master
	// can be safely updated to 24.2.
	ConditionDataNodesWithImaginaryChunksAbsent = "DataNodesWithImaginaryChunksAbsent"
)
