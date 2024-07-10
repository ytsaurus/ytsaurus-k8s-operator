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

	"k8s.io/apimachinery/pkg/api/resource"
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
	LocationTypeImageCache       LocationType = "ImageCache"
)

type LocationSpec struct {
	LocationType LocationType `json:"locationType,omitempty"`
	//+kubebuilder:validation:MinLength:=1
	Path string `json:"path,omitempty"`

	//+kubebuilder:default:=default
	Medium string `json:"medium,omitempty"`

	// Disk space quota, default is size of related volume.
	//+optional
	Quota *resource.Quantity `json:"quota,omitempty"`
	// Limit above which the volume is considered to be non-full.
	//+optional
	LowWatermark *resource.Quantity `json:"lowWatermark,omitempty"`
	// Max TTL of trash in milliseconds.
	//+kubebuilder:validation:Minimum:=60000
	MaxTrashMilliseconds *int64 `json:"maxTrashMilliseconds,omitempty"`
}

// LogLevel string describes possible Ytsaurus logging level.
// +enum
type LogLevel string

const (
	LogLevelTrace   LogLevel = "trace"
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
)

// LogWriterType string describes types of possible log writers.
// +enum
type LogWriterType string

const (
	LogWriterTypeFile   LogWriterType = "file"
	LogWriterTypeStderr LogWriterType = "stderr"
)

type LogFormat string

const (
	LogFormatPlainText LogFormat = "plain_text"
	LogFormatYson      LogFormat = "yson"
	LogFormatJson      LogFormat = "json"
)

type LogCompression string

const (
	LogCompressionNone LogCompression = "none"
	LogCompressionGzip LogCompression = "gzip"
	LogCompressionZstd LogCompression = "zstd"
)

// CategoriesFilterType string describes types of possible log CategoriesFilter.
// +enum
type CategoriesFilterType string

const (
	CategoriesFilterTypeExclude CategoriesFilterType = "exclude"
	CategoriesFilterTypeInclude CategoriesFilterType = "include"
)

type CategoriesFilter struct {
	//+kubebuilder:validation:Enum=exclude;include
	Type CategoriesFilterType `json:"type,omitempty"`
	//+kubebuilder:validation:MinItems=1
	Values []string `json:"values,omitempty"`
}

type LogRotationPolicy struct {
	RotationPeriodMilliseconds *int64 `json:"rotationPeriodMilliseconds,omitempty" yson:"rotation_period,omitempty"`
	MaxSegmentSize             *int64 `json:"maxSegmentSize,omitempty" yson:"max_segment_size,omitempty"`
	MaxTotalSizeToKeep         *int64 `json:"maxTotalSizeToKeep,omitempty" yson:"max_total_size_to_keep,omitempty"`
	MaxSegmentCountToKeep      *int64 `json:"maxSegmentCountToKeep,omitempty" yson:"max_segment_count_to_keep,omitempty"`
}

type BaseLoggerSpec struct {
	//+kubebuilder:validation:MinLength:=1
	Name string `json:"name,omitempty"`
	//+kubebuilder:default:=plain_text
	//+kubebuilder:validation:Enum=plain_text;json;yson
	Format LogFormat `json:"format,omitempty"`
	//+kubebuilder:validation:Enum=trace;debug;info;warning;error
	//+kubebuilder:default:=info
	MinLogLevel LogLevel `json:"minLogLevel,omitempty"`
	//+kubebuilder:default:=none
	//+kubebuilder:validation:Enum=none;gzip;zstd
	Compression LogCompression `json:"compression,omitempty"`
	//+kubebuilder:default:=false
	//+optional
	UseTimestampSuffix bool               `json:"useTimestampSuffix"`
	RotationPolicy     *LogRotationPolicy `json:"rotationPolicy,omitempty"`
}

type TextLoggerSpec struct {
	BaseLoggerSpec `json:",inline"`
	//+kubebuilder:validation:Enum=file;stderr
	WriterType       LogWriterType     `json:"writerType,omitempty"`
	CategoriesFilter *CategoriesFilter `json:"categoriesFilter,omitempty"`
}

