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
	KafkaProxyType           ComponentType = "KafkaProxy"
	UIType                   ComponentType = "UI"
	YqlAgentType             ComponentType = "YqlAgent"
	YtsaurusClientType       ComponentType = "YtsaurusClient"
	ChytType                 ComponentType = "CHYT"
	SpytType                 ComponentType = "SPYT"
)

type ComponentClass string

const (
	// ComponentClassStateless group contains only stateless components (not master, data nodes, tablet nodes)
	ComponentClassUnspecified ComponentClass = ""
	ComponentClassStateless   ComponentClass = "Stateless"
	ComponentClassEverything  ComponentClass = "Everything"
	ComponentClassNothing     ComponentClass = "Nothing"
)
