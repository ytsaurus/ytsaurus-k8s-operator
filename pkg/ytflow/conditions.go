package ytflow

import (
	"github.com/ytsaurus/yt-k8s-operator/pkg/state"
)

// Condition aliased for brevity.
type Condition = state.Condition

// *Built conditions are kept in sync by flow automatically.
// Developers shouldn't set them in code manually.
var (
	YtsaurusClientBuiltCondName = isBuiltCondName(YtsaurusClientName)
	DiscoveryBuiltCondName      = isBuiltCondName(DiscoveryName)
	HttpProxyBuiltCondName      = isBuiltCondName(HttpProxyName)
	MasterBuiltCondName         = isBuiltCondName(MasterName)
	DataNodeBuiltCondName       = isBuiltCondName(DataNodeName)
	SchedulerBuiltCondName      = isBuiltCondName(SchedulerName)
	QueryTrackerBuiltCondName   = isBuiltCondName(QueryTrackerName)
)

var (
	YtsaurusClientBuilt = isBuilt(YtsaurusClientName)
	DiscoveryBuilt      = isBuilt(DiscoveryName)
	HttpProxyBuilt      = isBuilt(HttpProxyName)
	MasterBuilt         = isBuilt(MasterName)
	DataNodeBuilt       = isBuilt(DataNodeName)
	SchedulerBuilt      = isBuilt(SchedulerName)
	QueryTrackerBuilt   = isBuilt(QueryTrackerName)
)

var (
	YtsaurusClientReadyCondName = isReadyCondName(YtsaurusClientName)
	DiscoveryReadyCondName      = isReadyCondName(DiscoveryName)
	HttpProxyReadyCondName      = isReadyCondName(HttpProxyName)
	MasterReadyCondName         = isReadyCondName(MasterName)
	DataNodeReadyCondName       = isReadyCondName(DataNodeName)
	SchedulerReadyCondName      = isReadyCondName(SchedulerName)
)

var (
	YtsaurusClientReady = isReady(YtsaurusClientName)
	DiscoveryReady      = isReady(DiscoveryName)
	HttpProxyReady      = isReady(HttpProxyName)
	MasterReady         = isReady(MasterName)
	DataNodeReady       = isReady(DataNodeName)
	SchedulerReady      = isReady(SchedulerName)
)

// Special conditions.

// TODO: it is not very good to have condition and condition dependency at the same time
// as it makes harder to search for usages.
var (
	AllComponentsBuiltCondName Condition = "AllComponentsBuilt"

	IsFullUpdateNeededCond           Condition = "IsFullUpdateNeeded"
	IsFullUpdatePossibleCond         Condition = "IsFullUpdatePossible"
	IsSafeModeEnabledCond            Condition = "IsSafeModeEnabled"
	DoTabletCellsNeedRecoverCond     Condition = "DoTabletCellsNeedRecover"
	IsMasterInReadOnlyCond           Condition = "IsMasterInReadOnly"
	IsOperationArchiveNeedUpdateCond Condition = "IsOperationArchiveNeedUpdate"
	IsQueryTrackerNeedInitCond       Condition = "IsQueryTrackerNeedInit"

	// IsTabletCellsRemovalStartedCond is an intermediate condition of Tablet cell backup action
	IsTabletCellsRemovalStartedCond Condition = "IsTabletCellsRemovalStarted"
	// IsMasterSnapshotBuildingStartedCond is an intermediate condition of Build master snapshots action.
	IsMasterSnapshotBuildingStartedCond Condition = "IsMasterSnapshotBuildingStarted"

	NothingToDoCondName Condition = "NothingToDo"
)

var (
	AllComponentsBuilt = isTrue(AllComponentsBuiltCondName)

	FullUpdateNeeded           = isTrue(IsFullUpdateNeededCond)
	FullUpdatePossible         = isTrue(IsFullUpdatePossibleCond)
	SafeModeEnabled            = isTrue(IsSafeModeEnabledCond)
	TabletCellsNeedRecover     = isTrue(DoTabletCellsNeedRecoverCond)
	MasterIsInReadOnly         = isTrue(IsMasterInReadOnlyCond)
	OperationArchiveNeedUpdate = isTrue(IsOperationArchiveNeedUpdateCond)
	QueryTrackerNeedsInit      = isTrue(IsQueryTrackerNeedInitCond)

	NothingToDo = isTrue(NothingToDoCondName)
)

//var initialConditions = []Condition{
//	isDone(BuildMasterSnapshotsStep),
//}
