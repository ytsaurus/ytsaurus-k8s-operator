package ytflow

type ComponentName string
type stepName string

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

func compNameToStepName(compName ComponentName) stepName {
	return stepName(compName)
}

const (
	CheckFullUpdatePossibilityStep stepName = "CheckFullUpdatePossibility"
	EnableSafeModeStep             stepName = "EnableSafeMode"
	BackupTabletCellsStep          stepName = "BackupTabletCells"
	BuildMasterSnapshotsStep       stepName = "BuildMasterSnapshots"
	MasterExitReadOnlyStep         stepName = "MasterExitReadOnly"
	RecoverTabletCellsStep         stepName = "RecoverTabletCells"
	UpdateOpArchiveStep            stepName = "UpdateOpArchive"
	InitQueryTrackerStep           stepName = "InitQueryTracker"
	DisableSafeModeStep            stepName = "DisableSafeMode"
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

var dependencies = map[stepName][]condition{
	YtsaurusClientStep: {},

	CheckFullUpdatePossibilityStep: {
		NeedFullUpdate,
		YtsaurusClientReady,
	},
	EnableSafeModeStep: {
		isDone(CheckFullUpdatePossibilityStep),
		YtsaurusClientReady,
	},
	BackupTabletCellsStep: {
		isDone(EnableSafeModeStep),
		YtsaurusClientReady,
	},
	BuildMasterSnapshotsStep: {
		isDone(BackupTabletCellsStep),
		YtsaurusClientReady,
	},

	DiscoveryStep: {},
	HttpProxyStep: {},
	MasterStep: {
		isDone(BuildMasterSnapshotsStep),
	},

	MasterExitReadOnlyStep: {
		MasterBuilt,
		isDone(BuildMasterSnapshotsStep),
	},
	RecoverTabletCellsStep: {
		isDone(MasterExitReadOnlyStep),
	},
	UpdateOpArchiveStep: {
		isDone(RecoverTabletCellsStep),
		SchedulerBuilt,
	},
	InitQueryTrackerStep: {
		isDone(UpdateOpArchiveStep),
	},
	DisableSafeModeStep: {
		isDone(EnableSafeModeStep),
		isDone(InitQueryTrackerStep),
	},

	// finalize somehow?
}
