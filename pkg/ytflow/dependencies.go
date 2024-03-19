package ytflow

type ComponentName string
type StepName string

const (
	YtsaurusClientName ComponentName = "YtsaurusClient"
	MasterName         ComponentName = "Master"
	DiscoveryName      ComponentName = "Discovery"
	DataNodeName       ComponentName = "DataNode"
	TabletNodeName     ComponentName = "TabletNode"
	ExecNodeName       ComponentName = "ExecNode"
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
	TabletNodeStep     = compNameToStepName(TabletNodeName)
	ExecNodeStep       = compNameToStepName(ExecNodeName)
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
		not(OperationArchiveNeedUpdate),
		not(QueryTrackerNeedsInit),
	},

	isReady(YtsaurusClientName).Name: {
		isBuilt(YtsaurusClientName),
		isBuilt(HttpProxyName),
		isBuilt(MasterName),
	},
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
	},
	EnableSafeModeStep: {
		not(SafeModeEnabled),
		SafeModeCanBeEnabled,
	},
	BackupTabletCellsStep: {
		not(TabletCellsNeedRecover),
		needSync(MasterName),
		SafeModeEnabled,
	},
	BuildMasterSnapshotsStep: {
		not(MasterIsInReadOnly),
		needSync(MasterName),
		SafeModeEnabled,
		TabletCellsNeedRecover,
	},

	DiscoveryStep: {
		needSync(DiscoveryName),
		ComponentsCanBeSynced,
	},
	HttpProxyStep: {
		needSync(HttpProxyName),
		ComponentsCanBeSynced,
	},
	DataNodeStep: {
		needSync(DataNodeName),
		ComponentsCanBeSynced,
	},
	TabletNodeStep: {
		needSync(TabletNodeName),
		isBuilt(YtsaurusClientName),
		ComponentsCanBeSynced,
	},
	ExecNodeStep: {
		needSync(ExecNodeName),
		isBuilt(MasterName),
		ComponentsCanBeSynced,
	},
	MasterStep: {
		needSync(MasterName),
		MasterCanBeSynced,
	},
	SchedulerStep: {
		needSync(SchedulerName),
		ComponentsCanBeSynced,
	},
	QueryTrackerStep: {
		needSync(QueryTrackerName),
		ComponentsCanBeSynced,
	},

	MasterExitReadOnlyStep: {
		MasterIsInReadOnly,
		SafeModeEnabled,
		// Currently it works as before, but maybe we just need masterComponent to be built?
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
		not(MasterIsInReadOnly), // we need to write here
		TabletCellsReady,
	},
	InitQueryTrackerStep: {
		QueryTrackerNeedsInit,
		not(MasterIsInReadOnly), // we need to write in this step
		TabletCellsReady,
		//not(OperationArchiveNeedUpdate),
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
