package ytflow

import (
	"fmt"
)

type condition string

// *Built conditions are kept in sync by flow automatically.
// Developers shouldn't set them in code manually.
var (
	YtsaurusClientBuilt = isBuilt(YtsaurusClientStep)
	HttpProxyBuilt      = isBuilt(HttpProxyStep)
	MasterBuilt         = isBuilt(MasterStep)
	DataNodeBuilt       = isBuilt(DataNodeStep)
	SchedulerBuilt      = isBuilt(SchedulerStep)
	QueryTrackerBuilt   = isBuilt(QueryTrackerStep)
)

var (
	YtsaurusClientReady = isReady(YtsaurusClientStep)
	HttpProxyReady      = isReady(HttpProxyStep)
	MasterReady         = isReady(MasterStep)
	DataNodeReady       = isReady(DataNodeStep)
	SchedulerReady      = isReady(SchedulerStep)
)

// Special conditions.

var (
	NeedFullUpdate condition = "NeedFullUpdate"
)

var initialConditions = []condition{
	isDone(BuildMasterSnapshotsStep),
}

var setTrueIfAllTrue = map[condition][]condition{
	YtsaurusClientReady: {
		YtsaurusClientBuilt,
		HttpProxyBuilt,
		MasterBuilt,
	},
	MasterReady: {
		MasterBuilt, // is it true?
	},
	HttpProxyReady: {
		HttpProxyBuilt,
		MasterReady,
	},
	DataNodeReady: {
		DataNodeBuilt,
		MasterReady,
	},
}

// Components' statuses.
func isBuilt(name stepName) condition {
	return condition(fmt.Sprintf("%sBuilt", name))
}

func isReady(name stepName) condition {
	return condition(fmt.Sprintf("%sHealthy", name))
}

// Actions statuses.
func isBlocked(name stepName) condition {
	return condition(fmt.Sprintf("%sBlocked", name))
}

func isRun(name stepName) condition {
	return condition(fmt.Sprintf("%sRun", name))
}

func isUpdating(name stepName) condition {
	return condition(fmt.Sprintf("%sUpdating", name))
}

func isDone(name stepName) condition {
	return condition(fmt.Sprintf("%sDone", name))
}
