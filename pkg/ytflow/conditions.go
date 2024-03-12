package ytflow

// condition is what we put in the `Type` field of k8s Condition struct.
// For the sake of brevity we call it just condition here, but it is really id/name.
// Condition *value* can be true or false.
type condition string

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
	AllComponentsBuiltCondName condition = "AllComponentsBuilt"

	IsFullUpdateNeededCond           condition = "IsFullUpdateNeeded"
	IsFullUpdatePossibleCond         condition = "IsFullUpdatePossible"
	IsSafeModeEnabledCond            condition = "IsSafeModeEnabled"
	AreTabletCellsNeedRecoverCond    condition = "AreTabletCellsNeedRecover"
	IsMasterInReadOnlyCond           condition = "IsMasterInReadOnly"
	IsOperationArchiveNeedUpdateCond condition = "IsOperationArchiveNeedUpdate"
	IsQueryTrackerNeedInitCond       condition = "IsQueryTrackerNeedInit"

	NothingToDoCondName condition = "NothingToDo"
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

//var initialConditions = []condition{
//	isDone(BuildMasterSnapshotsStep),
//}
