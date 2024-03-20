package consts

type ComponentType string

const (
	ControllerAgentType      ComponentType = "ControllerAgent"
	DataNodeType             ComponentType = "DataNode"
	DiscoveryType            ComponentType = "Discovery"
	ExecNodeType             ComponentType = "ExecNode"
	HttpProxyType            ComponentType = "HttpProxy"
	MasterCacheType          ComponentType = "MasterCache"
	MasterType               ComponentType = "Master"
	QueryTrackerType         ComponentType = "QueryTracker"
	QueueAgentType           ComponentType = "QueueAgent"
	RpcProxyType             ComponentType = "RpcProxy"
	SchedulerType            ComponentType = "Scheduler"
	StrawberryControllerType ComponentType = "StrawberryController"
	TabletNodeType           ComponentType = "TabletNode"
	TcpProxyType             ComponentType = "TcpProxy"
	UIType                   ComponentType = "UI"
	YqlAgentType             ComponentType = "YqlAgent"
	YtsaurusClientType       ComponentType = "YtsaurusClient"
)
