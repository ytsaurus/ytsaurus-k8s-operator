package v1

type ComponentType string

// NOTE: In alphabetical order.
const (
	BundleControllerType     ComponentType = "BundleController"
	ControllerAgentType      ComponentType = "ControllerAgent"
	ChytType                 ComponentType = "CHYT"
	ClusterConnectionType    ComponentType = "ClusterConnection"
	CypressProxyType         ComponentType = "CypressProxy"
	DataNodeType             ComponentType = "DataNode"
	DiscoveryType            ComponentType = "Discovery"
	ExecNodeType             ComponentType = "ExecNode"
	HttpProxyType            ComponentType = "HttpProxy"
	KafkaProxyType           ComponentType = "KafkaProxy"
	MasterCacheType          ComponentType = "MasterCache"
	MasterType               ComponentType = "Master"
	NativeClientConfigType   ComponentType = "NativeClientConfig"
	OffshoreDataGatewayType  ComponentType = "OffshoreDataGateway"
	QueryTrackerType         ComponentType = "QueryTracker"
	QueueAgentType           ComponentType = "QueueAgent"
	RpcProxyType             ComponentType = "RpcProxy"
	SchedulerType            ComponentType = "Scheduler"
	SpytType                 ComponentType = "SPYT"
	StrawberryControllerType ComponentType = "StrawberryController"
	TabletBalancerType       ComponentType = "TabletBalancer"
	TabletNodeType           ComponentType = "TabletNode"
	TcpProxyType             ComponentType = "TcpProxy"
	TimbertruckType          ComponentType = "Timbertruck"
	UIType                   ComponentType = "UI"
	YqlAgentType             ComponentType = "YqlAgent"
	YtsaurusClientType       ComponentType = "YtsaurusClient"
)

type ComponentClass string

const (
	ComponentClassUnspecified ComponentClass = ""
	// ComponentClassNothing includes no components.
	ComponentClassNothing ComponentClass = "Nothing"
	// ComponentClassEverything includes all components.
	ComponentClassEverything ComponentClass = "Everything"
	// ComponentClassStateless includes all except master, data and tablet nodes.
	ComponentClassStateless ComponentClass = "Stateless"
)
