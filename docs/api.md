# API Reference

## Packages
- [cluster.ytsaurus.tech/v1](#clusterytsaurustechv1)


## cluster.ytsaurus.tech/v1

Package v1 contains API Schema definitions for the cluster v1 API group

### Resource Types
- [Chyt](#chyt)
- [RemoteExecNodes](#remoteexecnodes)
- [RemoteYtsaurus](#remoteytsaurus)
- [Spyt](#spyt)
- [Ytsaurus](#ytsaurus)



#### BaseLoggerSpec







_Appears in:_
- [StructuredLoggerSpec](#structuredloggerspec)
- [TextLoggerSpec](#textloggerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  | MinLength: 1 <br /> |
| `format` _[LogFormat](#logformat)_ |  | plain_text | Enum: [plain_text json yson] <br /> |
| `minLogLevel` _[LogLevel](#loglevel)_ |  | info | Enum: [trace debug info error] <br /> |
| `compression` _[LogCompression](#logcompression)_ |  | none | Enum: [none gzip zstd] <br /> |
| `useTimestampSuffix` _boolean_ |  | false |  |
| `rotationPolicy` _[LogRotationPolicy](#logrotationpolicy)_ |  |  |  |


#### BootstrapSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tabletCellBundles` _[BundlesBootstrapSpec](#bundlesbootstrapspec)_ |  |  |  |


#### BundleBootstrapSpec







_Appears in:_
- [BundlesBootstrapSpec](#bundlesbootstrapspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `snapshotMedium` _string_ |  |  |  |
| `changelogMedium` _string_ |  |  |  |
| `tabletCellCount` _integer_ |  | 1 |  |


#### BundlesBootstrapSpec







_Appears in:_
- [BootstrapSpec](#bootstrapspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sys` _[BundleBootstrapSpec](#bundlebootstrapspec)_ |  |  |  |
| `default` _[BundleBootstrapSpec](#bundlebootstrapspec)_ |  |  |  |


#### CRIJobEnvironmentSpec







_Appears in:_
- [JobEnvironmentSpec](#jobenvironmentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `entrypointWrapper` _string array_ | Specifies wrapper for CRI service (i.e. containerd) command. |  |  |
| `sandboxImage` _string_ | Sandbox (pause) image. |  |  |
| `apiRetryTimeoutSeconds` _integer_ | Timeout for retrying CRI API calls. |  |  |
| `criNamespace` _string_ | CRI namespace for jobs containers. |  |  |
| `baseCgroup` _string_ | Base cgroup for jobs. |  |  |


#### CategoriesFilter







_Appears in:_
- [TextLoggerSpec](#textloggerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[CategoriesFilterType](#categoriesfiltertype)_ |  |  | Enum: [exclude include] <br /> |
| `values` _string array_ |  |  | MinItems: 1 <br /> |


#### CategoriesFilterType

_Underlying type:_ _string_

CategoriesFilterType string describes types of possible log CategoriesFilter.



_Appears in:_
- [CategoriesFilter](#categoriesfilter)



#### Chyt



Chyt is the Schema for the chyts API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `Chyt` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ChytSpec](#chytspec)_ |  |  |  |


#### ChytSpec



ChytSpec defines the desired state of Chyt



_Appears in:_
- [Chyt](#chyt)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |
| `ytsaurus` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `image` _string_ |  |  |  |
| `makeDefault` _boolean_ |  | false |  |




#### ClusterNodesSpec



ClusterNodesSpec is a common part of spec for nodes of all flavors.



_Appears in:_
- [DataNodesSpec](#datanodesspec)
- [ExecNodesSpec](#execnodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [TabletNodesSpec](#tabletnodesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tags` _string array_ | List of the node tags. |  |  |
| `rack` _string_ | Name of the node rack. |  |  |


#### ClusterState

_Underlying type:_ _string_





_Appears in:_
- [YtsaurusStatus](#ytsaurusstatus)



#### CommonSpec



CommonSpec is a set of fields shared between `YtsaurusSpec` and `Remote*NodesSpec`.
It is inlined in these specs.



_Appears in:_
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `coreImage` _string_ |  |  |  |
| `jobImage` _string_ | Default docker image for user jobs. |  |  |
| `caBundle` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Reference to ConfigMap with trusted certificates: "ca.crt". |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Common config for native RPC bus transport. |  |  |
| `ephemeralCluster` _boolean_ | Allow prioritizing performance over data safety. Useful for tests and experiments. | false |  |
| `useIpv6` _boolean_ |  | false |  |
| `useIpv4` _boolean_ |  | false |  |
| `useShortNames` _boolean_ |  | true |  |
| `usePorto` _boolean_ |  | false |  |
| `hostNetwork` _boolean_ |  | false |  |
| `extraPodAnnotations` _object (keys:string, values:string)_ |  |  |  |
| `configOverrides` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |


#### ControllerAgentsSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### DataNodesSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `tags` _string array_ | List of the node tags. |  |  |
| `rack` _string_ | Name of the node rack. |  |  |
| `name` _string_ |  | default | MinLength: 1 <br /> |


#### DeprecatedSpytSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sparkVersion` _string_ |  |  |  |
| `spytVersion` _string_ |  |  |  |


#### DiscoverySpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### EmbeddedObjectMetadata



EmbeddedObjectMetadata contains a subset of the fields included in k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta
Only fields which are relevant to embedded resources are included.



_Appears in:_
- [EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name must be unique within a namespace. Is required when creating resources, although<br />some resources may allow a client to request the generation of an appropriate name<br />automatically. Name is primarily intended for creation idempotence and configuration<br />definition.<br />Cannot be updated.<br />More info: http://kubernetes.io/docs/user-guide/identifiers#names |  |  |
| `labels` _object (keys:string, values:string)_ | Map of string keys and values that can be used to organize and categorize<br />(scope and select) objects. May match selectors of replication controllers<br />and services.<br />More info: http://kubernetes.io/docs/user-guide/labels |  |  |
| `annotations` _object (keys:string, values:string)_ | Annotations is an unstructured key value map stored with a resource that may be<br />set by external tools to store and retrieve arbitrary metadata. They are not<br />queryable and should be preserved when modifying objects.<br />More info: http://kubernetes.io/docs/user-guide/annotations |  |  |


#### EmbeddedPersistentVolumeClaim



EmbeddedPersistentVolumeClaim is an embedded version of k8s.io/api/core/v1.PersistentVolumeClaim.
It contains TypeMeta and a reduced ObjectMeta.



_Appears in:_
- [ControllerAgentsSpec](#controlleragentsspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metadata` _[EmbeddedObjectMetadata](#embeddedobjectmetadata)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PersistentVolumeClaimSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimspec-v1-core)_ | Spec defines the desired characteristics of a volume requested by a pod author.<br />More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims |  |  |


#### ExecNodesSpec







_Appears in:_
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `tags` _string array_ | List of the node tags. |  |  |
| `rack` _string_ | Name of the node rack. |  |  |
| `name` _string_ |  | default | MinLength: 1 <br /> |
| `sidecars` _string array_ | List of sidecar containers as yaml of corev1.Container. |  |  |
| `privileged` _boolean_ |  | true |  |
| `jobProxyLoggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `jobResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for running jobs. |  |  |
| `jobEnvironment` _[JobEnvironmentSpec](#jobenvironmentspec)_ |  |  |  |


#### HTTPProxiesSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ |  | NodePort |  |
| `httpNodePort` _integer_ |  |  |  |
| `httpsNodePort` _integer_ |  |  |  |
| `role` _string_ |  | default | MinLength: 1 <br /> |
| `transport` _[HTTPTransportSpec](#httptransportspec)_ |  |  |  |


#### HTTPTransportSpec







_Appears in:_
- [HTTPProxiesSpec](#httpproxiesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `httpsSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Reference to kubernetes.io/tls secret. |  |  |
| `disableHttp` _boolean_ |  |  |  |


#### HealthcheckProbeParams







_Appears in:_
- [ControllerAgentsSpec](#controlleragentsspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `initialDelaySeconds` _integer_ |  |  |  |
| `timeoutSeconds` _integer_ |  |  |  |
| `periodSeconds` _integer_ |  |  |  |
| `successThreshold` _integer_ |  |  |  |
| `failureThreshold` _integer_ |  |  |  |


#### InstanceSpec







_Appears in:_
- [ControllerAgentsSpec](#controlleragentsspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### JobEnvironmentSpec







_Appears in:_
- [ExecNodesSpec](#execnodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `isolated` _boolean_ | Isolate job execution environment from exec node or not, by default true when possible. |  |  |
| `userSlots` _integer_ | Count of slots for user jobs on each exec node, default is 5 per CPU. |  |  |
| `cri` _[CRIJobEnvironmentSpec](#crijobenvironmentspec)_ | CRI service configuration for running jobs in sidecar container. |  |  |
| `useArtifactBinds` _boolean_ | Pass artifacts as read-only bind-mounts rather than symlinks. |  |  |
| `doNotSetUserId` _boolean_ | Do not use slot user id for running jobs. |  |  |


#### LocationSpec







_Appears in:_
- [ControllerAgentsSpec](#controlleragentsspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `locationType` _[LocationType](#locationtype)_ |  |  |  |
| `path` _string_ |  |  | MinLength: 1 <br /> |
| `medium` _string_ |  | default |  |


#### LocationType

_Underlying type:_ _string_

LocationType string describes types of disk locations for YT components.



_Appears in:_
- [LocationSpec](#locationspec)



#### LogCompression

_Underlying type:_ _string_





_Appears in:_
- [BaseLoggerSpec](#baseloggerspec)
- [StructuredLoggerSpec](#structuredloggerspec)
- [TextLoggerSpec](#textloggerspec)



#### LogFormat

_Underlying type:_ _string_





_Appears in:_
- [BaseLoggerSpec](#baseloggerspec)
- [StructuredLoggerSpec](#structuredloggerspec)
- [TextLoggerSpec](#textloggerspec)



#### LogLevel

_Underlying type:_ _string_

LogLevel string describes possible Ytsaurus logging level.



_Appears in:_
- [BaseLoggerSpec](#baseloggerspec)
- [StructuredLoggerSpec](#structuredloggerspec)
- [TextLoggerSpec](#textloggerspec)



#### LogRotationPolicy







_Appears in:_
- [BaseLoggerSpec](#baseloggerspec)
- [StructuredLoggerSpec](#structuredloggerspec)
- [TextLoggerSpec](#textloggerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `rotationPeriodMilliseconds` _integer_ |  |  |  |
| `maxSegmentSize` _integer_ |  |  |  |
| `maxTotalSizeToKeep` _integer_ |  |  |  |
| `maxSegmentCountToKeep` _integer_ |  |  |  |


#### LogWriterType

_Underlying type:_ _string_

LogWriterType string describes types of possible log writers.



_Appears in:_
- [TextLoggerSpec](#textloggerspec)



#### MasterCachesConnectionSpec







_Appears in:_
- [MasterCachesSpec](#mastercachesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cellTagMasterCaches` _integer_ |  |  |  |
| `hostAddressesMasterCaches` _string array_ |  |  |  |


#### MasterCachesSpec







_Appears in:_
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `cellTagMasterCaches` _integer_ |  |  |  |
| `hostAddressesMasterCaches` _string array_ |  |  |  |
| `hostAddressesLabel` _string_ |  |  |  |


#### MasterConnectionSpec







_Appears in:_
- [MastersSpec](#mastersspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cellTag` _integer_ |  |  |  |
| `hostAddresses` _string array_ |  |  |  |


#### MastersSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `cellTag` _integer_ |  |  |  |
| `hostAddresses` _string array_ |  |  |  |
| `hostAddressLabel` _string_ |  |  |  |
| `maxSnapshotCountToKeep` _integer_ |  |  |  |
| `maxChangelogCountToKeep` _integer_ |  |  |  |


#### OauthServiceSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `host` _string_ |  |  | MinLength: 1 <br /> |
| `port` _integer_ |  | 80 |  |
| `secure` _boolean_ |  | false |  |
| `userInfoHandler` _[OauthUserInfoHandlerSpec](#oauthuserinfohandlerspec)_ |  |  |  |


#### OauthUserInfoHandlerSpec







_Appears in:_
- [OauthServiceSpec](#oauthservicespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoint` _string_ |  | user/info |  |
| `loginField` _string_ |  | nickname |  |
| `errorField` _string_ |  |  |  |


#### QueryTrackerSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### QueueAgentSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### RPCProxiesSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ |  |  |  |
| `nodePort` _integer_ |  |  |  |
| `role` _string_ |  | default | MinLength: 1 <br /> |
| `transport` _[RPCTransportSpec](#rpctransportspec)_ |  |  |  |


#### RPCTransportSpec







_Appears in:_
- [CommonSpec](#commonspec)
- [ControllerAgentsSpec](#controlleragentsspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tlsSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Reference to kubernetes.io/tls secret. |  |  |
| `tlsRequired` _boolean_ | Require encrypted connections, otherwise only when required by peer. |  |  |
| `tlsInsecure` _boolean_ | Disable TLS certificate verification. |  |  |
| `tlsPeerAlternativeHostName` _string_ | Define alternative host name for certificate verification. |  |  |


#### RemoteExecNodes



RemoteExecNodes is the Schema for the remoteexecnodes API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `RemoteExecNodes` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RemoteExecNodesSpec](#remoteexecnodesspec)_ |  |  |  |


#### RemoteExecNodesSpec



RemoteExecNodesSpec defines the desired state of RemoteExecNodes



_Appears in:_
- [RemoteExecNodes](#remoteexecnodes)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `remoteClusterSpec` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `coreImage` _string_ |  |  |  |
| `jobImage` _string_ | Default docker image for user jobs. |  |  |
| `caBundle` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Reference to ConfigMap with trusted certificates: "ca.crt". |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Common config for native RPC bus transport. |  |  |
| `ephemeralCluster` _boolean_ | Allow prioritizing performance over data safety. Useful for tests and experiments. | false |  |
| `useIpv6` _boolean_ |  | false |  |
| `useIpv4` _boolean_ |  | false |  |
| `useShortNames` _boolean_ |  | true |  |
| `usePorto` _boolean_ |  | false |  |
| `hostNetwork` _boolean_ |  | false |  |
| `extraPodAnnotations` _object (keys:string, values:string)_ |  |  |  |
| `configOverrides` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `tags` _string array_ | List of the node tags. |  |  |
| `rack` _string_ | Name of the node rack. |  |  |
| `name` _string_ |  | default | MinLength: 1 <br /> |
| `sidecars` _string array_ | List of sidecar containers as yaml of corev1.Container. |  |  |
| `privileged` _boolean_ |  | true |  |
| `jobProxyLoggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `jobResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for running jobs. |  |  |
| `jobEnvironment` _[JobEnvironmentSpec](#jobenvironmentspec)_ |  |  |  |




#### RemoteYtsaurus



RemoteYtsaurus is the Schema for the remoteytsauruses API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `RemoteYtsaurus` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RemoteYtsaurusSpec](#remoteytsaurusspec)_ |  |  |  |


#### RemoteYtsaurusSpec



RemoteYtsaurusSpec defines the desired state of RemoteYtsaurus



_Appears in:_
- [RemoteYtsaurus](#remoteytsaurus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cellTag` _integer_ |  |  |  |
| `hostAddresses` _string array_ |  |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `cellTagMasterCaches` _integer_ |  |  |  |
| `hostAddressesMasterCaches` _string array_ |  |  |  |
| `hostAddressesLabel` _string_ |  |  |  |




#### SchedulersSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### Spyt



Spyt is the Schema for the spyts API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `Spyt` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[SpytSpec](#spytspec)_ |  |  |  |


#### SpytSpec



SpytSpec defines the desired state of Spyt



_Appears in:_
- [Spyt](#spyt)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |
| `ytsaurus` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `image` _string_ |  |  |  |




#### StrawberryControllerSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `image` _string_ |  |  |  |


#### StructuredLoggerSpec







_Appears in:_
- [ControllerAgentsSpec](#controlleragentsspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  | MinLength: 1 <br /> |
| `format` _[LogFormat](#logformat)_ |  | plain_text | Enum: [plain_text json yson] <br /> |
| `minLogLevel` _[LogLevel](#loglevel)_ |  | info | Enum: [trace debug info error] <br /> |
| `compression` _[LogCompression](#logcompression)_ |  | none | Enum: [none gzip zstd] <br /> |
| `useTimestampSuffix` _boolean_ |  | false |  |
| `rotationPolicy` _[LogRotationPolicy](#logrotationpolicy)_ |  |  |  |
| `category` _string_ |  |  |  |


#### TCPProxiesSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ |  |  |  |
| `minPort` _integer_ |  | 32000 |  |
| `portCount` _integer_ | Number of ports to allocate for balancing service. | 20 |  |
| `role` _string_ |  | default | MinLength: 1 <br /> |


#### TabletCellBundleInfo







_Appears in:_
- [UpdateStatus](#updatestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `tabletCellCount` _integer_ |  |  |  |


#### TabletNodesSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `tags` _string array_ | List of the node tags. |  |  |
| `rack` _string_ | Name of the node rack. |  |  |
| `name` _string_ |  | default | MinLength: 1 <br /> |


#### TextLoggerSpec







_Appears in:_
- [ControllerAgentsSpec](#controlleragentsspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  | MinLength: 1 <br /> |
| `format` _[LogFormat](#logformat)_ |  | plain_text | Enum: [plain_text json yson] <br /> |
| `minLogLevel` _[LogLevel](#loglevel)_ |  | info | Enum: [trace debug info error] <br /> |
| `compression` _[LogCompression](#logcompression)_ |  | none | Enum: [none gzip zstd] <br /> |
| `useTimestampSuffix` _boolean_ |  | false |  |
| `rotationPolicy` _[LogRotationPolicy](#logrotationpolicy)_ |  |  |  |
| `writerType` _[LogWriterType](#logwritertype)_ |  |  | Enum: [file stderr] <br /> |
| `categoriesFilter` _[CategoriesFilter](#categoriesfilter)_ |  |  |  |


#### UISpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ |  |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ |  | NodePort |  |
| `httpNodePort` _integer_ |  |  |  |
| `useInsecureCookies` _boolean_ |  | true |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `odinBaseUrl` _string_ |  |  |  |
| `extraEnvVariables` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ |  |  |  |
| `environment` _string_ |  | testing |  |
| `theme` _string_ |  | lavander |  |
| `description` _string_ |  |  |  |
| `group` _string_ |  |  |  |


#### UpdateFlow

_Underlying type:_ _string_





_Appears in:_
- [UpdateStatus](#updatestatus)



#### UpdateSelector

_Underlying type:_ _string_





_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)



#### UpdateState

_Underlying type:_ _string_





_Appears in:_
- [UpdateStatus](#updatestatus)



#### UpdateStatus







_Appears in:_
- [YtsaurusStatus](#ytsaurusstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[UpdateState](#updatestate)_ |  | None |  |
| `components` _string array_ |  |  |  |
| `updateStrategy` _[UpdateStrategy](#updatestrategy)_ |  |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#condition-v1-meta) array_ |  |  |  |
| `tabletCellBundles` _[TabletCellBundleInfo](#tabletcellbundleinfo) array_ |  |  |  |
| `masterMonitoringPaths` _string array_ |  |  |  |


#### UpdateStrategy

_Underlying type:_ _string_





_Appears in:_
- [UpdateStatus](#updatestatus)
- [YtsaurusSpec](#ytsaurusspec)



#### YQLAgentSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `runtimeClassName` _string_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### Ytsaurus



Ytsaurus is the Schema for the ytsaurus API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `Ytsaurus` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[YtsaurusSpec](#ytsaurusspec)_ |  |  |  |


#### YtsaurusSpec



YtsaurusSpec defines the desired state of Ytsaurus



_Appears in:_
- [Ytsaurus](#ytsaurus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `coreImage` _string_ |  |  |  |
| `jobImage` _string_ | Default docker image for user jobs. |  |  |
| `caBundle` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Reference to ConfigMap with trusted certificates: "ca.crt". |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Common config for native RPC bus transport. |  |  |
| `ephemeralCluster` _boolean_ | Allow prioritizing performance over data safety. Useful for tests and experiments. | false |  |
| `useIpv6` _boolean_ |  | false |  |
| `useIpv4` _boolean_ |  | false |  |
| `useShortNames` _boolean_ |  | true |  |
| `usePorto` _boolean_ |  | false |  |
| `hostNetwork` _boolean_ |  | false |  |
| `extraPodAnnotations` _object (keys:string, values:string)_ |  |  |  |
| `configOverrides` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |
| `uiImage` _string_ |  |  |  |
| `adminCredentials` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `oauthService` _[OauthServiceSpec](#oauthservicespec)_ |  |  |  |
| `isManaged` _boolean_ |  | true |  |
| `enableFullUpdate` _boolean_ |  | true |  |
| `updateStrategy` _[UpdateStrategy](#updatestrategy)_ | UpdateStrategy is an experimental field. Behaviour may change.<br />If UpdateStrategy is not empty EnableFullUpdate is ignored. |  |  |
| `bootstrap` _[BootstrapSpec](#bootstrapspec)_ |  |  |  |
| `discovery` _[DiscoverySpec](#discoveryspec)_ |  |  |  |
| `primaryMasters` _[MastersSpec](#mastersspec)_ |  |  |  |
| `secondaryMasters` _[MastersSpec](#mastersspec) array_ |  |  |  |
| `masterCaches` _[MasterCachesSpec](#mastercachesspec)_ |  |  |  |
| `httpProxies` _[HTTPProxiesSpec](#httpproxiesspec) array_ |  |  | MinItems: 1 <br /> |
| `rpcProxies` _[RPCProxiesSpec](#rpcproxiesspec) array_ |  |  |  |
| `tcpProxies` _[TCPProxiesSpec](#tcpproxiesspec) array_ |  |  |  |
| `dataNodes` _[DataNodesSpec](#datanodesspec) array_ |  |  | MinItems: 1 <br /> |
| `execNodes` _[ExecNodesSpec](#execnodesspec) array_ |  |  |  |
| `schedulers` _[SchedulersSpec](#schedulersspec)_ |  |  |  |
| `controllerAgents` _[ControllerAgentsSpec](#controlleragentsspec)_ |  |  |  |
| `tabletNodes` _[TabletNodesSpec](#tabletnodesspec) array_ |  |  |  |
| `strawberry` _[StrawberryControllerSpec](#strawberrycontrollerspec)_ |  |  |  |
| `chyt` _[StrawberryControllerSpec](#strawberrycontrollerspec)_ |  |  |  |
| `queryTrackers` _[QueryTrackerSpec](#querytrackerspec)_ |  |  |  |
| `spyt` _[DeprecatedSpytSpec](#deprecatedspytspec)_ |  |  |  |
| `yqlAgents` _[YQLAgentSpec](#yqlagentspec)_ |  |  |  |
| `queueAgents` _[QueueAgentSpec](#queueagentspec)_ |  |  |  |
| `ui` _[UISpec](#uispec)_ |  |  |  |




