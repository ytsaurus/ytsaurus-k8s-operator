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

var initialDependencies = []conditionDependency{
	MasterCanBeRebuilt,
}

// conditionDependencies simply is:
// condName become true if all condition deps are true
// condName become false if any of condition deps are false
var conditionDependencies = map[Condition][]conditionDependency{
	NothingToDoCondName: {
		AllComponentsBuilt,
		not(SafeModeEnabled),
	},

	YtsaurusClientReadyCondName: {
		YtsaurusClientBuilt,
		HttpProxyBuilt,
		MasterBuilt,
	},
	MasterReadyCondName: {
		MasterBuilt, // is it enough?
	},
	HttpProxyReadyCondName: {
		HttpProxyBuilt,
		MasterReady,
	},
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
var stepDependencies = map[StepName][]conditionDependency{
	YtsaurusClientStep: {
		not(YtsaurusClientBuilt),
	},

	CheckFullUpdatePossibilityStep: {
		not(FullUpdatePossible),
		FullUpdateNeeded,
		YtsaurusClientReady,
	},
	EnableSafeModeStep: {
		not(SafeModeEnabled),
		FullUpdatePossible,
		YtsaurusClientReady,
	},
	BackupTabletCellsStep: {
		not(TabletCellsNeedRecover),
		FullUpdatePossible,
		SafeModeEnabled,
		YtsaurusClientReady,
	},
	BuildMasterSnapshotsStep: {
		not(MasterCanBeRebuilt),
		FullUpdatePossible,
		not(TabletCellsNeedRecover),
		YtsaurusClientReady,
	},

	DiscoveryStep: {
		not(DiscoveryBuilt),
	},
	HttpProxyStep: {
		not(HttpProxyBuilt),
	},
	DataNodeStep: {
		not(DataNodeBuilt),
	},
	MasterStep: {
		not(MasterBuilt),
		// TODO: set initial condition that master is in read only (which is not true)?
		// It would be better to have OR-condition (IsInReadOnly | Initializing) here maybe?
		// (SafeModeEnabled & MasterInReadOnly | )
		MasterCanBeRebuilt,
	},

	MasterExitReadOnlyStep: {
		MasterCanBeRebuilt,
		// Currently it works as before, but maybe we just need master to be built?
		AllComponentsBuilt,
		SafeModeEnabled,
	},
	RecoverTabletCellsStep: {
		TabletCellsNeedRecover,
		not(MasterCanBeRebuilt), // we need to write in this step
		SafeModeEnabled,
	},
	UpdateOpArchiveStep: {
		OperationArchiveNeedUpdate,
		not(TabletCellsNeedRecover), // do we *really* depend on tablet cells in this job?
		not(MasterCanBeRebuilt),     // we need to write here
		SafeModeEnabled,
		//SchedulerBuilt,              // do we need scheduler for that script or only master
	},
	InitQueryTrackerStep: {
		QueryTrackerNeedsInit,
		not(OperationArchiveNeedUpdate), // do we *really* depend on tablet cells in this job?
		not(MasterCanBeRebuilt),         // we need to write in this step
		SafeModeEnabled,
		//QueryTrackerBuilt,       // do we need query tracker for that script or only master
	},
	DisableSafeModeStep: {
		SafeModeEnabled,
		not(MasterCanBeRebuilt), // we need to write in this step

		// All of those should be done before unlocking the cluster from read only.
		not(TabletCellsNeedRecover),
		not(OperationArchiveNeedUpdate),
		not(QueryTrackerNeedsInit),
	},
}
