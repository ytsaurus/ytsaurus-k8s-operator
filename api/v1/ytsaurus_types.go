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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

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

// Volume is a reduced version of core/v1 Volume.
// Represents a named volume in a pod that may be accessed by any container in the pod.
type Volume struct {
	// name of the volume.
	// Must be a DNS_LABEL and unique within the pod.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// volumeSource represents the location and type of the mounted volume.
	// If not specified, the Volume is implied to be an EmptyDir.
	// This implied behavior is deprecated and will be removed in a future version.
	VolumeSource `json:",inline" protobuf:"bytes,2,opt,name=volumeSource"`
}

// VolumeSource is a reduced version of core/v1 VolumeSource.
// Represents the source of a volume to mount.
// Only one of its members may be specified.
type VolumeSource struct {
	// hostPath represents a pre-existing file or directory on the host
	// machine that is directly exposed to the container. This is generally
	// used for system agents or other privileged things that are allowed
	// to see the host machine. Most containers will NOT need this.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath
	// +optional
	HostPath *corev1.HostPathVolumeSource `json:"hostPath,omitempty" protobuf:"bytes,1,opt,name=hostPath"`
	// emptyDir represents a temporary directory that shares a pod's lifetime.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir
	// +optional
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty" protobuf:"bytes,2,opt,name=emptyDir"`
	// secret represents a secret that should populate this volume.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#secret
	// +optional
	Secret *corev1.SecretVolumeSource `json:"secret,omitempty" protobuf:"bytes,6,opt,name=secret"`
	// nfs represents an NFS mount on the host that shares a pod's lifetime
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs
	// +optional
	NFS *corev1.NFSVolumeSource `json:"nfs,omitempty" protobuf:"bytes,7,opt,name=nfs"`
	// iscsi represents an ISCSI Disk resource that is attached to a
	// kubelet's host machine and then exposed to the pod.
	// More info: https://examples.k8s.io/volumes/iscsi/README.md
	// +optional
	ISCSI *corev1.ISCSIVolumeSource `json:"iscsi,omitempty" protobuf:"bytes,8,opt,name=iscsi"`
	// persistentVolumeClaimVolumeSource represents a reference to a
	// PersistentVolumeClaim in the same namespace.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
	// +optional
	PersistentVolumeClaim *corev1.PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty" protobuf:"bytes,10,opt,name=persistentVolumeClaim"`
	// downwardAPI represents downward API about the pod that should populate this volume
	// +optional
	DownwardAPI *corev1.DownwardAPIVolumeSource `json:"downwardAPI,omitempty" protobuf:"bytes,16,opt,name=downwardAPI"`
	// fc represents a Fibre Channel resource that is attached to a kubelet's host machine and then exposed to the pod.
	// +optional
	FC *corev1.FCVolumeSource `json:"fc,omitempty" protobuf:"bytes,17,opt,name=fc"`
	// configMap represents a configMap that should populate this volume
	// +optional
	ConfigMap *corev1.ConfigMapVolumeSource `json:"configMap,omitempty" protobuf:"bytes,19,opt,name=configMap"`
	// csi (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers (Beta feature).
	// +optional
	CSI *corev1.CSIVolumeSource `json:"csi,omitempty" protobuf:"bytes,28,opt,name=csi"`
	// ephemeral represents a volume that is handled by a cluster storage driver.
	// The volume's lifecycle is tied to the pod that defines it - it will be created before the pod starts,
	// and deleted when the pod is removed.
	//
	// Use this if:
	// a) the volume is only needed while the pod runs,
	// b) features of normal volumes like restoring from snapshot or capacity
	//    tracking are needed,
	// c) the storage driver is specified through a storage class, and
	// d) the storage driver supports dynamic volume provisioning through
	//    a PersistentVolumeClaim (see EphemeralVolumeSource for more
	//    information on the connection between this volume type
	//    and PersistentVolumeClaim).
	//
	// Use PersistentVolumeClaim or one of the vendor-specific
	// APIs for volumes that persist for longer than the lifecycle
	// of an individual pod.
	//
	// Use CSI for light-weight local ephemeral volumes if the CSI driver is meant to
	// be used that way - see the documentation of the driver for
	// more information.
	//
	// A pod can use both types of ephemeral volumes and
	// persistent volumes at the same time.
	//
	// +optional
	Ephemeral *corev1.EphemeralVolumeSource `json:"ephemeral,omitempty" protobuf:"bytes,29,opt,name=ephemeral"`
	// image represents an OCI object (a container image or artifact) pulled and mounted on the kubelet's host machine.
	// The volume is resolved at pod startup depending on which PullPolicy value is provided:
	//
	// - Always: the kubelet always attempts to pull the reference. Container creation will fail If the pull fails.
	// - Never: the kubelet never pulls the reference and only uses a local image or artifact. Container creation will fail if the reference isn't present.
	// - IfNotPresent: the kubelet pulls if the reference isn't already present on disk. Container creation will fail if the reference isn't present and the pull fails.
	//
	// The volume gets re-resolved if the pod gets deleted and recreated, which means that new remote content will become available on pod recreation.
	// A failure to resolve or pull the image during pod startup will block containers from starting and may add significant latency. Failures will be retried using normal volume backoff and will be reported on the pod reason and message.
	// The types of objects that may be mounted by this volume are defined by the container runtime implementation on a host machine and at minimum must include all valid types supported by the container image field.
	// The OCI object gets mounted in a single directory (spec.containers[*].volumeMounts.mountPath) by merging the manifest layers in the same way as for container images.
	// The volume will be mounted read-only (ro) and non-executable files (noexec).
	// Sub path mounts for containers are not supported (spec.containers[*].volumeMounts.subpath).
	// The field spec.securityContext.fsGroupChangePolicy has no effect on this volume type.
	// +featureGate=ImageVolume
	// +optional
	Image *corev1.ImageVolumeSource `json:"image,omitempty" protobuf:"bytes,30,opt,name=image"`
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

// A reference to a specific 'key' within a ConfigMap or Secret resource.
type FileObjectReference struct {
	// Kind is the type of resource: ConfigMap or Secret.
	//+kubebuilder:validation:Enum=ConfigMap;Secret
	Kind string `json:"kind,omitempty"`
	// Name is the name of resource being referenced
	Name string `json:"name,omitempty"`
	// Key is the name of entry in ConfigMap or Secret.
	Key string `json:"key,omitempty"`
	// Items defines entries in ConfigMap or Secret, default: if Key is defined map to Path: "ca-certificates.crt".
	Items []corev1.KeyToPath `json:"items,omitempty"`
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
	RotationPeriodMilliseconds *int64             `json:"rotationPeriodMilliseconds,omitempty"`
	MaxSegmentSize             *resource.Quantity `json:"maxSegmentSize,omitempty"`
	MaxTotalSizeToKeep         *resource.Quantity `json:"maxTotalSizeToKeep,omitempty"`
	MaxSegmentCountToKeep      *int64             `json:"maxSegmentCountToKeep,omitempty"`
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
	UseTimestampSuffix bool `json:"useTimestampSuffix"`
	//+kubebuilder:default:=true
	//+optional
	EnableAnchorProfiling *bool              `json:"enableAnchorProfiling,omitempty"`
	RotationPolicy        *LogRotationPolicy `json:"rotationPolicy,omitempty"`
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
	TabletCellCount int     `json:"tabletCellCount,omitempty"`
	NodeTagFilter   *string `json:"nodeTagFilter,omitempty"`
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
	// LoginTransformations will be applied to the login field consequentially if set.
	// Result of the transformations is treated as YTsaurus OAuth user's username.
	LoginTransformations []OauthUserLoginTransformation `json:"loginTransformations,omitempty"`
}

type OauthUserLoginTransformation struct {
	// MatchPattern expects RE2 (https://github.com/google/re2/wiki/syntax) syntax.
	MatchPattern string `json:"matchPattern,omitempty"`
	Replacement  string `json:"replacement,omitempty"`
}

type OauthServiceSpec struct {
	//+kubebuilder:validation:MinLength:=1
	Host string `json:"host,omitempty"`
	//+kubebuilder:default:=80
	Port int `json:"port,omitempty"`
	//+kubebuilder:default:=false
	Secure   bool                     `json:"secure,omitempty"`
	UserInfo OauthUserInfoHandlerSpec `json:"userInfoHandler,omitempty"`
	// If DisableUserCreation is set, proxies will NOT create non-existing users with OAuth authentication.
	DisableUserCreation *bool `json:"disableUserCreation,omitempty"`
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

type MetricShard struct {
	Filter   []string `json:"filter,omitempty"`
	GridStep int32    `json:"gridStep,omitempty"`
}

type MetricExporter struct {
	Host         *string                `json:"host,omitempty"`
	InstanceTags map[string]string      `json:"instanceTags,omitempty"`
	Shards       map[string]MetricShard `json:"shards,omitempty"`
	GridStep     int32                  `json:"gridStep,omitempty"`
}

type PodSpec struct {
	// Labels for instance pods.
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// Annotations for instance pods.
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`

	// Node selector for instance and init job pods.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Tolerations for instance and init job pods.
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Runtime class for instance pods.
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`

	// Use the host's network namespace.
	HostNetwork *bool `json:"hostNetwork,omitempty"`
	// SetHostnameAsFQDN indicates whether to set the hostname as FQDN.
	//+kubebuilder:default:=true
	SetHostnameAsFQDN *bool `json:"setHostnameAsFqdn,omitempty"`
	// DNSPolicy defines how DNS will be configured.
	DNSPolicy *corev1.DNSPolicy `json:"dnsPolicy,omitempty"`
	// DNSConfig allows customizing the DNS settings.
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`
}

type InstanceSpec struct {
	PodSpec `json:",inline"`

	// Overrides coreImage for component.
	//+optional
	Image *string `json:"image,omitempty"`
	// Specifies wrapper for component container command.
	//+optional
	EntrypointWrapper []string             `json:"entrypointWrapper,omitempty"`
	Volumes           []Volume             `json:"volumes,omitempty"`
	VolumeMounts      []corev1.VolumeMount `json:"volumeMounts,omitempty"`
	//+optional
	ReadinessProbeParams *HealthcheckProbeParams `json:"readinessProbeParams,omitempty"`
	// Resources dedicated for component. Capacity is defined by requests, or limits for zero requests.
	Resources             corev1.ResourceRequirements     `json:"resources,omitempty"`
	InstanceCount         int32                           `json:"instanceCount,omitempty"`
	MinReadyInstanceCount *int                            `json:"minReadyInstanceCount,omitempty"`
	Locations             []LocationSpec                  `json:"locations,omitempty"`
	VolumeClaimTemplates  []EmbeddedPersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	// Deprecated: use Affinity.PodAntiAffinity instead.
	EnableAntiAffinity *bool `json:"enableAntiAffinity,omitempty"`
	//+optional
	MonitoringPort    *int32                 `json:"monitoringPort,omitempty"`
	MetricExporter    *MetricExporter        `json:"metricExporter,omitempty"`
	Loggers           []TextLoggerSpec       `json:"loggers,omitempty"`
	StructuredLoggers []StructuredLoggerSpec `json:"structuredLoggers,omitempty"`
	Affinity          *corev1.Affinity       `json:"affinity,omitempty"`
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

type HydraPersistenceUploaderSpec struct {
	Image *string `json:"image,omitempty"`
}

type TimbertruckSpec struct {
	Image         *string `json:"image,omitempty"`
	DirectoryPath *string `json:"directoryPath,omitempty"`
}

type MastersSpec struct {
	InstanceSpec         `json:",inline"`
	MasterConnectionSpec `json:",inline"`

	HostAddressLabel string `json:"hostAddressLabel,omitempty"`

	MaxSnapshotCountToKeep  *int `json:"maxSnapshotCountToKeep,omitempty"`
	MaxChangelogCountToKeep *int `json:"maxChangelogCountToKeep,omitempty"`

	HydraPersistenceUploader *HydraPersistenceUploaderSpec `json:"hydraPersistenceUploader,omitempty"`
	Timbertruck              *TimbertruckSpec              `json:"timbertruck,omitempty"`

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

type CHYTProxySpec struct {
	//+kubebuilder:default:=8123
	HttpPort     *int32 `json:"httpPort,omitempty"`
	HttpNodePort *int32 `json:"httpNodePort,omitempty"`
	//+kubebuilder:default:=8443
	HttpsPort     *int32 `json:"httpsPort,omitempty"`
	HttpsNodePort *int32 `json:"httpsNodePort,omitempty"`
}

type HTTPProxiesSpec struct {
	InstanceSpec `json:",inline"`
	//+kubebuilder:default:=NodePort
	ServiceType corev1.ServiceType `json:"serviceType,omitempty"`
	//+kubebuilder:default:=80
	HttpPort *int32 `json:"httpPort,omitempty"`
	//+kubebuilder:default:=443
	HttpsPort     *int32         `json:"httpsPort,omitempty"`
	HttpNodePort  *int32         `json:"httpNodePort,omitempty"`
	HttpsNodePort *int32         `json:"httpsNodePort,omitempty"`
	ChytProxy     *CHYTProxySpec `json:"chytProxy,omitempty"`
	//+kubebuilder:default:=default
	//+kubebuilder:validation:MinLength:=1
	Role string `json:"role,omitempty"`
	//+optional
	Transport HTTPTransportSpec `json:"transport,omitempty"`
}

type RPCTransportSpec struct {
	// Server certificate. Reference to kubernetes.io/tls secret.
	//+optional
	TLSSecret *corev1.LocalObjectReference `json:"tlsSecret,omitempty"`
	// Client certificate. Reference to kubernetes.io/tls secret.
	//+optional
	TLSClientSecret *corev1.LocalObjectReference `json:"tlsClientSecret,omitempty"`
	// Require secure TLS connections by server, otherwise only by client.
	//+optional
	TLSRequired bool `json:"tlsRequired,omitempty"`
	// Disable client certificate verification when TLS is required, if not required - verify neither.
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

type KafkaProxiesSpec struct {
	InstanceSpec `json:",inline"`
	ServiceType  *corev1.ServiceType `json:"serviceType,omitempty"`
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

type CRIServiceType string

const (
	CRIServiceNone       CRIServiceType = "none"
	CRIServiceContainerd CRIServiceType = "containerd"
	CRIServiceCRIO       CRIServiceType = "crio"
)

type CRIJobEnvironmentSpec struct {
	// CRI service: containerd (default), crio, none.
	//+optional
	//+kubebuilder:validation:Enum={"containerd","crio","none"}
	CRIService *CRIServiceType `json:"criService,omitempty"`
	// CRI service monitoring port, default is 10026, set 0 to disable.
	// +optional
	MonitoringPort *int32 `json:"monitoringPort,omitempty"`
	// Specifies wrapper for CRI service command, default: ["tini", "--"].
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
	// For containerd: https://github.com/containerd/containerd/blob/main/docs/hosts.md
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

// GPU information provider.
// +enum
type GPUInfoProviderType string

const (
	GPUInfoProviderNvidiaSMI    GPUInfoProviderType = "nvidia_smi"
	GPUInfoProviderGPUAgent     GPUInfoProviderType = "gpu_agent"
	GPUInfoProviderNvGpuManager GPUInfoProviderType = "nv_gpu_manager"
)

type GPUManagerSpec struct {
	GPUInfoProvider *GPUInfoProviderType `json:"gpuInfoProvider,omitempty"`
}

type NvidiaRuntimeSpec struct{}

type JobRuntimeSpec struct {
	// Use NVIDIA Container Runtime.
	//+optional
	Nvidia *NvidiaRuntimeSpec `json:"nvidia,omitempty"`
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
	// Container Runtime configuration for CRI service. Default: runc.
	//+optional
	Runtime *JobRuntimeSpec `json:"runtime,omitempty"`
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
	Privileged bool `json:"privileged"`
	//+optional
	GPUManager      *GPUManagerSpec  `json:"gpuManager,omitempty"`
	JobProxyLoggers []TextLoggerSpec `json:"jobProxyLoggers,omitempty"`
	// Resources dedicated for running jobs. Capacity is defined by requests, or limits for zero requests. Default: same limits as exec node with zero requests.
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

type OffshoreDataGatewaySpec struct {
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
	PodSpec `json:",inline"`

	Image *string `json:"image,omitempty"`
	//+kubebuilder:default:=NodePort
	ServiceType  corev1.ServiceType `json:"serviceType,omitempty"`
	HttpNodePort *int32             `json:"httpNodePort,omitempty"`
	// If defined allows insecure (over http) authentication.
	//+kubebuilder:default:=true
	//+optional
	UseInsecureCookies bool `json:"useInsecureCookies"`
	// Use secure connection to the cluster's http-proxies.
	//+kubebuilder:default:=false
	//+optional
	Secure        bool                        `json:"secure"`
	Resources     corev1.ResourceRequirements `json:"resources,omitempty"`
	InstanceCount int32                       `json:"instanceCount,omitempty"`

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
	PodSpec `json:",inline"`

	Resources     corev1.ResourceRequirements `json:"resources,omitempty"`
	Image         *string                     `json:"image,omitempty"`
	ExternalProxy *string                     `json:"externalProxy,omitempty"`
	// Supported controller families, for example: "chyt", "jupyt", "livy".
	ControllerFamilies []string `json:"controllerFamilies,omitempty"`
	// The family that will receive requests for domains that are not explicitly specified in http_controller_mappings.
	// For example, "chyt" (with `ControllerFamilies` set to {"chyt", "jupyt"} would mean
	// that requests to "foo.<domain>" will be processed by chyt controller.
	DefaultRouteFamily *string `json:"defaultRouteFamily,omitempty"`
	// Write logs to stderr.
	LogToStderr bool `json:"logToStderr,omitempty"`
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

type CypressProxiesSpec struct {
	InstanceSpec `json:",inline"`
	Disable      *bool `json:"disable,omitempty"`
}

type BundleControllerSpec struct {
	InstanceSpec `json:",inline"`
}

type TabletBalancerSpec struct {
	InstanceSpec `json:",inline"`
}

type ClusterFeatures struct {
	// RPC proxies have "public_rpc" address. Required for separated internal/public TLS CA.
	RPCProxyHavePublicAddress bool `json:"rpcProxyHavePublicAddress,omitempty"`
	// HTTP proxies have "chyt_http_server" and "chyt_https_server". Opens ports for access to chyt via HTTP proxy.
	HTTPProxyHaveChytAddress bool `json:"httpProxyHaveChytAddress,omitempty"`
	// HTTP proxies have "https" address. Use HTTPS for all communications.
	HTTPProxyHaveHTTPSAddress bool `json:"httpProxyHaveHttpsAddress,omitempty"`
	// Validate that only secure transports are allowed for cluster connections.
	SecureClusterTransports bool `json:"secureClusterTransports,omitempty"`
}

// CommonSpec is a set of fields shared between `YtsaurusSpec` and `Remote*NodesSpec`.
// It is inlined in these specs.
type CommonSpec struct {
	CoreImage string `json:"coreImage,omitempty"`

	ClusterFeatures *ClusterFeatures `json:"clusterFeatures,omitempty"`

	// Default docker image for user jobs.
	//+optional
	JobImage *string `json:"jobImage,omitempty"`

	// Reference to trusted root certificates. Default kind="ConfigMap", key="ca-certificates.crt".
	// Will replace system CA root bundle for all server and job containers.
	//+optional
	CARootBundle *FileObjectReference `json:"caRootBundle,omitempty"`

	// Reference to trusted native transport certificates. Default kind="ConfigMap", key="ca.crt".
	// By default will use system CA bundle, which could be set by caRootBundle.
	//+optional
	CABundle *FileObjectReference `json:"caBundle,omitempty"`

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
	//+optional
	KeepSocket *bool `json:"keepSocket,omitempty"`
	//+optional
	ForceTCP *bool `json:"forceTcp,omitempty"`

	// Do not add resource name into names of resources under control.
	// When enabled resource should not share namespace with other Ytsaurus.
	//+kubebuilder:default:=true
	//+optional
	UseShortNames bool `json:"useShortNames"`

	//+kubebuilder:default:=false
	//+optional
	UsePorto bool `json:"usePorto"`

	// Deprecated: Despite its name, it adds annotations to all resources. Use podAnnotations instead.
	ExtraPodAnnotations map[string]string `json:"extraPodAnnotations,omitempty"`

	ConfigOverrides  *corev1.LocalObjectReference  `json:"configOverrides,omitempty"`
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

type RemoteNodeReleaseStatus string

const (
	RemoteNodeReleaseStatusPending RemoteNodeReleaseStatus = "Pending"
	RemoteNodeReleaseStatusRunning RemoteNodeReleaseStatus = "Running"
)

// CommonRemoteNodeStatus is a set of fields shared between `Remote*NodesStatus`.
// It is inlined in these specs.
type CommonRemoteNodeStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Reflects resource generation which was used for updating status.
	ObservedGeneration int64                   `json:"observedGeneration,omitempty"`
	ReleaseStatus      RemoteNodeReleaseStatus `json:"releaseStatus,omitempty"`
}

// YtsaurusSpec defines the desired state of Ytsaurus
type YtsaurusSpec struct {
	CommonSpec `json:",inline"`

	// Default settings for instance and init job pods.
	PodSpec `json:",inline"`

	UIImage string `json:"uiImage,omitempty"`

	// requiresOperatorVersion is used to lock the YT spec to an Operator version or range.
	// Syntax: https://github.com/Masterminds/semver
	// Example: "~0.1.2" is equivalent to ">= 0.1.2, < 0.2.0".
	//+optional
	RequiresOperatorVersion string `json:"requiresOperatorVersion,omitempty"`

	AdminCredentials *corev1.LocalObjectReference `json:"adminCredentials,omitempty"`

	OauthService *OauthServiceSpec `json:"oauthService,omitempty"`

	//+kubebuilder:default:=true
	//+optional
	IsManaged bool `json:"isManaged"`
	//+kubebuilder:default:=true
	//+optional
	EnableFullUpdate bool `json:"enableFullUpdate"`
	//+optional
	//+kubebuilder:validation:Enum={"","Nothing","MasterOnly","DataNodesOnly","TabletNodesOnly","ExecNodesOnly","StatelessOnly","Everything"}
	//
	// Deprecated: UpdateSelector is going to be removed soon. Please use UpdateSelectors instead.
	UpdateSelector UpdateSelector `json:"updateSelector,omitempty"`

	//+optional
	// Experimental: api may change.
	// Controls the components that should be updated during the update process.
	UpdatePlan []ComponentUpdateSelector `json:"updatePlan,omitempty"`

	Bootstrap *BootstrapSpec `json:"bootstrap,omitempty"`

	Discovery        DiscoverySpec `json:"discovery,omitempty"`
	PrimaryMasters   MastersSpec   `json:"primaryMasters,omitempty"`
	SecondaryMasters []MastersSpec `json:"secondaryMasters,omitempty"`
	//+optional
	MasterCaches *MasterCachesSpec `json:"masterCaches,omitempty"`
	// +kubebuilder:validation:MinItems:=1
	HTTPProxies  []HTTPProxiesSpec  `json:"httpProxies,omitempty"`
	RPCProxies   []RPCProxiesSpec   `json:"rpcProxies,omitempty"`
	TCPProxies   []TCPProxiesSpec   `json:"tcpProxies,omitempty"`
	KafkaProxies []KafkaProxiesSpec `json:"kafkaProxies,omitempty"`
	// +kubebuilder:validation:MinItems:=1
	DataNodes        []DataNodesSpec       `json:"dataNodes,omitempty"`
	ExecNodes        []ExecNodesSpec       `json:"execNodes,omitempty"`
	Schedulers       *SchedulersSpec       `json:"schedulers,omitempty"`
	ControllerAgents *ControllerAgentsSpec `json:"controllerAgents,omitempty"`
	TabletNodes      []TabletNodesSpec     `json:"tabletNodes,omitempty"`

	StrawberryController *StrawberryControllerSpec `json:"strawberry,omitempty"`
	QueryTrackers        *QueryTrackerSpec         `json:"queryTrackers,omitempty"`
	Spyt                 *DeprecatedSpytSpec       `json:"spyt,omitempty"`
	YQLAgents            *YQLAgentSpec             `json:"yqlAgents,omitempty"`
	QueueAgents          *QueueAgentSpec           `json:"queueAgents,omitempty"`
	CypressProxies       *CypressProxiesSpec       `json:"cypressProxies,omitempty"`
	BundleController     *BundleControllerSpec     `json:"bundleController,omitempty"`
	TabletBalancer       *TabletBalancerSpec       `json:"tabletBalancers,omitempty"`

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
	UpdateStateNone                                  UpdateState = "None"
	UpdateStatePossibilityCheck                      UpdateState = "PossibilityCheck"
	UpdateStateImpossibleToStart                     UpdateState = "ImpossibleToStart"
	UpdateStateWaitingForSafeModeEnabled             UpdateState = "WaitingForSafeModeEnabled"
	UpdateStateWaitingForTabletCellsSaving           UpdateState = "WaitingForTabletCellsSaving"
	UpdateStateWaitingForTabletCellsRemovingStart    UpdateState = "WaitingForTabletCellsRemovingStart"
	UpdateStateWaitingForTabletCellsRemoved          UpdateState = "WaitingForTabletCellsRemoved"
	UpdateStateWaitingForImaginaryChunksAbsence      UpdateState = "WaitingForImaginaryChunksAbsence"
	UpdateStateWaitingForSnapshots                   UpdateState = "WaitingForSnapshots"
	UpdateStateWaitingForPodsRemoval                 UpdateState = "WaitingForPodsRemoval"
	UpdateStateWaitingForPodsCreation                UpdateState = "WaitingForPodsCreation"
	UpdateStateWaitingForMasterExitReadOnly          UpdateState = "WaitingForMasterExitReadOnly"
	UpdateStateWaitingForCypressPatch                UpdateState = "WaitingForCypressPatch"
	UpdateStateWaitingForTabletCellsRecovery         UpdateState = "WaitingForTabletCellsRecovery"
	UpdateStateWaitingForOpArchiveUpdatingPrepare    UpdateState = "WaitingForOpArchiveUpdatingPrepare"
	UpdateStateWaitingForOpArchiveUpdate             UpdateState = "WaitingForOpArchiveUpdate"
	UpdateStateWaitingForSidecarsInitializingPrepare UpdateState = "WaitingForSidecarsInitializingPrepare"
	UpdateStateWaitingForSidecarsInitialize          UpdateState = "WaitingForSidecarsInitialize"
	UpdateStateWaitingForQTStateUpdatingPrepare      UpdateState = "WaitingForQTStateUpdatingPrepare"
	UpdateStateWaitingForQTStateUpdate               UpdateState = "WaitingForQTStateUpdate"
	UpdateStateWaitingForQAStateUpdatingPrepare      UpdateState = "WaitingForQAStateUpdatingPrepare"
	UpdateStateWaitingForQAStateUpdate               UpdateState = "WaitingForQAStateUpdate"
	UpdateStateWaitingForYqlaUpdatingPrepare         UpdateState = "WaitingForYqlaUpdatingPrepare"
	UpdateStateWaitingForYqlaUpdate                  UpdateState = "WaitingForYqlaUpdate"
	UpdateStateWaitingForSafeModeDisabled            UpdateState = "WaitingForSafeModeDisabled"
	UpdateStateWaitingForTimbertruckPrepared         UpdateState = "WaitingForTimbertruckPrepared"
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
	// UpdateSelectorMasterOnly means that only master could be updated.
	UpdateSelectorMasterOnly UpdateSelector = "MasterOnly"
	// UpdateSelectorTabletNodesOnly means that only data nodes could be updated
	UpdateSelectorDataNodesOnly UpdateSelector = "DataNodesOnly"
	// UpdateSelectorTabletNodesOnly means that only tablet nodes could be updated
	UpdateSelectorTabletNodesOnly UpdateSelector = "TabletNodesOnly"
	// UpdateSelectorExecNodesOnly means that only tablet nodes could be updated
	UpdateSelectorExecNodesOnly UpdateSelector = "ExecNodesOnly"
	// UpdateSelectorStatelessOnly means that only stateless components (everything but master, data nodes, and tablet nodes)
	// could be updated.
	UpdateSelectorStatelessOnly UpdateSelector = "StatelessOnly"
	// UpdateSelectorEverything means that all components could be updated.
	// With this setting and if master or tablet nodes need update all the components would be updated.
	UpdateSelectorEverything UpdateSelector = "Everything"
)

type ComponentUpdateModeType string

const (
	ComponentUpdateModeTypeBulkUpdate    ComponentUpdateModeType = "BulkUpdate"
	ComponentUpdateModeTypeRollingUpdate ComponentUpdateModeType = "RollingUpdate"
	ComponentUpdateModeTypeOnDelete      ComponentUpdateModeType = "OnDelete"
)

type ComponentRollingUpdateMode struct {
	// BatchSize controls how many replicas can be rolled at once.
	BatchSize *int32 `json:"batchSize,omitempty"`
}

type ComponentOnDeleteUpdateMode struct {
	// empty struct for now
}

type ComponentUpdateStrategy struct {
	RunPreChecks  *bool                        `json:"runPreChecks,omitempty"`
	RollingUpdate *ComponentRollingUpdateMode  `json:"rollingUpdate,omitempty"`
	OnDelete      *ComponentOnDeleteUpdateMode `json:"onDelete,omitempty"`
}

type ComponentUpdateSelector struct {
	//+optional
	//+kubebuilder:validation:Enum={"","Nothing","Stateless","Everything"}
	Class consts.ComponentClass `json:"class,omitempty"`
	//+optional
	Component Component `json:"component,omitempty"`
	//+optional
	Strategy *ComponentUpdateStrategy `json:"strategy,omitempty"`
}

func (m *ComponentUpdateStrategy) Type() ComponentUpdateModeType {
	switch {
	case m.RollingUpdate != nil:
		return ComponentUpdateModeTypeRollingUpdate
	case m.OnDelete != nil:
		return ComponentUpdateModeTypeOnDelete
	default:
		return ComponentUpdateModeTypeBulkUpdate
	}
}

func (selector *ComponentUpdateSelector) GetUpdateStrategyType() ComponentUpdateModeType {
	if selector == nil || selector.Strategy == nil {
		return ""
	}
	return selector.Strategy.Type()
}

type UpdateFlow string

type UpdateStatus struct {
	//+kubebuilder:default:=None
	State UpdateState `json:"state,omitempty"`
	// Deprecated: Use updatingComponents instead.
	Components         []string    `json:"components,omitempty"`
	UpdatingComponents []Component `json:"updatingComponents,omitempty"`
	// UpdatingComponentsSummary is used only for representation in kubectl, since it only supports
	// "simple" JSONPath, and it is unclear how to force to print required data based on UpdatingComponents field.
	UpdatingComponentsSummary string `json:"updatingComponentsSummary,omitempty"`
	BlockedComponentsSummary  string `json:"blockedComponentsSummary,omitempty"`
	// Flow is an internal field that is needed to persist the chosen flow until the end of an update.
	// Flow can be on of ""(unspecified), Stateless, Master, TabletNodes, Full and update cluster stage
	// executes steps corresponding to that update flow.
	//
	// Deprecated: Use updatingComponents instead.
	Flow              UpdateFlow             `json:"flow,omitempty"`
	Conditions        []metav1.Condition     `json:"conditions,omitempty"`
	TabletCellBundles []TabletCellBundleInfo `json:"tabletCellBundles,omitempty"`
}

type Component struct {
	Name string               `json:"name,omitempty"`
	Type consts.ComponentType `json:"type,omitempty"`
}

func (c *Component) String() string {
	if c.Name == "" {
		return string(c.Type)
	}
	return fmt.Sprintf("%s(%s)", c.Type, c.Name)
}

// YtsaurusStatus defines the observed state of Ytsaurus
type YtsaurusStatus struct {
	//+kubebuilder:default:=Created
	State      ClusterState       `json:"state,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Reflects resource generation which was used for updating status.
	ObservedGeneration int64        `json:"observedGeneration,omitempty"`
	UpdateStatus       UpdateStatus `json:"updateStatus,omitempty"`
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

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"
//+kubebuilder:printcolumn:name="ClusterState",type="string",JSONPath=".status.state",description="State of Ytsaurus cluster"
//+kubebuilder:printcolumn:name="UpdateState",type="string",JSONPath=".status.updateStatus.state",description="Update state of Ytsaurus cluster"
//+kubebuilder:printcolumn:name="UpdatingComponents",type="string",JSONPath=".status.updateStatus.updatingComponentsSummary",description="Updating components"
//+kubebuilder:printcolumn:name="BlockedComponents",type="string",JSONPath=".status.updateStatus.blockedComponentsSummary",description="Blocked components"
//+kubebuilder:resource:path=ytsaurus,shortName=yt,categories=ytsaurus-all;yt-all
//+kubebuilder:subresource:status

// Ytsaurus is the Schema for the ytsaurus API
type Ytsaurus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              YtsaurusSpec   `json:"spec,omitempty"`
	Status            YtsaurusStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:metadata:labels="app.kubernetes.io/part-of=ytsaurus-k8s-operator"

// YtsaurusList contains a list of Ytsaurus
type YtsaurusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ytsaurus `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Ytsaurus{}, &YtsaurusList{})
}
