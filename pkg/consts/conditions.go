package consts

const ConditionHasPossibility = "HasPossibility"
const ConditionNoPossibility = "NoPossibility"
const ConditionSafeModeEnabled = "SafeModeEnabled"
const ConditionTabletCellsSaved = "TabletCellsSaved"
const ConditionTabletCellsRemovingStarted = "TabletCellsRemovingStarted"
const ConditionTabletCellsRemoved = "TabletCellsRemoved"
const ConditionSnapshotsBuildingStarted = "SnapshotsBuildingStarted"
const ConditionSnapshotsMonitoringInfoSaved = "SnapshotsMonitoringInfoSaved"
const ConditionSnaphotsSaved = "SnaphotsSaved"
const ConditionTabletCellsRecovered = "TabletCellsRecovered"
const ConditionOpArchiveUpdated = "OpArchiveUpdated"
const ConditionOpArchivePreparedForUpdating = "OpArchivePreparedForUpdating"
const ConditionSidecarsInitialized = "SidecarsInitialized"
const ConditionSidecarsPreparedForInitializing = "SidecarsPreparedForInitializing"
const ConditionQTStateUpdated = "QTStateUpdated"
const ConditionQTStatePreparedForUpdating = "QTStatePreparedForUpdating"
const ConditionQAStateUpdated = "QAStateUpdated"
const ConditionQAStatePreparedForUpdating = "QAStatePreparedForUpdating"
const ConditionYqlaUpdated = "YqlaUpdated"
const ConditionYqlaPreparedForUpdating = "YqlaPreparedForUpdating"
const ConditionMasterExitReadOnlyPrepared = "MasterExitReadOnlyPrepared"
const ConditionMasterExitedReadOnly = "MasterExitedReadOnly"
const ConditionSafeModeDisabled = "SafeModeDisabled"
const ConditionTimbertruckPrepared = "TimbertruckPrepared"
const ConditionTimbertruckUserInitialized = "TimbertruckUserInitialized"

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