type StructuredLoggerSpec struct {
	BaseLoggerSpec `json:",inline"`
	Category       string `json:"category,omitempty"`
}

type BundleBootstrapSpec struct {
	SnapshotPrimaryMedium  *string `json:"snapshotMedium,omitempty"`
	ChangelogPrimaryMedium *string `json:"changelogMedium,omitempty"`
	//+kubebuilder:default:=1
	TabletCellCount int `json:"tabletCellCount,omitempty"`
}

type BundlesBootstrapSpec struct {
	Sys     *BundleBootstrapSpec `json:"sys,omitempty"`
	Default *BundleBootstrapSpec `json:"default,omitempty"`
}

type BootstrapSpec struct {
	TabletCellBundles *BundlesBootstrapSpec `json:"tabletCellBundles,omitempty"`
}

type OauthUserInfoHandlerSpec struct {
	//+kubebuilder:default:=user/info
	Endpoint string `json:"endpoint,omitempty"`
	//+kubebuilder:default:=nickname
	LoginField string  `json:"loginField,omitempty"`
	ErrorField *string `json:"errorField,omitempty"`
}

type OauthServiceSpec struct {
	//+kubebuilder:validation:MinLength:=1
	Host string `json:"host,omitempty"`
	//+kubebuilder:default:=80
	Port int `json:"port,omitempty"`
	//+kubebuilder:default:=false
	Secure   bool                     `json:"secure,omitempty"`
	UserInfo OauthUserInfoHandlerSpec `json:"userInfoHandler,omitempty"`
}

type HealthcheckProbeParams struct {
	//+optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty"`
	//+optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
	//+optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty"`
	//+optional
	SuccessThreshold int32 `json:"successThreshold,omitempty"`
	//+optional
	FailureThreshold int32 `json:"failureThreshold,omitempty"`
}

type InstanceSpec struct {
	// Overrides coreImage for component.
	//+optional
	Image *string `json:"image,omitempty"`
	// Specifies wrapper for component container command.
	//+optional
	EntrypointWrapper []string             `json:"entrypointWrapper,omitempty"`
	Volumes           []corev1.Volume      `json:"volumes,omitempty"`
	VolumeMounts      []corev1.VolumeMount `json:"volumeMounts,omitempty"`
	//+optional
	ReadinessProbeParams  *HealthcheckProbeParams         `json:"readinessProbeParams,omitempty"`
	Resources             corev1.ResourceRequirements     `json:"resources,omitempty"`
	InstanceCount         int32                           `json:"instanceCount,omitempty"`
	MinReadyInstanceCount *int                            `json:"minReadyInstanceCount,omitempty"`
	Locations             []LocationSpec                  `json:"locations,omitempty"`
	VolumeClaimTemplates  []EmbeddedPersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	//+optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`
	// Deprecated: use Affinity.PodAntiAffinity instead.
	EnableAntiAffinity *bool `json:"enableAntiAffinity,omitempty"`
	// Use the host's network namespace, this overrides global option.
	//+optional
	HostNetwork *bool `json:"hostNetwork,omitempty"`
	//+optional
	MonitoringPort    *int32                 `json:"monitoringPort,omitempty"`
	Loggers           []TextLoggerSpec       `json:"loggers,omitempty"`
	StructuredLoggers []StructuredLoggerSpec `json:"structuredLoggers,omitempty"`
	Affinity          *corev1.Affinity       `json:"affinity,omitempty"`
	NodeSelector      map[string]string      `json:"nodeSelector,omitempty"`
	Tolerations       []corev1.Toleration    `json:"tolerations,omitempty"`
	PodLabels         map[string]string      `json:"podLabels,omitempty"`
	PodAnnotations    map[string]string      `json:"podAnnotations,omitempty"`
	// SetHostnameAsFQDN indicates whether to set the hostname as FQDN.
	//+kubebuilder:default:=true
	SetHostnameAsFQDN *bool `json:"setHostnameAsFqdn,omitempty"`
	// Optional duration in seconds the pod needs to terminate gracefully.
	//+optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
	// Component config for native RPC bus transport.
	//+optional
	NativeTransport *RPCTransportSpec `json:"nativeTransport,omitempty"`
}

