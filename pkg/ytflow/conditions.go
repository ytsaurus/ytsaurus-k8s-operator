package ytflow

// conditionName is what we put in the Type field of k8s condition
type conditionName string

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
	NeedFullUpdateCondName     conditionName = "NeedFullUpdate"
	SafeModeCondName           conditionName = "SafeModeEnabled"
	MasterReadOnlyCondName     conditionName = "MasterInReadOnly"
	NothingToDoCondName        conditionName = "NothingToDo"
	AllComponentsBuiltCondName conditionName = "AllComponentsBuilt"
)

var (
	FullUpdateNeeded   = isTrue(NeedFullUpdateCondName)
	SafeModeEnabled    = isTrue(SafeModeCondName)
	SafeModeDisabled   = isFalse(SafeModeCondName)
	MasterIsInReadOnly = isTrue(MasterReadOnlyCondName)
	AllComponentsBuilt = isTrue(AllComponentsBuiltCondName)
	NothingToDo        = isTrue(NothingToDoCondName)
)

//var initialConditions = []condition{
//	isDone(BuildMasterSnapshotsStep),
//}
