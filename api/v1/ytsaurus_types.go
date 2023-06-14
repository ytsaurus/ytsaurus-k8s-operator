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

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EmbeddedPersistentVolumeClaim is an embedded version of k8s.io/api/core/v1.PersistentVolumeClaim.
// It contains TypeMeta and a reduced ObjectMeta.
type EmbeddedPersistentVolumeClaim struct {
	metav1.TypeMeta `json:",inline"`

	// EmbeddedMetadata contains metadata relevant to an EmbeddedResource.
	EmbeddedObjectMetadata `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec defines the desired characteristics of a volume requested by a pod author.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
	// +optional
	Spec corev1.PersistentVolumeClaimSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// EmbeddedObjectMetadata contains a subset of the fields included in k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
// Only fields which are relevant to embedded resources are included.
type EmbeddedObjectMetadata struct {
	// Name must be unique within a namespace. Is required when creating resources, although
	// some resources may allow a client to request the generation of an appropriate name
	// automatically. Name is primarily intended for creation idempotence and configuration
	// definition.
	// Cannot be updated.
	// More info: http://kubernetes.io/docs/user-guide/identifiers#names
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`

	// Map of string keys and values that can be used to organize and categorize
	// (scope and select) objects. May match selectors of replication controllers
	// and services.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,11,rep,name=labels"`

	// Annotations is an unstructured key value map stored with a resource that may be
	// set by external tools to store and retrieve arbitrary metadata. They are not
	// queryable and should be preserved when modifying objects.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,12,rep,name=annotations"`
}

// LocationType string describes types of disk locations for YT components.
// +enum
type LocationType string

const (
	LocationTypeChunkStore       LocationType = "ChunkStore"
	LocationTypeChunkCache       LocationType = "ChunkCache"
	LocationTypeSlots            LocationType = "Slots"
	LocationTypeLogs             LocationType = "Logs"
	LocationTypeMasterChangelogs LocationType = "MasterChangelogs"
	LocationTypeMasterSnapshots  LocationType = "MasterSnapshots"
)

type LocationSpec struct {
	LocationType LocationType `json:"locationType,omitempty"`
	Path         string       `json:"path,omitempty"`

	//+kubebuilder:default:=default
	Medium string `json:"medium,omitempty"`
}

// LogLevel string describes possible YTsaurus logging level.
// +enum
type LogLevel string

const (
	LogLevelTrace LogLevel = "trace"
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelError LogLevel = "error"
)

// LogWriterType string describes types of possible log writers.
// +enum
type LogWriterType string

const (
	LogWriterTypeFile   LogWriterType = "file"
	LogWriterTypeStderr LogWriterType = "stderr"
)

type LoggerSpec struct {
	Name string `json:"name,omitempty"`
	//+kubebuilder:validation:Enum=file;stderr
	WriterType LogWriterType `json:"writerType,omitempty"`
	//+kubebuilder:validation:Enum=trace;debug;info;error
	MinLogLevel LogLevel `json:"minLogLevel,omitempty"`
}

type InstanceSpec struct {
	Volumes              []corev1.Volume                 `json:"volumes,omitempty"`
	VolumeMounts         []corev1.VolumeMount            `json:"volumeMounts,omitempty"`
	Resources            corev1.ResourceRequirements     `json:"resources,omitempty"`
	InstanceCount        int32                           `json:"instanceCount,omitempty"`
	Locations            []LocationSpec                  `json:"locations,omitempty"`
	VolumeClaimTemplates []EmbeddedPersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	// Enables pod anti affinity
	EnableAntiAffinity *bool        `json:"enableAntiAffinity,omitempty"`
	Loggers            []LoggerSpec `json:"loggers,omitempty"`
}

type MastersSpec struct {
	InstanceSpec `json:",inline"`
	CellTag      int16 `json:"cellTag"`
}

type HTTPProxiesSpec struct {
	InstanceSpec `json:",inline"`
	//+kubebuilder:default:=NodePort
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
	//+kubebuilder:default:=default
	//+kubebuilder:validation:MinLength:=1
	Role string `json:"role,omitempty"`
}

type RPCProxiesSpec struct {
	InstanceSpec `json:",inline"`
	ServiceType  *corev1.ServiceType `json:"serviceType,omitempty"`
	//+kubebuilder:default:=default
	//+kubebuilder:validation:MinLength:=1
	Role string `json:"role,omitempty"`
}

type DataNodesSpec struct {
	// label filter (for daemonset)
	// use host network
	InstanceSpec `json:",inline"`
}

type ExecNodesSpec struct {
	// label filter (for daemonset)
	InstanceSpec `json:",inline"`
}

type TabletNodesSpec struct {
	// label filter (for daemonset)
	InstanceSpec `json:",inline"`
}

type SchedulersSpec struct {
	// label filter (for daemonset)
	InstanceSpec `json:",inline"`
}

type ControllerAgentsSpec struct {
	// label filter (for daemonset)
	InstanceSpec `json:",inline"`
}

type DiscoverySpec struct {
	// label filter (for daemonset)
	InstanceSpec `json:",inline"`
}

type UISpec struct {
	//+kubebuilder:default:=NodePort
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
	//+kubebuilder:default:=false
	UseMetrikaCounter bool `json:"useMetrikaCounter,omitempty"`
	//+kubebuilder:default:=true
	UseInsecureCookies bool                        `json:"useInsecureCookies,omitempty"`
	Resources          corev1.ResourceRequirements `json:"resources,omitempty"`
	InstanceCount      int32                       `json:"instanceCount,omitempty"`
}

