package consts

import (
	"fmt"
)

// "<ComponentType>[-<InstanceGroup>]"
// NOTE: instance group comes from spec and can be non-CamelCase.
type ComponentName string

type ComponentTypeNames struct {
	Short    string
	Singular string
	Plural   string
}

var componentTypesNames = map[ComponentType]ComponentTypeNames{
	ControllerAgentType: {
		Short:    "ca",
		Singular: "controller-agent",
		Plural:   "controller-agents",
	},
	DataNodeType: {
		Short:    "dnd",
		Singular: "data-node",
		Plural:   "data-nodes",
	},
	DiscoveryType: {
		Short:    "ds",
		Singular: "discovery",
		Plural:   "discovery",
	},
	ExecNodeType: {
		Short:    "end",
		Singular: "exec-node",
		Plural:   "exec-nodes",
	},
	HttpProxyType: {
		Short:    "hp",
		Singular: "http-proxy",
		Plural:   "http-proxies",
	},
	MasterCacheType: {
		Short:    "msc",
		Singular: "master-cache",
		Plural:   "master-caches",
	},
	MasterType: {
		Short:    "ms",
		Singular: "master",
		Plural:   "masters",
	},
	QueryTrackerType: {
		Short:    "qt",
		Singular: "query-tracker",
		Plural:   "query-trackers",
	},
	QueueAgentType: {
		Short:    "qa",
		Singular: "queue-agent",
		Plural:   "queue-agents",
	},
	RpcProxyType: {
		Short:    "rp",
		Singular: "rpc-proxy",
		Plural:   "rpc-proxies",
	},
	SchedulerType: {
		Short:    "sch",
		Singular: "scheduler",
		Plural:   "schedulers",
	},
	StrawberryControllerType: {
		Short:    "strawberry",
		Singular: "strawberry-controller",
		Plural:   "strawberry",
	},
	TabletNodeType: {
		Short:    "tnd",
		Singular: "tablet-node",
		Plural:   "tablet-nodes",
	},
	TcpProxyType: {
		Short:    "tcp",
		Singular: "tcp-proxy",
		Plural:   "tcp-proxies",
	},
	KafkaProxyType: {
		Short:    "kp",
		Singular: "kafka-proxy",
		Plural:   "kafka-proxies",
	},
	UIType: {
		Short:    "ui",
		Singular: "ui",
		Plural:   "ui",
	},
	YqlAgentType: {
		Short:    "yqla",
		Singular: "yql-agent",
		Plural:   "yql-agents",
	},
	ChytType: {
		Short:    "chyt",
		Singular: "chyt",
		Plural:   "chyts",
	},
	SpytType: {
		Short:    "spyt",
		Singular: "spyt",
		Plural:   "spyts",
	},
	ClusterConnectionType: {
		Short:    "cluster-connection",
		Singular: "cluster-connection",
		Plural:   "cluster-connections",
	},
	YtsaurusClientType: {
		Short:    "client",
		Singular: "client",
		Plural:   "clients",
	},
	NativeClientConfigType: {
		Short:    "native-client",
		Singular: "native-client",
		Plural:   "native-clients",
	},
	TimbertruckType: {
		Short:    "tt",
		Singular: "timbertruck",
		Plural:   "timbertrucks",
	},
}

func getComponentTypeNames(component ComponentType) ComponentTypeNames {
	names, found := componentTypesNames[component]
	if !found {
		panic(fmt.Sprintf("No names is defined for component type: %s", component))
	}
	return names
}

func (c ComponentType) ShortName() string {
	return getComponentTypeNames(c).Short
}

func (c ComponentType) SingularName() string {
	return getComponentTypeNames(c).Singular
}

func (c ComponentType) PluralName() string {
	return getComponentTypeNames(c).Plural
}