type MasterConnectionSpec struct {
	CellTag       int16    `json:"cellTag"`
	HostAddresses []string `json:"hostAddresses,omitempty"`
}

type MastersSpec struct {
	InstanceSpec         `json:",inline"`
	MasterConnectionSpec `json:",inline"`

	HostAddressLabel string `json:"hostAddressLabel,omitempty"`

	MaxSnapshotCountToKeep  *int `json:"maxSnapshotCountToKeep,omitempty"`
	MaxChangelogCountToKeep *int `json:"maxChangelogCountToKeep,omitempty"`

	// List of sidecar containers as yaml of core/v1 Container.
	Sidecars []string `json:"sidecars,omitempty"`
}

type HTTPTransportSpec struct {
	// Reference to kubernetes.io/tls secret.
	//+optional
	HTTPSSecret *corev1.LocalObjectReference `json:"httpsSecret,omitempty"`
	//+optional
	DisableHTTP bool `json:"disableHttp,omitempty"`
}

type HTTPProxiesSpec struct {
	InstanceSpec `json:",inline"`
	//+kubebuilder:default:=NodePort
	ServiceType   corev1.ServiceType `json:"serviceType,omitempty"`
	HttpNodePort  *int32             `json:"httpNodePort,omitempty"`
	HttpsNodePort *int32             `json:"httpsNodePort,omitempty"`
	//+kubebuilder:default:=default
	//+kubebuilder:validation:MinLength:=1
	Role string `json:"role,omitempty"`
	//+optional
	Transport HTTPTransportSpec `json:"transport,omitempty"`
}

type RPCTransportSpec struct {
	// Reference to kubernetes.io/tls secret.
	//+optional
	TLSSecret *corev1.LocalObjectReference `json:"tlsSecret,omitempty"`
	// Require encrypted connections, otherwise only when required by peer.
	//+optional
	TLSRequired bool `json:"tlsRequired,omitempty"`
	// Disable TLS certificate verification.
	//+optional
	TLSInsecure bool `json:"tlsInsecure,omitempty"`
	// Define alternative host name for certificate verification.
	//+optional
	TLSPeerAlternativeHostName string `json:"tlsPeerAlternativeHostName,omitempty"`
}

type RPCProxiesSpec struct {
	InstanceSpec `json:",inline"`
	ServiceType  *corev1.ServiceType `json:"serviceType,omitempty"`
	NodePort     *int32              `json:"nodePort,omitempty"`
	//+kubebuilder:default:=default
	//+kubebuilder:validation:MinLength:=1
	Role string `json:"role,omitempty"`
	//+optional
	Transport RPCTransportSpec `json:"transport,omitempty"`
}

type TCPProxiesSpec struct {
	InstanceSpec `json:",inline"`
	ServiceType  *corev1.ServiceType `json:"serviceType,omitempty"`
	//+kubebuilder:default:=32000
	MinPort int32 `json:"minPort"`
	// Number of ports to allocate for balancing service.
	//+kubebuilder:default:=20
	PortCount int32 `json:"portCount"`
	//+kubebuilder:default:=default
	//+kubebuilder:validation:MinLength:=1
	Role string `json:"role,omitempty"`
}

// ClusterNodesSpec is a common part of spec for nodes of all flavors.
type ClusterNodesSpec struct {
	// List of the node tags.
	Tags []string `json:"tags,omitempty"`
	// Name of the node rack.
	Rack string `json:"rack,omitempty"`
}

type DataNodesSpec struct {
	InstanceSpec `json:",inline"`
	// Common part of the cluster node spec.
	ClusterNodesSpec `json:",inline"`
	//+kubebuilder:default:=default
	//+kubebuilder:validation:MinLength:=1
	Name string `json:"name,omitempty"`
}

