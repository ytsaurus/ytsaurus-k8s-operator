package ytflow

import (
	"github.com/ytsaurus/yt-k8s-operator/pkg/conditions"
)

// Condition aliased for brevity.
type Condition = conditions.Condition

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

var (
	AllComponentsBuiltCondName Condition = "AllComponentsBuilt"

	IsFullUpdateNeededCond           Condition = "IsFullUpdateNeeded"
	IsFullUpdatePossibleCond         Condition = "IsFullUpdatePossible"
	IsSafeModeEnabledCond            Condition = "IsSafeModeEnabled"
	AreTabletCellsNeedRecoverCond    Condition = "AreTabletCellsNeedRecover"
	IsMasterInReadOnlyCond           Condition = "IsMasterInReadOnly"
	IsOperationArchiveNeedUpdateCond Condition = "IsOperationArchiveNeedUpdate"
	IsQueryTrackerNeedInitCond       Condition = "IsQueryTrackerNeedInit"

	NothingToDoCondName Condition = "NothingToDo"
)

var (
	AllComponentsBuilt = isTrue(AllComponentsBuiltCondName)

	FullUpdateNeeded           = isTrue(IsFullUpdateNeededCond)
	FullUpdatePossible         = isTrue(IsFullUpdatePossibleCond)
	SafeModeEnabled            = isTrue(IsSafeModeEnabledCond)
	TabletCellsNeedRecover     = isTrue(AreTabletCellsNeedRecoverCond)
	MasterIsInReadOnly         = isTrue(IsMasterInReadOnlyCond)
	OperationArchiveNeedUpdate = isTrue(IsOperationArchiveNeedUpdateCond)
	QueryTrackerNeedsInit      = isTrue(IsQueryTrackerNeedInitCond)

	NothingToDo = isTrue(NothingToDoCondName)
)

//var initialConditions = []Condition{
//	isDone(BuildMasterSnapshotsStep),
//}
