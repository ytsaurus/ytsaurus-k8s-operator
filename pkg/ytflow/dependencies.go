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
var conditionDependencies = map[conditionName][]conditionDependency{
	NothingToDoCondName: {
		AllComponentsBuilt,
		// +safe mode disabled
	},

	//	YtsaurusClientReady: {
	//		YtsaurusClientBuilt,
	//		HttpProxyBuilt,
	//		MasterBuilt,
	//	},
	MasterReadyCondName: {
		MasterBuilt, // is it enough?
	},
	HttpProxyReadyCondName: {
		HttpProxyBuilt,
		MasterReady,
	},
	DataNodeReadyCondName: {
		DataNodeBuilt,
		MasterReady,
	},
	//
	//	MasterInReadOnly: {
	//		isDone(BuildMasterSnapshotsStep),
	//	},
}

var stepDependencies = map[StepName][]conditionDependency{
	YtsaurusClientStep: {
		not(YtsaurusClientBuilt),
	},

	//CheckFullUpdatePossibilityStep: {
	//	FullUpdateNeeded,
	//	YtsaurusClientReady,
	//},
	//EnableSafeModeStep: {
	//	isDone(CheckFullUpdatePossibilityStep),
	//	YtsaurusClientReady,
	//	// safe mode disabled?
	//},
	//BackupTabletCellsStep: {
	//	isDone(EnableSafeModeStep),
	//	YtsaurusClientReady,
	//},
	//BuildMasterSnapshotsStep: {
	//	isDone(BackupTabletCellsStep),
	//	YtsaurusClientReady,
	//},

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
		//MasterIsInReadOnly,
	},

	//MasterExitReadOnlyStep: {
	//	MasterBuilt,
	//	MasterIsInReadOnly,
	//},
	//RecoverTabletCellsStep: {
	//	isDone(BackupTabletCellsStep), // TabletCellsSaved — disable after recover
	//	isDone(MasterExitReadOnlyStep),
	//},
	//UpdateOpArchiveStep: {
	//	isDone(RecoverTabletCellsStep),
	//	SchedulerBuilt,
	//},
	//InitQueryTrackerStep: {
	//	isDone(UpdateOpArchiveStep),
	//	QueryTrackerBuilt,
	//},
	//DisableSafeModeStep: {
	//	isDone(InitQueryTrackerStep),
	//	SafeModeEnabled,
	//},

	// finalize somehow?
}