type CRIJobEnvironmentSpec struct {
	// Specifies wrapper for CRI service (i.e. containerd) command.
	//+optional
	EntrypointWrapper []string `json:"entrypointWrapper,omitempty"`
	// Sandbox (pause) image.
	//+optional
	SandboxImage *string `json:"sandboxImage,omitempty"`
	// Timeout for retrying CRI API calls.
	//+optional
	APIRetryTimeoutSeconds *int32 `json:"apiRetryTimeoutSeconds,omitempty"`
	// CRI namespace for jobs containers.
	//+optional
	CRINamespace *string `json:"criNamespace,omitempty"`
	// Base cgroup for jobs.
	//+optional
	BaseCgroup *string `json:"baseCgroup,omitempty"`
	// See: https://github.com/containerd/containerd/blob/main/docs/hosts.md
	//+optional
	RegistryConfigPath *string `json:"registryConfigPath,omitempty"`
	// Initial estimation for space required for pulling image into cache.
	//+optional
	ImageSizeEstimation *int64 `json:"imageSizeEstimation,omitempty"`
	// Multiplier for image size to account space used by unpacked images.
	//+optional
	ImageCompressionRatioEstimation *int32 `json:"imageCompressionRatioEstimation,omitempty"`
	// Always pull "latest" images.
	//+optional
	AlwaysPullLatestImage *bool `json:"alwaysPullLatestImage,omitempty"`
	// Pull images periodically.
	//+optional
	ImagePullPeriodSeconds *int32 `json:"imagePullPeriodSeconds,omitempty"`
}

type JobEnvironmentSpec struct {
	// Isolate job execution environment from exec node or not, by default true when possible.
	//+optional
	Isolated *bool `json:"isolated,omitempty"`
	// Count of slots for user jobs on each exec node, default is 5 per CPU.
	//+optional
	UserSlots *int `json:"userSlots,omitempty"`
	// CRI service configuration for running jobs in sidecar container.
	//+optional
	CRI *CRIJobEnvironmentSpec `json:"cri,omitempty"`
	// Pass artifacts as read-only bind-mounts rather than symlinks.
	//+optional
	UseArtifactBinds *bool `json:"useArtifactBinds,omitempty"`
	// Do not use slot user id for running jobs.
	//+optional
	DoNotSetUserId *bool `json:"doNotSetUserId,omitempty"`
}

type ExecNodesSpec struct {
	// label filter (for daemonset)
	InstanceSpec `json:",inline"`
	// Common part of the cluster node spec.
	ClusterNodesSpec `json:",inline"`
	//+kubebuilder:default:=default
	//+kubebuilder:validation:MinLength:=1
	Name string `json:"name,omitempty"`
	// List of init containers as yaml of core/v1 Container.
	InitContainers []string `json:"initContainers,omitempty"`
	// List of sidecar containers as yaml of core/v1 Container.
	Sidecars []string `json:"sidecars,omitempty"`
	//+kubebuilder:default:=true
	//+optional
	Privileged      bool             `json:"privileged"`
	JobProxyLoggers []TextLoggerSpec `json:"jobProxyLoggers,omitempty"`
	// Resources dedicated for running jobs.
	//+optional
	JobResources *corev1.ResourceRequirements `json:"jobResources,omitempty"`
	//+optional
	JobEnvironment *JobEnvironmentSpec `json:"jobEnvironment,omitempty"`
}