type QueryTrackerSpec struct {
	InstanceSpec `json:",inline"`
}

type ChytSpec struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type SpytSpec struct {
	SparkVersion string `json:"sparkVersion,omitempty"`
	SpytVersion  string `json:"spytVersion,omitempty"`
}

type YQLAgentSpec struct {
	InstanceSpec `json:",inline"`
}

// YtsaurusSpec defines the desired state of Ytsaurus
type YtsaurusSpec struct {
	CoreImage        string                        `json:"coreImage,omitempty"`
	UIImage          string                        `json:"uiImage,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`

	AdminCredentials *corev1.LocalObjectReference `json:"adminCredentials,omitempty"`
	ConfigOverrides  *corev1.LocalObjectReference `json:"configOverrides,omitempty"`

	//+kubebuilder:default:=true
	IsManaged bool `json:"isManaged,omitempty"`

	//+kubebuilder:default:=false
	UseIPv6 bool `json:"useIpv6,omitempty"`
	//+kubebuilder:default:=true
	UseShortNames bool `json:"useShortNames,omitempty"`
	//+kubebuilder:default:=false
	UsePorto bool `json:"usePorto,omitempty"`

	ExtraPodAnnotations map[string]string `json:"extraPodAnnotations,omitempty"`

	Discovery        DiscoverySpec `json:"discovery,omitempty"`
	PrimaryMasters   MastersSpec   `json:"primaryMasters,omitempty"`
	SecondaryMasters []MastersSpec `json:"secondaryMasters,omitempty"`
	// +kubebuilder:validation:MinItems:=1
	HTTPProxies []HTTPProxiesSpec `json:"httpProxies,omitempty"`
	RPCProxies  []RPCProxiesSpec  `json:"rpcProxies,omitempty"`
	// +kubebuilder:validation:MinItems:=1
	DataNodes        []DataNodesSpec       `json:"dataNodes,omitempty"`
	ExecNodes        []ExecNodesSpec       `json:"execNodes,omitempty"`
	Schedulers       *SchedulersSpec       `json:"schedulers,omitempty"`
	ControllerAgents *ControllerAgentsSpec `json:"controllerAgents,omitempty"`
	TabletNodes      []TabletNodesSpec     `json:"tabletNodes,omitempty"`

	Chyt          *ChytSpec         `json:"chyt,omitempty"`
	QueryTrackers *QueryTrackerSpec `json:"queryTrackers,omitempty"`
	Spyt          *SpytSpec         `json:"spyt,omitempty"`
	YQLAgents     *YQLAgentSpec     `json:"yqlAgents,omitempty"`

	UI *UISpec `json:"ui,omitempty"`
}

type ClusterState string

const (
	ClusterStateCreated         ClusterState = "Created"
	ClusterStateInitializing    ClusterState = "Initializing"
	ClusterStateRunning         ClusterState = "Running"
	ClusterStateReconfiguration ClusterState = "Reconfiguration"
	ClusterStateUpdating        ClusterState = "Updating"
	ClusterStateUpdateFinishing ClusterState = "UpdateFinishing"
)

type UpdateState string

const (
	UpdateStateNone                               UpdateState = "None"
	UpdateStateWaitingForSafeModeEnabled          UpdateState = "WaitingForSafeModeEnabled"
	UpdateStateWaitingForTabletCellsSaving        UpdateState = "WaitingForTabletCellsSaving"
	UpdateStateWaitingForTabletCellsRemovingStart UpdateState = "WaitingForTabletCellsRemovingStart"
	UpdateStateWaitingForTabletCellsRemoved       UpdateState = "WaitingForTabletCellsRemoved"
	UpdateStateWaitingForSnapshots                UpdateState = "WaitingForSnapshots"
	UpdateStateWaitingForPodsRemoval              UpdateState = "WaitingForPodsRemoval"
	UpdateStateWaitingForPodsCreation             UpdateState = "WaitingForPodsCreation"
	UpdateStateWaitingForTabletCellsRecovery      UpdateState = "WaitingForTabletCellsRecovery"
	UpdateStateWaitingForSafeModeDisabled         UpdateState = "WaitingForSafeModeDisabled"
)

type TabletCellBundleInfo struct {
	Name            string `yson:",value" json:"name"`
	TabletCellCount int    `yson:"tablet_cell_count,attr" json:"tabletCellCount"`
}

type UpdateStatus struct {
	//+kubebuilder:default:=None
	State                 UpdateState            `json:"state,omitempty"`
	Conditions            []metav1.Condition     `json:"conditions,omitempty"`
	TabletCellBundles     []TabletCellBundleInfo `json:"tabletCellBundles,omitempty"`
	MasterMonitoringPaths []string               `json:"masterMonitoringPaths,omitempty"`
}

// YtsaurusStatus defines the observed state of Ytsaurus
type YtsaurusStatus struct {
	//+kubebuilder:default:=Created
	State      ClusterState       `json:"state,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	UpdateStatus UpdateStatus `json:"updateStatus,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Ytsaurus is the Schema for the ytsaurus API
type Ytsaurus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   YtsaurusSpec   `json:"spec,omitempty"`
	Status YtsaurusStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// YtsaurusList contains a list of Ytsaurus
type YtsaurusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ytsaurus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Ytsaurus{}, &YtsaurusList{})
}
