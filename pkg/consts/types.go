package consts

type ComponentType string

const (
	BundleControllerType     ComponentType = "BundleController"
	TabletBalancerType       ComponentType = "TabletBalancer"
	ControllerAgentType      ComponentType = "ControllerAgent"
	CypressProxyType         ComponentType = "CypressProxy"
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
	OffshoreDataGatewayType  ComponentType = "OffshoreDataGateway"
	TcpProxyType             ComponentType = "TcpProxy"
	KafkaProxyType           ComponentType = "KafkaProxy"
	UIType                   ComponentType = "UI"
	YqlAgentType             ComponentType = "YqlAgent"
	YtsaurusClientType       ComponentType = "YtsaurusClient"
	ChytType                 ComponentType = "CHYT"
	SpytType                 ComponentType = "SPYT"
	ClusterConnectionType    ComponentType = "ClusterConnection"
	NativeClientConfigType   ComponentType = "NativeClientConfig"
	TimbertruckType          ComponentType = "Timbertruck"
)

type ComponentClass string

const (
	// ComponentClassStateless group contains only stateless components (not master, data nodes, tablet nodes)
	ComponentClassUnspecified    ComponentClass = ""
	ComponentClassStateless      ComponentClass = "Stateless"
	ComponentClassStatefulAgents ComponentClass = "StatefulAgents"
	ComponentClassEverything     ComponentClass = "Everything"
	ComponentClassNothing        ComponentClass = "Nothing"
)

var (
	LocalComponentTypes = []ComponentType{
		BundleControllerType,
		TabletBalancerType,
		ControllerAgentType,
		DataNodeType,
		DiscoveryType,
		ExecNodeType,
		HttpProxyType,
		MasterCacheType,
		MasterType,
		QueryTrackerType,
		QueueAgentType,
		RpcProxyType,
		SchedulerType,
		StrawberryControllerType,
		TabletNodeType,
		TcpProxyType,
		KafkaProxyType,
		YqlAgentType,
		ClusterConnectionType,
		NativeClientConfigType,
	}

	RemoteComponentTypes = []ComponentType{
		DataNodeType,
		ExecNodeType,
		TabletNodeType,
		OffshoreDataGatewayType,
	}
)