type TabletNodesSpec struct {
	// label filter (for daemonset)
	InstanceSpec `json:",inline"`
	// Common part of the cluster node spec.
	ClusterNodesSpec `json:",inline"`
	//+kubebuilder:default:=default
	//+kubebuilder:validation:MinLength:=1
	Name string `json:"name,omitempty"`
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
	Image *string `json:"image,omitempty"`
	//+kubebuilder:default:=NodePort
	ServiceType  corev1.ServiceType `json:"serviceType,omitempty"`
	HttpNodePort *int32             `json:"httpNodePort,omitempty"`
	//+kubebuilder:default:=true
	//+optional
	UseInsecureCookies bool                        `json:"useInsecureCookies"`
	Resources          corev1.ResourceRequirements `json:"resources,omitempty"`
	InstanceCount      int32                       `json:"instanceCount,omitempty"`

	// If defined it will be used for direct heavy url/commands like: read_table, write_table, etc.
	//+optional
	ExternalProxy *string `json:"externalProxy,omitempty"`
	// Odin is a service for monitoring the availability of YTsaurus clusters.
	//+optional
	OdinBaseUrl *string `json:"odinBaseUrl,omitempty"`

	ExtraEnvVariables []corev1.EnvVar `json:"extraEnvVariables,omitempty"`

	//+kubebuilder:default:=testing
	Environment string `json:"environment,omitempty"`
	//+kubebuilder:default:=lavander
	Theme       string  `json:"theme,omitempty"`
	Description *string `json:"description,omitempty"`
	Group       *string `json:"group,omitempty"`
	// When this is set to false, UI will use backend for downloading instead of proxy.
	// If this is set to true or omitted, UI use proxies, which is a default behaviour.
	//+optional
	DirectDownload *bool `json:"directDownload,omitempty"`
}

type QueryTrackerSpec struct {
	InstanceSpec `json:",inline"`
}

type StrawberryControllerSpec struct {
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	Image     *string                     `json:"image,omitempty"`
}

type YQLAgentSpec struct {
	InstanceSpec `json:",inline"`
}

type QueueAgentSpec struct {
	InstanceSpec `json:",inline"`
}

type DeprecatedSpytSpec struct {
	SparkVersion string `json:"sparkVersion,omitempty"`
	SpytVersion  string `json:"spytVersion,omitempty"`
}

type MasterCachesConnectionSpec struct {
	CellTag       int16    `json:"cellTagMasterCaches"`
	HostAddresses []string `json:"hostAddressesMasterCaches,omitempty"`
}

type MasterCachesSpec struct {
	InstanceSpec               `json:",inline"`
	MasterCachesConnectionSpec `json:",inline"`
	HostAddressLabel           string `json:"hostAddressesLabel,omitempty"`
}

