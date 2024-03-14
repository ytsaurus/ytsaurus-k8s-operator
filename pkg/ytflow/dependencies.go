package ytflow

type ComponentName string
type StepName string

const (
	YtsaurusClientName ComponentName = "YtsaurusClient"
	MasterName         ComponentName = "Master"
	DiscoveryName      ComponentName = "Discovery"
	DataNodeName       ComponentName = "DataNode"
	TabletNodeName     ComponentName = "TabletNode"
	HttpProxyName      ComponentName = "HttpProxy"
	SchedulerName      ComponentName = "Scheduler"
	QueryTrackerName   ComponentName = "QueryTracker"
)

func compNameToStepName(compName ComponentName) StepName {
	return StepName(compName)
}

const (
	CheckFullUpdatePossibilityStep StepName = "CheckFullUpdatePossibility"
	EnableSafeModeStep             StepName = "EnableSafeMode"
	BackupTabletCellsStep          StepName = "BackupTabletCells"
	BuildMasterSnapshotsStep       StepName = "BuildMasterSnapshots"
	MasterExitReadOnlyStep         StepName = "MasterExitReadOnly"
	RecoverTabletCellsStep         StepName = "RecoverTabletCells"
	UpdateOpArchiveStep            StepName = "UpdateOpArchive"
	InitQueryTrackerStep           StepName = "InitQueryTracker"
	DisableSafeModeStep            StepName = "DisableSafeMode"
)

var (
	YtsaurusClientStep = compNameToStepName(YtsaurusClientName)
	MasterStep         = compNameToStepName(MasterName)
	DiscoveryStep      = compNameToStepName(DiscoveryName)
	DataNodeStep       = compNameToStepName(DataNodeName)
	HttpProxyStep      = compNameToStepName(HttpProxyName)
	SchedulerStep      = compNameToStepName(SchedulerName)
	QueryTrackerStep   = compNameToStepName(QueryTrackerName)
)

// conditionDependencies simply is:
// condName become true if all condition deps are true
// condName become false if any of condition deps are false
var conditionDependencies = map[ConditionName][]Condition{
	NothingToDo.Name: {
		AllComponentsSynced,
		not(SafeModeEnabled),
	},

	isReady(YtsaurusClientName).Name: {
		isBuilt(YtsaurusClientName),
		isBuilt(HttpProxyName),
		isBuilt(MasterName),
	},
	//HttpProxyReady.name: {
	//	HttpProxyBuilt,
	//	MasterBuilt,
	//},
	//DataNodeReadyCondName: {
	//	DataNodeBuilt,
	//	MasterReady,
	//},
}

// stepDependencies describes what conditions must be satisfied for step to run.
// For this map to be understandable it is suggested to list dependencies in following manner:
//   - first dependency mentions main step condition, it is expected that condition will flip after the step
//     successfully run (often it is not-condition);
//   - other dependencies are secondary and usually declare an order in which steps should run.
var stepDependencies = map[StepName][]Condition{
	YtsaurusClientStep: {
		not(isBuilt(YtsaurusClientName)),
	},

	CheckFullUpdatePossibilityStep: {
		// isBuilt + needSync == cluster needs to run full update routine.
		not(SafeModeCanBeEnabled),
		isBuilt(MasterName),
		needSync(MasterName),
		not(SafeModeEnabled),
		//YtsaurusClientReady,  // TODO: fix that later
	},
	EnableSafeModeStep: {
		not(SafeModeEnabled),
		SafeModeCanBeEnabled,
		//YtsaurusClientReady,  // TODO: fix that later
	},
	BackupTabletCellsStep: {
		not(TabletCellsNeedRecover),
		needSync(MasterName),
		SafeModeEnabled,
		//YtsaurusClientReady,  // TODO: fix that later
	},
	BuildMasterSnapshotsStep: {
		not(MasterIsInReadOnly),
		needSync(MasterName),
		SafeModeEnabled,
		TabletCellsNeedRecover,
		//YtsaurusClientReady,  // TODO: fix that later
	},

	DiscoveryStep: {
		needSync(DiscoveryName),
	},
	HttpProxyStep: {
		needSync(HttpProxyName),
	},
	DataNodeStep: {
		needSync(DataNodeName),
	},
	MasterStep: {
		needSync(MasterName),
		MasterCanBeSynced,
	},

	MasterExitReadOnlyStep: {
		MasterIsInReadOnly,
		SafeModeEnabled,
		// Currently it works as before, but maybe we just need master to be built?
		AllComponentsSynced,
	},
	RecoverTabletCellsStep: {
		TabletCellsNeedRecover,
		SafeModeEnabled,
		AllComponentsSynced,
		not(MasterIsInReadOnly), // we need to write in this step
	},
	UpdateOpArchiveStep: {
		OperationArchiveNeedUpdate,
		// Do we *really* depend on tablet cells in this job,
		// or it could be done independently after exit RO?
		not(TabletCellsNeedRecover),
		not(MasterIsInReadOnly), // we need to write here
	},
	InitQueryTrackerStep: {
		QueryTrackerNeedsInit,
		// Do we *really* depend on tablet cells OR UpdateOpArchiveStep in this job,
		// or it could be done independently after exit RO?
		not(OperationArchiveNeedUpdate),
		not(MasterIsInReadOnly), // we need to write in this step
	},
	DisableSafeModeStep: {
		SafeModeEnabled,
		AllComponentsSynced,
		not(MasterIsInReadOnly), // we need to write in this step

		// All of those should be done before unlocking the cluster from read only.
		not(TabletCellsNeedRecover),
		not(OperationArchiveNeedUpdate),
		not(QueryTrackerNeedsInit),
	},
}
