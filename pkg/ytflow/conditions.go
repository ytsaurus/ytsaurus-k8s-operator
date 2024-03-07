package ytflow

import (
	"fmt"
)

type condition string

const (
	NeedFullUpdate condition = "NeedFullUpdate"
)

var (
	YtsaurusClientHealthy = isHealthy(YtsaurusClientStep)
	HttpProxyHealthy      = isHealthy(HttpProxyStep)
	MasterHealthy         = isHealthy(MasterStep)
	DataNodeHealthy       = isHealthy(DataNodeStep)
	SchedulerHealthy      = isHealthy(SchedulerStep)

	YtsaurusClientBuilt = isBuilt(YtsaurusClientStep)
	HttpProxyBuilt      = isBuilt(HttpProxyStep)
	MasterBuilt         = isBuilt(MasterStep)
	DataNodeBuilt       = isBuilt(DataNodeStep)
	SchedulerBuilt      = isBuilt(SchedulerStep)
	QueryTrackerBuilt   = isBuilt(QueryTrackerStep)
)

var initialConditions = []condition{
	isDone(BuildMasterSnapshotsStep),
}

var setTrueIfAllTrue = map[condition][]condition{
	YtsaurusClientHealthy: {
		YtsaurusClientBuilt,
		HttpProxyBuilt,
		MasterBuilt,
	},
	MasterHealthy: {
		MasterBuilt, // is it true?
	},
	HttpProxyHealthy: {
		HttpProxyBuilt,
		MasterHealthy,
	},
	DataNodeHealthy: {
		DataNodeBuilt,
		MasterHealthy,
	},
}

func isBuilt(name stepName) condition {
	return condition(fmt.Sprintf("%sBuilt", name))
}

func isHealthy(name stepName) condition {
	return condition(fmt.Sprintf("%sHealthy", name))
}

func isDone(name stepName) condition {
	return condition(fmt.Sprintf("%sDone", name))
}
