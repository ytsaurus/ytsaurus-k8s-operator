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

import "fmt"

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
	GarbageCollectorType     ComponentType = "GarbageCollector"
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
	UpdatePlanType           ComponentType = "UpdatePlan"
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

type ComponentState string

const (
	// Status is unknown
	ComponentStateUndefined ComponentState = ""
	// Running, reconciliation is not required
	ComponentStateReady ComponentState = "Ready"
	// Reconciliation is complete, is not ready yet
	ComponentStateStarted ComponentState = "Started"
	// Reconciliation is impossible
	ComponentStateBlocked ComponentState = "Blocked"
	// Reconciliation is possible
	ComponentStatePending ComponentState = "Pending"
	// Running, update is required
	ComponentStateNeedUpdate ComponentState = "NeedUpdate"
	// Update in progress
	ComponentStateUpdating ComponentState = "Updating"
)

type Component struct {
	Type ComponentType `json:"type,omitempty"`
	Name string        `json:"name,omitempty"`
}

// +kubebuilder:object:generate=false
type ComponentSpec interface {
	GetComponent() Component
	GetInstanceCount() int
}

func getComponent(componentType ComponentType, name string) Component {
	if name == "default" {
		name = ""
	}
	return Component{componentType, name}
}

func (c *Component) String() string {
	if c.Name == "" {
		return string(c.Type)
	}
	return fmt.Sprintf("%s-%s", c.Type, c.Name)
}

type ComponentStatus struct {
	State   ComponentState `json:"state,omitempty"`
	Message string         `json:"message,omitempty"`
}

type YtsaurusComponentStatus struct {
	State              ComponentState `json:"state,omitempty"`
	Message            string         `json:"message,omitempty"`
	ObservedGeneration int64          `json:"observedGeneration,omitempty"`
}

// Points to Ytsaurus resource in the same namespace.
type YtsaurusReference struct {
	Name string `json:"name"`
}

type YtsaurusComponentBase struct {
	Status YtsaurusComponentStatus `json:"status,omitempty"`
}

func (b *YtsaurusComponentBase) GetStatusObservedGeneration() int64 {
	return b.Status.ObservedGeneration
}

func (b *YtsaurusComponentBase) SetStatusObservedGeneration(generation int64) {
	b.Status.ObservedGeneration = generation
}

func (b *YtsaurusComponentBase) GetComponentStatus() ComponentStatus {
	return ComponentStatus{b.Status.State, b.Status.Message}
}

func (b *YtsaurusComponentBase) SetComponentStatus(status ComponentStatus) {
	b.Status.State = status.State
	b.Status.Message = status.Message
}