// CommonSpec is a set of fields shared between `YtsaurusSpec` and `Remote*NodesSpec`.
// It is inlined in these specs.
type CommonSpec struct {
	CoreImage string `json:"coreImage,omitempty"`

	// Default docker image for user jobs.
	//+optional
	JobImage *string `json:"jobImage,omitempty"`

	// Reference to ConfigMap with trusted certificates: "ca.crt".
	//+optional
	CABundle *corev1.LocalObjectReference `json:"caBundle,omitempty"`

	// Common config for native RPC bus transport.
	//+optional
	NativeTransport *RPCTransportSpec `json:"nativeTransport,omitempty"`

	// Allow prioritizing performance over data safety. Useful for tests and experiments.
	//+kubebuilder:default:=false
	//+optional
	EphemeralCluster bool `json:"ephemeralCluster,omitempty"`

	//+kubebuilder:default:=false
	//+optional
	UseIPv6 bool `json:"useIpv6"`
	//+kubebuilder:default:=false
	//+optional
	UseIPv4 bool `json:"useIpv4"`
	//+kubebuilder:default:=true
	//+optional
	UseShortNames bool `json:"useShortNames"`
	// Use the host's network namespace for all components.
	//+kubebuilder:default:=false
	//+optional
	HostNetwork bool `json:"hostNetwork"`

	//+kubebuilder:default:=false
	//+optional
	UsePorto bool `json:"usePorto"`

	ExtraPodAnnotations map[string]string `json:"extraPodAnnotations,omitempty"`

	ConfigOverrides  *corev1.LocalObjectReference  `json:"configOverrides,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// YtsaurusSpec defines the desired state of Ytsaurus
type YtsaurusSpec struct {
	CommonSpec `json:",inline"`
	UIImage    string `json:"uiImage,omitempty"`

	AdminCredentials *corev1.LocalObjectReference `json:"adminCredentials,omitempty"`

	OauthService *OauthServiceSpec `json:"oauthService,omitempty"`

	//+kubebuilder:default:=true
	//+optional
	IsManaged bool `json:"isManaged"`
	//+kubebuilder:default:=true
	//+optional
	EnableFullUpdate bool `json:"enableFullUpdate"`
	//+optional
	//+kubebuilder:validation:Enum={"","Nothing","StatelessOnly","MasterOnly","TabletNodesOnly","ExecNodesOnly","Everything"}
	// UpdateSelector is an experimental field. Behaviour may change.
	// If UpdateSelector is not empty EnableFullUpdate is ignored.
	UpdateSelector UpdateSelector `json:"updateSelector"`

	Bootstrap *BootstrapSpec `json:"bootstrap,omitempty"`

	Discovery        DiscoverySpec `json:"discovery,omitempty"`
	PrimaryMasters   MastersSpec   `json:"primaryMasters,omitempty"`
	SecondaryMasters []MastersSpec `json:"secondaryMasters,omitempty"`
	//+optional
	MasterCaches *MasterCachesSpec `json:"masterCaches,omitempty"`
	// +kubebuilder:validation:MinItems:=1
	HTTPProxies []HTTPProxiesSpec `json:"httpProxies,omitempty"`
	RPCProxies  []RPCProxiesSpec  `json:"rpcProxies,omitempty"`
	TCPProxies  []TCPProxiesSpec  `json:"tcpProxies,omitempty"`
	// +kubebuilder:validation:MinItems:=1
	DataNodes        []DataNodesSpec       `json:"dataNodes,omitempty"`
	ExecNodes        []ExecNodesSpec       `json:"execNodes,omitempty"`
	Schedulers       *SchedulersSpec       `json:"schedulers,omitempty"`
	ControllerAgents *ControllerAgentsSpec `json:"controllerAgents,omitempty"`
	TabletNodes      []TabletNodesSpec     `json:"tabletNodes,omitempty"`

	StrawberryController     *StrawberryControllerSpec `json:"strawberry,omitempty"`
	DeprecatedChytController *StrawberryControllerSpec `json:"chyt,omitempty"`
	QueryTrackers            *QueryTrackerSpec         `json:"queryTrackers,omitempty"`
	Spyt                     *DeprecatedSpytSpec       `json:"spyt,omitempty"`
	YQLAgents                *YQLAgentSpec             `json:"yqlAgents,omitempty"`
	QueueAgents              *QueueAgentSpec           `json:"queueAgents,omitempty"`

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
	ClusterStateCancelUpdate    ClusterState = "CancelUpdate"
)

func IsReadyToUpdateClusterState(clusterState ClusterState) bool {
	return clusterState == ClusterStateRunning
}

type UpdateState string

const (
	UpdateStateNone                               UpdateState = "None"
	UpdateStatePossibilityCheck                   UpdateState = "PossibilityCheck"
	UpdateStateImpossibleToStart                  UpdateState = "ImpossibleToStart"
	UpdateStateWaitingForSafeModeEnabled          UpdateState = "WaitingForSafeModeEnabled"
	UpdateStateWaitingForTabletCellsSaving        UpdateState = "WaitingForTabletCellsSaving"
	UpdateStateWaitingForTabletCellsRemovingStart UpdateState = "WaitingForTabletCellsRemovingStart"
	UpdateStateWaitingForTabletCellsRemoved       UpdateState = "WaitingForTabletCellsRemoved"
	UpdateStateWaitingForSnapshots                UpdateState = "WaitingForSnapshots"
	UpdateStateWaitingForPodsRemoval              UpdateState = "WaitingForPodsRemoval"
	UpdateStateWaitingForPodsCreation             UpdateState = "WaitingForPodsCreation"
	UpdateStateWaitingForMasterExitReadOnly       UpdateState = "WaitingForMasterExitReadOnly"
	UpdateStateWaitingForTabletCellsRecovery      UpdateState = "WaitingForTabletCellsRecovery"
	UpdateStateWaitingForOpArchiveUpdatingPrepare UpdateState = "WaitingForOpArchiveUpdatingPrepare"
	UpdateStateWaitingForOpArchiveUpdate          UpdateState = "WaitingForOpArchiveUpdate"
	UpdateStateWaitingForQTStateUpdatingPrepare   UpdateState = "WaitingForQTStateUpdatingPrepare"
	UpdateStateWaitingForQTStateUpdate            UpdateState = "WaitingForQTStateUpdate"
	UpdateStateWaitingForSafeModeDisabled         UpdateState = "WaitingForSafeModeDisabled"
)

type TabletCellBundleInfo struct {
	Name            string `yson:",value" json:"name"`
	TabletCellCount int    `yson:"tablet_cell_count,attr" json:"tabletCellCount"`
}

type UpdateSelector string

const (
	// UpdateSelectorUnspecified means that selector is disabled and would be ignored completely.
	UpdateSelectorUnspecified UpdateSelector = ""
	// UpdateSelectorNothing means that no component could be updated.
	UpdateSelectorNothing UpdateSelector = "Nothing"
	// UpdateSelectorStatelessOnly means that only stateless components (everything but master and tablet nodes)
	// could be updated.
	UpdateSelectorStatelessOnly UpdateSelector = "StatelessOnly"
	// UpdateSelectorMasterOnly means that only master could be updated.
	UpdateSelectorMasterOnly UpdateSelector = "MasterOnly"
	// UpdateSelectorTabletNodesOnly means that only tablet nodes could be updated
	UpdateSelectorTabletNodesOnly UpdateSelector = "TabletNodesOnly"
	// UpdateSelectorExecNodesOnly means that only tablet nodes could be updated
	UpdateSelectorExecNodesOnly UpdateSelector = "ExecNodesOnly"
	// UpdateSelectorEverything means that all components could be updated.
	// With this setting and if master or tablet nodes need update all the components would be updated.
	UpdateSelectorEverything UpdateSelector = "Everything"
)

type UpdateFlow string

const (
	UpdateFlowNone        UpdateFlow = ""
	UpdateFlowStateless   UpdateFlow = "Stateless"
	UpdateFlowMaster      UpdateFlow = "Master"
	UpdateFlowTabletNodes UpdateFlow = "TabletNodes"
	UpdateFlowFull        UpdateFlow = "Full"
)

type UpdateStatus struct {
	//+kubebuilder:default:=None
	State      UpdateState `json:"state,omitempty"`
	Components []string    `json:"components,omitempty"`
	// Flow is an internal field that is needed to persist the chosen flow until the end of an update.
	// Flow can be on of ""(unspecified), Stateless, Master, TabletNodes, Full and update cluster stage
	// executes steps corresponding to that update flow.
	Flow                  UpdateFlow             `json:"flow,omitempty"`
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

//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=ytsaurus,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=ytsaurus/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.ytsaurus.tech,resources=ytsaurus/finalizers,verbs=update

//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="ClusterState",type="string",JSONPath=".status.state",description="State of Ytsaurus cluster"
// +kubebuilder:printcolumn:name="UpdateState",type="string",JSONPath=".status.updateStatus.state",description="Update state of Ytsaurus cluster"
// +kubebuilder:printcolumn:name="UpdatingComponents",type="string",JSONPath=".status.updateStatus.components",description="Updating components (for local update)"
// +kubebuilder:resource:path=ytsaurus,shortName=yt
// +kubebuilder:subresource:status

// Ytsaurus is the Schema for the ytsaurus API
type Ytsaurus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              YtsaurusSpec   `json:"spec,omitempty"`
	Status            YtsaurusStatus `json:"status,omitempty"`
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
