/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
	ImageHeaterType          ComponentType = "ImageHeater"
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
