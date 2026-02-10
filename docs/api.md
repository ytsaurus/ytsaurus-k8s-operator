# API Reference

## Packages
- [cluster.ytsaurus.tech/v1](#clusterytsaurustechv1)


## cluster.ytsaurus.tech/v1

Package v1 contains API Schema definitions for the cluster v1 API group

### Resource Types
- [Chyt](#chyt)
- [OffshoreDataGateways](#offshoredatagateways)
- [OffshoreDataGatewaysList](#offshoredatagatewayslist)
- [RemoteDataNodes](#remotedatanodes)
- [RemoteDataNodesList](#remotedatanodeslist)
- [RemoteExecNodes](#remoteexecnodes)
- [RemoteTabletNodes](#remotetabletnodes)
- [RemoteTabletNodesList](#remotetabletnodeslist)
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
| `minLogLevel` _[LogLevel](#loglevel)_ |  | info | Enum: [trace debug info warning error] <br /> |
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
| `nodeTagFilter` _string_ |  |  |  |


#### BundleControllerSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### BundlesBootstrapSpec







_Appears in:_
- [BootstrapSpec](#bootstrapspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sys` _[BundleBootstrapSpec](#bundlebootstrapspec)_ |  |  |  |
| `default` _[BundleBootstrapSpec](#bundlebootstrapspec)_ |  |  |  |


#### CHYTProxySpec







_Appears in:_
- [HTTPProxiesSpec](#httpproxiesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `httpPort` _integer_ |  | 8123 |  |
| `httpNodePort` _integer_ |  |  |  |
| `httpsPort` _integer_ |  | 8443 |  |
| `httpsNodePort` _integer_ |  |  |  |


#### CRIJobEnvironmentSpec







_Appears in:_
- [JobEnvironmentSpec](#jobenvironmentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `criService` _[CRIServiceType](#criservicetype)_ | CRI service: containerd (default), crio, none. |  | Enum: [containerd crio none] <br /> |
| `monitoringPort` _integer_ | CRI service monitoring port, default is 10026, set 0 to disable. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for CRI service command, default: ["tini", "--"]. |  |  |
| `sandboxImage` _string_ | Sandbox (pause) image. |  |  |
| `apiRetryTimeoutSeconds` _integer_ | Timeout for retrying CRI API calls. |  |  |
| `criNamespace` _string_ | CRI namespace for jobs containers. |  |  |
| `baseCgroup` _string_ | Base cgroup for jobs. |  |  |
| `registryConfigPath` _string_ | For containerd: https://github.com/containerd/containerd/blob/main/docs/hosts.md |  |  |
| `imageSizeEstimation` _integer_ | Initial estimation for space required for pulling image into cache. |  |  |
| `imageCompressionRatioEstimation` _integer_ | Multiplier for image size to account space used by unpacked images. |  |  |
| `alwaysPullLatestImage` _boolean_ | Always pull "latest" images. |  |  |
| `imagePullPeriodSeconds` _integer_ | Pull images periodically. |  |  |


#### CRIServiceType

_Underlying type:_ _string_





_Appears in:_
- [CRIJobEnvironmentSpec](#crijobenvironmentspec)

| Field | Description |
| --- | --- |
| `none` |  |
| `containerd` |  |
| `crio` |  |


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

| Field | Description |
| --- | --- |
| `exclude` |  |
| `include` |  |


#### Chyt



Chyt is the Schema for the chyts API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `Chyt` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ChytSpec](#chytspec)_ |  |  |  |
| `status` _[ChytStatus](#chytstatus)_ |  |  |  |


#### ChytReleaseStatus

_Underlying type:_ _string_





_Appears in:_
- [ChytStatus](#chytstatus)

| Field | Description |
| --- | --- |
| `CreatingUserSecret` |  |
| `CreatingUser` |  |
| `UploadingIntoCypress` |  |
| `CreatingChPublicClique` |  |
| `Finished` |  |


#### ChytSpec



ChytSpec defines the desired state of Chyt



_Appears in:_
- [Chyt](#chyt)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |
| `ytsaurus` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `image` _string_ |  |  |  |
| `makeDefault` _boolean_ | Mark specified image as default for cliques. | false |  |
| `createPublicClique` _boolean_ | Create ch_public clique, which is used by default when running CHYT queries. |  |  |


#### ChytStatus



ChytStatus defines the observed state of Chyt



_Appears in:_
- [Chyt](#chyt)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#condition-v1-meta) array_ |  |  |  |
| `releaseStatus` _[ChytReleaseStatus](#chytreleasestatus)_ |  |  |  |


#### ClusterFeatures







_Appears in:_
- [CommonSpec](#commonspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `rpcProxyHavePublicAddress` _boolean_ | RPC proxies have "public_rpc" address. Required for separated internal/public TLS CA. |  |  |
| `httpProxyHaveChytAddress` _boolean_ | HTTP proxies have "chyt_http_server" and "chyt_https_server". Opens ports for access to chyt via HTTP proxy. |  |  |
| `httpProxyHaveHttpsAddress` _boolean_ | HTTP proxies have "https" address. Use HTTPS for all communications. |  |  |
| `secureClusterTransports` _boolean_ | Validate that only secure transports are allowed for cluster connections. |  |  |
| `enableImageHeater` _boolean_ | EnableImageHeater enables image heater for the cluster initialization. Independent from the updatePlan logic. | false |  |


#### ClusterNodesSpec



ClusterNodesSpec is a common part of spec for nodes of all flavors.



_Appears in:_
- [DataNodesSpec](#datanodesspec)
- [ExecNodesSpec](#execnodesspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [TabletNodesSpec](#tabletnodesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tags` _string array_ | List of the node tags. |  |  |
| `rack` _string_ | Name of the node rack. |  |  |


#### ClusterState

_Underlying type:_ _string_





_Appears in:_
- [YtsaurusStatus](#ytsaurusstatus)

| Field | Description |
| --- | --- |
| `Created` |  |
| `Initializing` |  |
| `Running` |  |
| `Reconfiguration` |  |
| `Updating` |  |
| `UpdateFinishing` |  |
| `CancelUpdate` |  |


#### CommonRemoteNodeStatus



CommonRemoteNodeStatus is a set of fields shared between `Remote*NodesStatus`.
It is inlined in these specs.



_Appears in:_
- [OffshoreDataGatewaysStatus](#offshoredatagatewaysstatus)
- [RemoteDataNodesStatus](#remotedatanodesstatus)
- [RemoteExecNodesStatus](#remoteexecnodesstatus)
- [RemoteTabletNodesStatus](#remotetabletnodesstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | Reflects resource generation which was used for updating status. |  |  |
| `releaseStatus` _[RemoteNodeReleaseStatus](#remotenodereleasestatus)_ |  |  |  |


#### CommonSpec



CommonSpec is a set of fields shared between `YtsaurusSpec` and `Remote*NodesSpec`.
It is inlined in these specs.



_Appears in:_
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `coreImage` _string_ |  |  |  |
| `clusterFeatures` _[ClusterFeatures](#clusterfeatures)_ |  |  |  |
| `jobImage` _string_ | Default docker image for user jobs. |  |  |
| `caRootBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted root certificates. Default kind="ConfigMap", key="ca-certificates.crt".<br />Will replace system CA root bundle for all server and job containers. |  |  |
| `caBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted native transport certificates. Default kind="ConfigMap", key="ca.crt".<br />By default will use system CA bundle, which could be set by caRootBundle. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Common config for native RPC bus transport. |  |  |
| `ephemeralCluster` _boolean_ | Allow prioritizing performance over data safety. Useful for tests and experiments. | false |  |
| `useIpv6` _boolean_ |  | false |  |
| `useIpv4` _boolean_ |  | false |  |
| `keepSocket` _boolean_ |  |  |  |
| `forceTcp` _boolean_ |  |  |  |
| `useShortNames` _boolean_ | Do not add resource name into names of resources under control.<br />When enabled resource should not share namespace with other Ytsaurus. | true |  |
| `usePorto` _boolean_ |  | false |  |
| `extraPodAnnotations` _object (keys:string, values:string)_ | Deprecated: Despite its name, it adds annotations to all resources. Use podAnnotations instead. |  |  |
| `configOverrides` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |


#### Component







_Appears in:_
- [ComponentUpdateSelector](#componentupdateselector)
- [UpdateStatus](#updatestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `type` _[ComponentType](#componenttype)_ |  |  |  |


#### ComponentOnDeleteUpdateMode







_Appears in:_
- [ComponentUpdateStrategy](#componentupdatestrategy)



#### ComponentRollingUpdateMode







_Appears in:_
- [ComponentUpdateStrategy](#componentupdatestrategy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `batchSize` _integer_ | BatchSize controls how many replicas can be rolled at once. |  |  |




#### ComponentUpdateSelector







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `class` _[ComponentClass](#componentclass)_ |  |  | Enum: [ Nothing Stateless Everything] <br /> |
| `component` _[Component](#component)_ |  |  |  |
| `strategy` _[ComponentUpdateStrategy](#componentupdatestrategy)_ |  |  |  |


#### ComponentUpdateStrategy







_Appears in:_
- [ComponentUpdateSelector](#componentupdateselector)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `runPreChecks` _boolean_ |  |  |  |
| `rollingUpdate` _[ComponentRollingUpdateMode](#componentrollingupdatemode)_ |  |  |  |
| `onDelete` _[ComponentOnDeleteUpdateMode](#componentondeleteupdatemode)_ |  |  |  |


#### ControllerAgentsSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### CypressProxiesSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `disable` _boolean_ |  |  |  |


#### DataNodesSpec







_Appears in:_
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
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
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
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
- [BundleControllerSpec](#bundlecontrollerspec)
- [ControllerAgentsSpec](#controlleragentsspec)
- [CypressProxiesSpec](#cypressproxiesspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [KafkaProxiesSpec](#kafkaproxiesspec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [OffshoreDataGatewaySpec](#offshoredatagatewayspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletBalancerSpec](#tabletbalancerspec)
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
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `tags` _string array_ | List of the node tags. |  |  |
| `rack` _string_ | Name of the node rack. |  |  |
| `name` _string_ |  | default | MinLength: 1 <br /> |
| `initContainers` _string array_ | List of init containers as yaml of core/v1 Container. |  |  |
| `sidecars` _string array_ | List of sidecar containers as yaml of core/v1 Container. |  |  |
| `privileged` _boolean_ |  | true |  |
| `gpuManager` _[GPUManagerSpec](#gpumanagerspec)_ |  |  |  |
| `jobProxyLoggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `jobResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for running jobs. Capacity is defined by requests, or limits for zero requests. Default: same limits as exec node with zero requests. |  |  |
| `jobEnvironment` _[JobEnvironmentSpec](#jobenvironmentspec)_ |  |  |  |


#### FileObjectReference



A reference to a specific 'key' within a ConfigMap or Secret resource.



_Appears in:_
- [CommonSpec](#commonspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind is the type of resource: ConfigMap or Secret. |  | Enum: [ConfigMap Secret] <br /> |
| `name` _string_ | Name is the name of resource being referenced |  |  |
| `key` _string_ | Key is the name of entry in ConfigMap or Secret. |  |  |
| `items` _[KeyToPath](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#keytopath-v1-core) array_ | Items defines entries in ConfigMap or Secret, default: if Key is defined map to Path: "ca-certificates.crt". |  |  |


#### GPUInfoProviderType

_Underlying type:_ _string_

GPU information provider.



_Appears in:_
- [GPUManagerSpec](#gpumanagerspec)

| Field | Description |
| --- | --- |
| `nvidia_smi` |  |
| `gpu_agent` |  |
| `nv_gpu_manager` |  |


#### GPUManagerSpec







_Appears in:_
- [ExecNodesSpec](#execnodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `gpuInfoProvider` _[GPUInfoProviderType](#gpuinfoprovidertype)_ |  |  |  |


#### HTTPProxiesSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ |  | NodePort |  |
| `httpPort` _integer_ |  | 80 |  |
| `httpsPort` _integer_ |  | 443 |  |
| `httpNodePort` _integer_ |  |  |  |
| `httpsNodePort` _integer_ |  |  |  |
| `chytProxy` _[CHYTProxySpec](#chytproxyspec)_ |  |  |  |
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
- [BundleControllerSpec](#bundlecontrollerspec)
- [ControllerAgentsSpec](#controlleragentsspec)
- [CypressProxiesSpec](#cypressproxiesspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [KafkaProxiesSpec](#kafkaproxiesspec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [OffshoreDataGatewaySpec](#offshoredatagatewayspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletBalancerSpec](#tabletbalancerspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `initialDelaySeconds` _integer_ |  |  |  |
| `timeoutSeconds` _integer_ |  |  |  |
| `periodSeconds` _integer_ |  |  |  |
| `successThreshold` _integer_ |  |  |  |
| `failureThreshold` _integer_ |  |  |  |


#### HydraPersistenceUploaderSpec







_Appears in:_
- [MastersSpec](#mastersspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ |  |  |  |


#### InstanceSpec







_Appears in:_
- [BundleControllerSpec](#bundlecontrollerspec)
- [ControllerAgentsSpec](#controlleragentsspec)
- [CypressProxiesSpec](#cypressproxiesspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [KafkaProxiesSpec](#kafkaproxiesspec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [OffshoreDataGatewaySpec](#offshoredatagatewayspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletBalancerSpec](#tabletbalancerspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
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
| `runtime` _[JobRuntimeSpec](#jobruntimespec)_ | Container Runtime configuration for CRI service. Default: runc. |  |  |


#### JobRuntimeSpec







_Appears in:_
- [JobEnvironmentSpec](#jobenvironmentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nvidia` _[NvidiaRuntimeSpec](#nvidiaruntimespec)_ | Use NVIDIA Container Runtime. |  |  |


#### KafkaProxiesSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ |  |  |  |
| `role` _string_ |  | default | MinLength: 1 <br /> |


#### LocationSpec







_Appears in:_
- [BundleControllerSpec](#bundlecontrollerspec)
- [ControllerAgentsSpec](#controlleragentsspec)
- [CypressProxiesSpec](#cypressproxiesspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [KafkaProxiesSpec](#kafkaproxiesspec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [OffshoreDataGatewaySpec](#offshoredatagatewayspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletBalancerSpec](#tabletbalancerspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `locationType` _[LocationType](#locationtype)_ |  |  |  |
| `path` _string_ |  |  | MinLength: 1 <br /> |
| `medium` _string_ |  | default |  |
| `quota` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#quantity-resource-api)_ | Disk space quota, default is size of related volume. |  |  |
| `lowWatermark` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#quantity-resource-api)_ | Limit above which the volume is considered to be non-full. |  |  |
| `maxTrashMilliseconds` _integer_ | Max TTL of trash in milliseconds. |  | Minimum: 60000 <br /> |


#### LocationType

_Underlying type:_ _string_

LocationType string describes types of disk locations for YT components.



_Appears in:_
- [LocationSpec](#locationspec)

| Field | Description |
| --- | --- |
| `ChunkStore` |  |
| `ChunkCache` |  |
| `Slots` |  |
| `Logs` |  |
| `MasterChangelogs` |  |
| `MasterSnapshots` |  |
| `ImageCache` |  |


#### LogCompression

_Underlying type:_ _string_





_Appears in:_
- [BaseLoggerSpec](#baseloggerspec)
- [StructuredLoggerSpec](#structuredloggerspec)
- [TextLoggerSpec](#textloggerspec)

| Field | Description |
| --- | --- |
| `none` |  |
| `gzip` |  |
| `zstd` |  |


#### LogFormat

_Underlying type:_ _string_





_Appears in:_
- [BaseLoggerSpec](#baseloggerspec)
- [StructuredLoggerSpec](#structuredloggerspec)
- [TextLoggerSpec](#textloggerspec)

| Field | Description |
| --- | --- |
| `plain_text` |  |
| `yson` |  |
| `json` |  |


#### LogLevel

_Underlying type:_ _string_

LogLevel string describes possible Ytsaurus logging level.



_Appears in:_
- [BaseLoggerSpec](#baseloggerspec)
- [StructuredLoggerSpec](#structuredloggerspec)
- [TextLoggerSpec](#textloggerspec)

| Field | Description |
| --- | --- |
| `trace` |  |
| `debug` |  |
| `info` |  |
| `warning` |  |
| `error` |  |


#### LogRotationPolicy







_Appears in:_
- [BaseLoggerSpec](#baseloggerspec)
- [StructuredLoggerSpec](#structuredloggerspec)
- [TextLoggerSpec](#textloggerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `rotationPeriodMilliseconds` _integer_ |  |  |  |
| `maxSegmentSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#quantity-resource-api)_ |  |  |  |
| `maxTotalSizeToKeep` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#quantity-resource-api)_ |  |  |  |
| `maxSegmentCountToKeep` _integer_ |  |  |  |


#### LogWriterType

_Underlying type:_ _string_

LogWriterType string describes types of possible log writers.



_Appears in:_
- [TextLoggerSpec](#textloggerspec)

| Field | Description |
| --- | --- |
| `file` |  |
| `stderr` |  |


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
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
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
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `cellTag` _integer_ |  |  |  |
| `hostAddresses` _string array_ |  |  |  |
| `hostAddressLabel` _string_ |  |  |  |
| `maxSnapshotCountToKeep` _integer_ |  |  |  |
| `maxChangelogCountToKeep` _integer_ |  |  |  |
| `hydraPersistenceUploader` _[HydraPersistenceUploaderSpec](#hydrapersistenceuploaderspec)_ |  |  |  |
| `timbertruck` _[TimbertruckSpec](#timbertruckspec)_ |  |  |  |
| `sidecars` _string array_ | List of sidecar containers as yaml of core/v1 Container. |  |  |


#### MetricExporter







_Appears in:_
- [BundleControllerSpec](#bundlecontrollerspec)
- [ControllerAgentsSpec](#controlleragentsspec)
- [CypressProxiesSpec](#cypressproxiesspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [KafkaProxiesSpec](#kafkaproxiesspec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [OffshoreDataGatewaySpec](#offshoredatagatewayspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletBalancerSpec](#tabletbalancerspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `host` _string_ |  |  |  |
| `instanceTags` _object (keys:string, values:string)_ |  |  |  |
| `shards` _object (keys:string, values:[MetricShard](#metricshard))_ |  |  |  |
| `gridStep` _integer_ |  |  |  |


#### MetricShard







_Appears in:_
- [MetricExporter](#metricexporter)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `filter` _string array_ |  |  |  |
| `gridStep` _integer_ |  |  |  |


#### NvidiaRuntimeSpec







_Appears in:_
- [JobRuntimeSpec](#jobruntimespec)



#### OauthServiceSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `host` _string_ |  |  | MinLength: 1 <br /> |
| `port` _integer_ |  | 80 |  |
| `secure` _boolean_ |  | false |  |
| `userInfoHandler` _[OauthUserInfoHandlerSpec](#oauthuserinfohandlerspec)_ |  |  |  |
| `disableUserCreation` _boolean_ | If DisableUserCreation is set, proxies will NOT create non-existing users with OAuth authentication. |  |  |


#### OauthUserInfoHandlerSpec







_Appears in:_
- [OauthServiceSpec](#oauthservicespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoint` _string_ |  | user/info |  |
| `loginField` _string_ |  | nickname |  |
| `errorField` _string_ |  |  |  |
| `loginTransformations` _[OauthUserLoginTransformation](#oauthuserlogintransformation) array_ | LoginTransformations will be applied to the login field consequentially if set.<br />Result of the transformations is treated as YTsaurus OAuth user's username. |  |  |


#### OauthUserLoginTransformation







_Appears in:_
- [OauthUserInfoHandlerSpec](#oauthuserinfohandlerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `matchPattern` _string_ | MatchPattern expects RE2 (https://github.com/google/re2/wiki/syntax) syntax. |  |  |
| `replacement` _string_ |  |  |  |


#### OffshoreDataGatewaySpec







_Appears in:_
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### OffshoreDataGateways



OffshoreDataGateways is the Schema for the OffshoreDataGateways API
Be careful: this component is experimental and is not part of the public
API yet, so there are no guarantees it will work even when configured



_Appears in:_
- [OffshoreDataGatewaysList](#offshoredatagatewayslist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `OffshoreDataGateways` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)_ |  |  |  |
| `status` _[OffshoreDataGatewaysStatus](#offshoredatagatewaysstatus)_ |  |  |  |


#### OffshoreDataGatewaysList



OffshoreDataGatewaysList contains a list of OffshoreDataGateways





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `OffshoreDataGatewaysList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[OffshoreDataGateways](#offshoredatagateways) array_ |  |  |  |


#### OffshoreDataGatewaysSpec



OffshoreDataGatewaysSpec defines the desired state of OffshoreDataGateways



_Appears in:_
- [OffshoreDataGateways](#offshoredatagateways)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `remoteClusterSpec` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `coreImage` _string_ |  |  |  |
| `clusterFeatures` _[ClusterFeatures](#clusterfeatures)_ |  |  |  |
| `jobImage` _string_ | Default docker image for user jobs. |  |  |
| `caRootBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted root certificates. Default kind="ConfigMap", key="ca-certificates.crt".<br />Will replace system CA root bundle for all server and job containers. |  |  |
| `caBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted native transport certificates. Default kind="ConfigMap", key="ca.crt".<br />By default will use system CA bundle, which could be set by caRootBundle. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Common config for native RPC bus transport. |  |  |
| `ephemeralCluster` _boolean_ | Allow prioritizing performance over data safety. Useful for tests and experiments. | false |  |
| `useIpv6` _boolean_ |  | false |  |
| `useIpv4` _boolean_ |  | false |  |
| `keepSocket` _boolean_ |  |  |  |
| `forceTcp` _boolean_ |  |  |  |
| `useShortNames` _boolean_ | Do not add resource name into names of resources under control.<br />When enabled resource should not share namespace with other Ytsaurus. | true |  |
| `usePorto` _boolean_ |  | false |  |
| `extraPodAnnotations` _object (keys:string, values:string)_ | Deprecated: Despite its name, it adds annotations to all resources. Use podAnnotations instead. |  |  |
| `configOverrides` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### OffshoreDataGatewaysStatus



OffshoreDataGatewaysStatus defines the observed state of OffshoreDataGateways



_Appears in:_
- [OffshoreDataGateways](#offshoredatagateways)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | Reflects resource generation which was used for updating status. |  |  |
| `releaseStatus` _[RemoteNodeReleaseStatus](#remotenodereleasestatus)_ |  |  |  |


#### PodSpec







_Appears in:_
- [BundleControllerSpec](#bundlecontrollerspec)
- [ControllerAgentsSpec](#controlleragentsspec)
- [CypressProxiesSpec](#cypressproxiesspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [KafkaProxiesSpec](#kafkaproxiesspec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [OffshoreDataGatewaySpec](#offshoredatagatewayspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [StrawberryControllerSpec](#strawberrycontrollerspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletBalancerSpec](#tabletbalancerspec)
- [TabletNodesSpec](#tabletnodesspec)
- [UISpec](#uispec)
- [YQLAgentSpec](#yqlagentspec)
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |


#### QueryTrackerSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### QueueAgentSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### RPCProxiesSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ |  |  |  |
| `nodePort` _integer_ |  |  |  |
| `role` _string_ |  | default | MinLength: 1 <br /> |
| `transport` _[RPCTransportSpec](#rpctransportspec)_ |  |  |  |


#### RPCTransportSpec







_Appears in:_
- [BundleControllerSpec](#bundlecontrollerspec)
- [CommonSpec](#commonspec)
- [ControllerAgentsSpec](#controlleragentsspec)
- [CypressProxiesSpec](#cypressproxiesspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [KafkaProxiesSpec](#kafkaproxiesspec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [OffshoreDataGatewaySpec](#offshoredatagatewayspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletBalancerSpec](#tabletbalancerspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tlsSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Server certificate. Reference to kubernetes.io/tls secret. |  |  |
| `tlsClientSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Client certificate. Reference to kubernetes.io/tls secret. |  |  |
| `tlsRequired` _boolean_ | Require secure TLS connections by server, otherwise only by client. |  |  |
| `tlsInsecure` _boolean_ | Disable client certificate verification when TLS is required, if not required - verify neither. |  |  |
| `tlsPeerAlternativeHostName` _string_ | Define alternative host name for certificate verification. |  |  |


#### RemoteDataNodes



RemoteDataNodes is the Schema for the remotedatanodes API



_Appears in:_
- [RemoteDataNodesList](#remotedatanodeslist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `RemoteDataNodes` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RemoteDataNodesSpec](#remotedatanodesspec)_ |  |  |  |
| `status` _[RemoteDataNodesStatus](#remotedatanodesstatus)_ |  |  |  |


#### RemoteDataNodesList



RemoteDataNodesList contains a list of RemoteDataNodes





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `RemoteDataNodesList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[RemoteDataNodes](#remotedatanodes) array_ |  |  |  |


#### RemoteDataNodesSpec



RemoteDataNodesSpec defines the desired state of RemoteDataNodes



_Appears in:_
- [RemoteDataNodes](#remotedatanodes)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `remoteClusterSpec` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `coreImage` _string_ |  |  |  |
| `clusterFeatures` _[ClusterFeatures](#clusterfeatures)_ |  |  |  |
| `jobImage` _string_ | Default docker image for user jobs. |  |  |
| `caRootBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted root certificates. Default kind="ConfigMap", key="ca-certificates.crt".<br />Will replace system CA root bundle for all server and job containers. |  |  |
| `caBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted native transport certificates. Default kind="ConfigMap", key="ca.crt".<br />By default will use system CA bundle, which could be set by caRootBundle. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Common config for native RPC bus transport. |  |  |
| `ephemeralCluster` _boolean_ | Allow prioritizing performance over data safety. Useful for tests and experiments. | false |  |
| `useIpv6` _boolean_ |  | false |  |
| `useIpv4` _boolean_ |  | false |  |
| `keepSocket` _boolean_ |  |  |  |
| `forceTcp` _boolean_ |  |  |  |
| `useShortNames` _boolean_ | Do not add resource name into names of resources under control.<br />When enabled resource should not share namespace with other Ytsaurus. | true |  |
| `usePorto` _boolean_ |  | false |  |
| `extraPodAnnotations` _object (keys:string, values:string)_ | Deprecated: Despite its name, it adds annotations to all resources. Use podAnnotations instead. |  |  |
| `configOverrides` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `tags` _string array_ | List of the node tags. |  |  |
| `rack` _string_ | Name of the node rack. |  |  |
| `name` _string_ |  | default | MinLength: 1 <br /> |


#### RemoteDataNodesStatus



RemoteDataNodesStatus defines the observed state of RemoteDataNodes



_Appears in:_
- [RemoteDataNodes](#remotedatanodes)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | Reflects resource generation which was used for updating status. |  |  |
| `releaseStatus` _[RemoteNodeReleaseStatus](#remotenodereleasestatus)_ |  |  |  |


#### RemoteExecNodes



RemoteExecNodes is the Schema for the remoteexecnodes API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `RemoteExecNodes` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RemoteExecNodesSpec](#remoteexecnodesspec)_ |  |  |  |
| `status` _[RemoteExecNodesStatus](#remoteexecnodesstatus)_ |  |  |  |


#### RemoteExecNodesSpec



RemoteExecNodesSpec defines the desired state of RemoteExecNodes



_Appears in:_
- [RemoteExecNodes](#remoteexecnodes)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `remoteClusterSpec` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `coreImage` _string_ |  |  |  |
| `clusterFeatures` _[ClusterFeatures](#clusterfeatures)_ |  |  |  |
| `jobImage` _string_ | Default docker image for user jobs. |  |  |
| `caRootBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted root certificates. Default kind="ConfigMap", key="ca-certificates.crt".<br />Will replace system CA root bundle for all server and job containers. |  |  |
| `caBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted native transport certificates. Default kind="ConfigMap", key="ca.crt".<br />By default will use system CA bundle, which could be set by caRootBundle. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Common config for native RPC bus transport. |  |  |
| `ephemeralCluster` _boolean_ | Allow prioritizing performance over data safety. Useful for tests and experiments. | false |  |
| `useIpv6` _boolean_ |  | false |  |
| `useIpv4` _boolean_ |  | false |  |
| `keepSocket` _boolean_ |  |  |  |
| `forceTcp` _boolean_ |  |  |  |
| `useShortNames` _boolean_ | Do not add resource name into names of resources under control.<br />When enabled resource should not share namespace with other Ytsaurus. | true |  |
| `usePorto` _boolean_ |  | false |  |
| `extraPodAnnotations` _object (keys:string, values:string)_ | Deprecated: Despite its name, it adds annotations to all resources. Use podAnnotations instead. |  |  |
| `configOverrides` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `tags` _string array_ | List of the node tags. |  |  |
| `rack` _string_ | Name of the node rack. |  |  |
| `name` _string_ |  | default | MinLength: 1 <br /> |
| `initContainers` _string array_ | List of init containers as yaml of core/v1 Container. |  |  |
| `sidecars` _string array_ | List of sidecar containers as yaml of core/v1 Container. |  |  |
| `privileged` _boolean_ |  | true |  |
| `gpuManager` _[GPUManagerSpec](#gpumanagerspec)_ |  |  |  |
| `jobProxyLoggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `jobResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for running jobs. Capacity is defined by requests, or limits for zero requests. Default: same limits as exec node with zero requests. |  |  |
| `jobEnvironment` _[JobEnvironmentSpec](#jobenvironmentspec)_ |  |  |  |


#### RemoteExecNodesStatus



RemoteExecNodesStatus defines the observed state of RemoteExecNodes



_Appears in:_
- [RemoteExecNodes](#remoteexecnodes)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | Reflects resource generation which was used for updating status. |  |  |
| `releaseStatus` _[RemoteNodeReleaseStatus](#remotenodereleasestatus)_ |  |  |  |


#### RemoteNodeReleaseStatus

_Underlying type:_ _string_





_Appears in:_
- [CommonRemoteNodeStatus](#commonremotenodestatus)
- [OffshoreDataGatewaysStatus](#offshoredatagatewaysstatus)
- [RemoteDataNodesStatus](#remotedatanodesstatus)
- [RemoteExecNodesStatus](#remoteexecnodesstatus)
- [RemoteTabletNodesStatus](#remotetabletnodesstatus)

| Field | Description |
| --- | --- |
| `Pending` |  |
| `Running` |  |


#### RemoteTabletNodes



RemoteTabletNodes is the Schema for the remotetabletnodes API



_Appears in:_
- [RemoteTabletNodesList](#remotetabletnodeslist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `RemoteTabletNodes` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RemoteTabletNodesSpec](#remotetabletnodesspec)_ |  |  |  |
| `status` _[RemoteTabletNodesStatus](#remotetabletnodesstatus)_ |  |  |  |


#### RemoteTabletNodesList



RemoteTabletNodesList contains a list of RemoteTabletNodes





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `RemoteTabletNodesList` | | |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[RemoteTabletNodes](#remotetabletnodes) array_ |  |  |  |


#### RemoteTabletNodesSpec



RemoteTabletNodesSpec defines the desired state of RemoteTabletNodes



_Appears in:_
- [RemoteTabletNodes](#remotetabletnodes)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `remoteClusterSpec` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `coreImage` _string_ |  |  |  |
| `clusterFeatures` _[ClusterFeatures](#clusterfeatures)_ |  |  |  |
| `jobImage` _string_ | Default docker image for user jobs. |  |  |
| `caRootBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted root certificates. Default kind="ConfigMap", key="ca-certificates.crt".<br />Will replace system CA root bundle for all server and job containers. |  |  |
| `caBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted native transport certificates. Default kind="ConfigMap", key="ca.crt".<br />By default will use system CA bundle, which could be set by caRootBundle. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Common config for native RPC bus transport. |  |  |
| `ephemeralCluster` _boolean_ | Allow prioritizing performance over data safety. Useful for tests and experiments. | false |  |
| `useIpv6` _boolean_ |  | false |  |
| `useIpv4` _boolean_ |  | false |  |
| `keepSocket` _boolean_ |  |  |  |
| `forceTcp` _boolean_ |  |  |  |
| `useShortNames` _boolean_ | Do not add resource name into names of resources under control.<br />When enabled resource should not share namespace with other Ytsaurus. | true |  |
| `usePorto` _boolean_ |  | false |  |
| `extraPodAnnotations` _object (keys:string, values:string)_ | Deprecated: Despite its name, it adds annotations to all resources. Use podAnnotations instead. |  |  |
| `configOverrides` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `tags` _string array_ | List of the node tags. |  |  |
| `rack` _string_ | Name of the node rack. |  |  |
| `name` _string_ |  | default | MinLength: 1 <br /> |


#### RemoteTabletNodesStatus



RemoteTabletNodesStatus defines the observed state of RemoteTabletNodes



_Appears in:_
- [RemoteTabletNodes](#remotetabletnodes)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | Reflects resource generation which was used for updating status. |  |  |
| `releaseStatus` _[RemoteNodeReleaseStatus](#remotenodereleasestatus)_ |  |  |  |


#### RemoteYtsaurus



RemoteYtsaurus is the Schema for the remoteytsauruses API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `RemoteYtsaurus` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RemoteYtsaurusSpec](#remoteytsaurusspec)_ |  |  |  |
| `status` _[RemoteYtsaurusStatus](#remoteytsaurusstatus)_ |  |  |  |


#### RemoteYtsaurusSpec



RemoteYtsaurusSpec defines the desired state of RemoteYtsaurus



_Appears in:_
- [RemoteYtsaurus](#remoteytsaurus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cellTag` _integer_ |  |  |  |
| `hostAddresses` _string array_ |  |  |  |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `cellTagMasterCaches` _integer_ |  |  |  |
| `hostAddressesMasterCaches` _string array_ |  |  |  |
| `hostAddressesLabel` _string_ |  |  |  |


#### RemoteYtsaurusStatus



RemoteYtsaurusStatus defines the observed state of RemoteYtsaurus



_Appears in:_
- [RemoteYtsaurus](#remoteytsaurus)



#### SchedulersSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### Spyt



Spyt is the Schema for the spyts API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `Spyt` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[SpytSpec](#spytspec)_ |  |  |  |
| `status` _[SpytStatus](#spytstatus)_ |  |  |  |


#### SpytReleaseStatus

_Underlying type:_ _string_





_Appears in:_
- [SpytStatus](#spytstatus)

| Field | Description |
| --- | --- |
| `CreatingUserSecret` |  |
| `CreatingUser` |  |
| `UploadingIntoCypress` |  |
| `Finished` |  |


#### SpytSpec



SpytSpec defines the desired state of Spyt



_Appears in:_
- [Spyt](#spyt)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |
| `ytsaurus` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `image` _string_ |  |  |  |
| `sparkVersions` _string array_ | If multiple versions are provided, only the first one is used for query tracker dynamic config. |  |  |
| `sparkDistribOffline` _boolean_ | Use only local artifacts for spark distribution setup. |  |  |


#### SpytStatus



SpytStatus defines the observed state of Spyt



_Appears in:_
- [Spyt](#spyt)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#condition-v1-meta) array_ |  |  |  |
| `releaseStatus` _[SpytReleaseStatus](#spytreleasestatus)_ |  |  |  |


#### StrawberryControllerSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `image` _string_ |  |  |  |
| `externalProxy` _string_ |  |  |  |
| `controllerFamilies` _string array_ | Supported controller families, for example: "chyt", "jupyt", "livy". |  |  |
| `defaultRouteFamily` _string_ | The family that will receive requests for domains that are not explicitly specified in http_controller_mappings.<br />For example, "chyt" (with `ControllerFamilies` set to \{"chyt", "jupyt"\} would mean<br />that requests to "foo.<domain>" will be processed by chyt controller. |  |  |
| `logToStderr` _boolean_ | Write logs to stderr. |  |  |


#### StructuredLoggerSpec







_Appears in:_
- [BundleControllerSpec](#bundlecontrollerspec)
- [ControllerAgentsSpec](#controlleragentsspec)
- [CypressProxiesSpec](#cypressproxiesspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [KafkaProxiesSpec](#kafkaproxiesspec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [OffshoreDataGatewaySpec](#offshoredatagatewayspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletBalancerSpec](#tabletbalancerspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  | MinLength: 1 <br /> |
| `format` _[LogFormat](#logformat)_ |  | plain_text | Enum: [plain_text json yson] <br /> |
| `minLogLevel` _[LogLevel](#loglevel)_ |  | info | Enum: [trace debug info warning error] <br /> |
| `compression` _[LogCompression](#logcompression)_ |  | none | Enum: [none gzip zstd] <br /> |
| `useTimestampSuffix` _boolean_ |  | false |  |
| `rotationPolicy` _[LogRotationPolicy](#logrotationpolicy)_ |  |  |  |
| `category` _string_ |  |  |  |


#### TCPProxiesSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ |  |  |  |
| `minPort` _integer_ |  | 32000 |  |
| `portCount` _integer_ | Number of ports to allocate for balancing service. | 20 |  |
| `role` _string_ |  | default | MinLength: 1 <br /> |


#### TabletBalancerSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### TabletCellBundleInfo







_Appears in:_
- [UpdateStatus](#updatestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `tabletCellCount` _integer_ |  |  |  |


#### TabletNodesSpec







_Appears in:_
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |
| `tags` _string array_ | List of the node tags. |  |  |
| `rack` _string_ | Name of the node rack. |  |  |
| `name` _string_ |  | default | MinLength: 1 <br /> |


#### TextLoggerSpec







_Appears in:_
- [BundleControllerSpec](#bundlecontrollerspec)
- [ControllerAgentsSpec](#controlleragentsspec)
- [CypressProxiesSpec](#cypressproxiesspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [KafkaProxiesSpec](#kafkaproxiesspec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [OffshoreDataGatewaySpec](#offshoredatagatewayspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletBalancerSpec](#tabletbalancerspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  | MinLength: 1 <br /> |
| `format` _[LogFormat](#logformat)_ |  | plain_text | Enum: [plain_text json yson] <br /> |
| `minLogLevel` _[LogLevel](#loglevel)_ |  | info | Enum: [trace debug info warning error] <br /> |
| `compression` _[LogCompression](#logcompression)_ |  | none | Enum: [none gzip zstd] <br /> |
| `useTimestampSuffix` _boolean_ |  | false |  |
| `rotationPolicy` _[LogRotationPolicy](#logrotationpolicy)_ |  |  |  |
| `writerType` _[LogWriterType](#logwritertype)_ |  |  | Enum: [file stderr] <br /> |
| `categoriesFilter` _[CategoriesFilter](#categoriesfilter)_ |  |  |  |


#### TimbertruckSpec







_Appears in:_
- [MastersSpec](#mastersspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ |  |  |  |
| `directoryPath` _string_ |  |  |  |


#### UISpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ |  |  |  |
| `serviceType` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ |  | NodePort |  |
| `httpNodePort` _integer_ |  |  |  |
| `useInsecureCookies` _boolean_ | If defined allows insecure (over http) authentication. | true |  |
| `secure` _boolean_ | Use secure connection to the cluster's http-proxies. | false |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `instanceCount` _integer_ |  |  |  |
| `externalProxy` _string_ | If defined it will be used for direct heavy url/commands like: read_table, write_table, etc. |  |  |
| `odinBaseUrl` _string_ | Odin is a service for monitoring the availability of YTsaurus clusters. |  |  |
| `extraEnvVariables` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ |  |  |  |
| `environment` _string_ |  | testing |  |
| `theme` _string_ |  | lavander |  |
| `description` _string_ |  |  |  |
| `group` _string_ |  |  |  |
| `directDownload` _boolean_ | When this is set to false, UI will use backend for downloading instead of proxy.<br />If this is set to true or omitted, UI use proxies, which is a default behaviour. |  |  |


#### UpdateFlow

_Underlying type:_ _string_





_Appears in:_
- [UpdateStatus](#updatestatus)



#### UpdateSelector

_Underlying type:_ _string_





_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description |
| --- | --- |
| `` | UpdateSelectorUnspecified means that selector is disabled and would be ignored completely.<br /> |
| `Nothing` | UpdateSelectorNothing means that no component could be updated.<br /> |
| `MasterOnly` | UpdateSelectorMasterOnly means that only master could be updated.<br /> |
| `DataNodesOnly` | UpdateSelectorTabletNodesOnly means that only data nodes could be updated<br /> |
| `TabletNodesOnly` | UpdateSelectorTabletNodesOnly means that only tablet nodes could be updated<br /> |
| `ExecNodesOnly` | UpdateSelectorExecNodesOnly means that only tablet nodes could be updated<br /> |
| `StatelessOnly` | UpdateSelectorStatelessOnly means that only stateless components (everything but master, data nodes, and tablet nodes)<br />could be updated.<br /> |
| `Everything` | UpdateSelectorEverything means that all components could be updated.<br />With this setting and if master or tablet nodes need update all the components would be updated.<br /> |


#### UpdateState

_Underlying type:_ _string_





_Appears in:_
- [UpdateStatus](#updatestatus)

| Field | Description |
| --- | --- |
| `None` |  |
| `WaitingForImageHeater` |  |
| `PossibilityCheck` |  |
| `ImpossibleToStart` |  |
| `WaitingForSafeModeEnabled` |  |
| `WaitingForTabletCellsSaving` |  |
| `WaitingForTabletCellsRemovingStart` |  |
| `WaitingForTabletCellsRemoved` |  |
| `WaitingForImaginaryChunksAbsence` |  |
| `WaitingForSnapshots` |  |
| `WaitingForPodsRemoval` |  |
| `WaitingForPodsCreation` |  |
| `WaitingForMasterExitReadOnly` |  |
| `WaitingForCypressPatch` |  |
| `WaitingForTabletCellsRecovery` |  |
| `WaitingForOpArchiveUpdatingPrepare` |  |
| `WaitingForOpArchiveUpdate` |  |
| `WaitingForSidecarsInitializingPrepare` |  |
| `WaitingForSidecarsInitialize` |  |
| `WaitingForQTStateUpdatingPrepare` |  |
| `WaitingForQTStateUpdate` |  |
| `WaitingForQAStateUpdatingPrepare` |  |
| `WaitingForQAStateUpdate` |  |
| `WaitingForYqlaUpdatingPrepare` |  |
| `WaitingForYqlaUpdate` |  |
| `WaitingForSafeModeDisabled` |  |
| `WaitingForTimbertruckPrepared` |  |


#### UpdateStatus







_Appears in:_
- [YtsaurusStatus](#ytsaurusstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[UpdateState](#updatestate)_ |  | None |  |
| `components` _string array_ | Deprecated: Use updatingComponents instead. |  |  |
| `updatingComponents` _[Component](#component) array_ |  |  |  |
| `updatingComponentsSummary` _string_ | UpdatingComponentsSummary is used only for representation in kubectl, since it only supports<br />"simple" JSONPath, and it is unclear how to force to print required data based on UpdatingComponents field. |  |  |
| `blockedComponentsSummary` _string_ |  |  |  |
| `flow` _[UpdateFlow](#updateflow)_ | Flow is an internal field that is needed to persist the chosen flow until the end of an update.<br />Flow can be on of ""(unspecified), Stateless, Master, TabletNodes, Full and update cluster stage<br />executes steps corresponding to that update flow.<br />Deprecated: Use updatingComponents instead. |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#condition-v1-meta) array_ |  |  |  |
| `tabletCellBundles` _[TabletCellBundleInfo](#tabletcellbundleinfo) array_ |  |  |  |


#### Volume



Volume is a reduced version of core/v1 Volume.
Represents a named volume in a pod that may be accessed by any container in the pod.



_Appears in:_
- [BundleControllerSpec](#bundlecontrollerspec)
- [ControllerAgentsSpec](#controlleragentsspec)
- [CypressProxiesSpec](#cypressproxiesspec)
- [DataNodesSpec](#datanodesspec)
- [DiscoverySpec](#discoveryspec)
- [ExecNodesSpec](#execnodesspec)
- [HTTPProxiesSpec](#httpproxiesspec)
- [InstanceSpec](#instancespec)
- [KafkaProxiesSpec](#kafkaproxiesspec)
- [MasterCachesSpec](#mastercachesspec)
- [MastersSpec](#mastersspec)
- [OffshoreDataGatewaySpec](#offshoredatagatewayspec)
- [OffshoreDataGatewaysSpec](#offshoredatagatewaysspec)
- [QueryTrackerSpec](#querytrackerspec)
- [QueueAgentSpec](#queueagentspec)
- [RPCProxiesSpec](#rpcproxiesspec)
- [RemoteDataNodesSpec](#remotedatanodesspec)
- [RemoteExecNodesSpec](#remoteexecnodesspec)
- [RemoteTabletNodesSpec](#remotetabletnodesspec)
- [RemoteYtsaurusSpec](#remoteytsaurusspec)
- [SchedulersSpec](#schedulersspec)
- [TCPProxiesSpec](#tcpproxiesspec)
- [TabletBalancerSpec](#tabletbalancerspec)
- [TabletNodesSpec](#tabletnodesspec)
- [YQLAgentSpec](#yqlagentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | name of the volume.<br />Must be a DNS_LABEL and unique within the pod.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |  |  |
| `hostPath` _[HostPathVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostpathvolumesource-v1-core)_ | hostPath represents a pre-existing file or directory on the host<br />machine that is directly exposed to the container. This is generally<br />used for system agents or other privileged things that are allowed<br />to see the host machine. Most containers will NOT need this.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath |  |  |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#emptydirvolumesource-v1-core)_ | emptyDir represents a temporary directory that shares a pod's lifetime.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir |  |  |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretvolumesource-v1-core)_ | secret represents a secret that should populate this volume.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#secret |  |  |
| `nfs` _[NFSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#nfsvolumesource-v1-core)_ | nfs represents an NFS mount on the host that shares a pod's lifetime<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs |  |  |
| `iscsi` _[ISCSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#iscsivolumesource-v1-core)_ | iscsi represents an ISCSI Disk resource that is attached to a<br />kubelet's host machine and then exposed to the pod.<br />More info: https://examples.k8s.io/volumes/iscsi/README.md |  |  |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimvolumesource-v1-core)_ | persistentVolumeClaimVolumeSource represents a reference to a<br />PersistentVolumeClaim in the same namespace.<br />More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims |  |  |
| `downwardAPI` _[DownwardAPIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#downwardapivolumesource-v1-core)_ | downwardAPI represents downward API about the pod that should populate this volume |  |  |
| `fc` _[FCVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#fcvolumesource-v1-core)_ | fc represents a Fibre Channel resource that is attached to a kubelet's host machine and then exposed to the pod. |  |  |
| `configMap` _[ConfigMapVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapvolumesource-v1-core)_ | configMap represents a configMap that should populate this volume |  |  |
| `csi` _[CSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#csivolumesource-v1-core)_ | csi (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers (Beta feature). |  |  |
| `ephemeral` _[EphemeralVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#ephemeralvolumesource-v1-core)_ | ephemeral represents a volume that is handled by a cluster storage driver.<br />The volume's lifecycle is tied to the pod that defines it - it will be created before the pod starts,<br />and deleted when the pod is removed.<br />Use this if:<br />a) the volume is only needed while the pod runs,<br />b) features of normal volumes like restoring from snapshot or capacity<br />   tracking are needed,<br />c) the storage driver is specified through a storage class, and<br />d) the storage driver supports dynamic volume provisioning through<br />   a PersistentVolumeClaim (see EphemeralVolumeSource for more<br />   information on the connection between this volume type<br />   and PersistentVolumeClaim).<br />Use PersistentVolumeClaim or one of the vendor-specific<br />APIs for volumes that persist for longer than the lifecycle<br />of an individual pod.<br />Use CSI for light-weight local ephemeral volumes if the CSI driver is meant to<br />be used that way - see the documentation of the driver for<br />more information.<br />A pod can use both types of ephemeral volumes and<br />persistent volumes at the same time. |  |  |
| `image` _[ImageVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#imagevolumesource-v1-core)_ | image represents an OCI object (a container image or artifact) pulled and mounted on the kubelet's host machine.<br />The volume is resolved at pod startup depending on which PullPolicy value is provided:<br />- Always: the kubelet always attempts to pull the reference. Container creation will fail If the pull fails.<br />- Never: the kubelet never pulls the reference and only uses a local image or artifact. Container creation will fail if the reference isn't present.<br />- IfNotPresent: the kubelet pulls if the reference isn't already present on disk. Container creation will fail if the reference isn't present and the pull fails.<br />The volume gets re-resolved if the pod gets deleted and recreated, which means that new remote content will become available on pod recreation.<br />A failure to resolve or pull the image during pod startup will block containers from starting and may add significant latency. Failures will be retried using normal volume backoff and will be reported on the pod reason and message.<br />The types of objects that may be mounted by this volume are defined by the container runtime implementation on a host machine and at minimum must include all valid types supported by the container image field.<br />The OCI object gets mounted in a single directory (spec.containers[*].volumeMounts.mountPath) by merging the manifest layers in the same way as for container images.<br />The volume will be mounted read-only (ro) and non-executable files (noexec).<br />Sub path mounts for containers are not supported (spec.containers[*].volumeMounts.subpath).<br />The field spec.securityContext.fsGroupChangePolicy has no effect on this volume type. |  |  |


#### VolumeSource



VolumeSource is a reduced version of core/v1 VolumeSource.
Represents the source of a volume to mount.
Only one of its members may be specified.



_Appears in:_
- [Volume](#volume)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `hostPath` _[HostPathVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostpathvolumesource-v1-core)_ | hostPath represents a pre-existing file or directory on the host<br />machine that is directly exposed to the container. This is generally<br />used for system agents or other privileged things that are allowed<br />to see the host machine. Most containers will NOT need this.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath |  |  |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#emptydirvolumesource-v1-core)_ | emptyDir represents a temporary directory that shares a pod's lifetime.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir |  |  |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretvolumesource-v1-core)_ | secret represents a secret that should populate this volume.<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#secret |  |  |
| `nfs` _[NFSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#nfsvolumesource-v1-core)_ | nfs represents an NFS mount on the host that shares a pod's lifetime<br />More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs |  |  |
| `iscsi` _[ISCSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#iscsivolumesource-v1-core)_ | iscsi represents an ISCSI Disk resource that is attached to a<br />kubelet's host machine and then exposed to the pod.<br />More info: https://examples.k8s.io/volumes/iscsi/README.md |  |  |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimvolumesource-v1-core)_ | persistentVolumeClaimVolumeSource represents a reference to a<br />PersistentVolumeClaim in the same namespace.<br />More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims |  |  |
| `downwardAPI` _[DownwardAPIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#downwardapivolumesource-v1-core)_ | downwardAPI represents downward API about the pod that should populate this volume |  |  |
| `fc` _[FCVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#fcvolumesource-v1-core)_ | fc represents a Fibre Channel resource that is attached to a kubelet's host machine and then exposed to the pod. |  |  |
| `configMap` _[ConfigMapVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapvolumesource-v1-core)_ | configMap represents a configMap that should populate this volume |  |  |
| `csi` _[CSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#csivolumesource-v1-core)_ | csi (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers (Beta feature). |  |  |
| `ephemeral` _[EphemeralVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#ephemeralvolumesource-v1-core)_ | ephemeral represents a volume that is handled by a cluster storage driver.<br />The volume's lifecycle is tied to the pod that defines it - it will be created before the pod starts,<br />and deleted when the pod is removed.<br />Use this if:<br />a) the volume is only needed while the pod runs,<br />b) features of normal volumes like restoring from snapshot or capacity<br />   tracking are needed,<br />c) the storage driver is specified through a storage class, and<br />d) the storage driver supports dynamic volume provisioning through<br />   a PersistentVolumeClaim (see EphemeralVolumeSource for more<br />   information on the connection between this volume type<br />   and PersistentVolumeClaim).<br />Use PersistentVolumeClaim or one of the vendor-specific<br />APIs for volumes that persist for longer than the lifecycle<br />of an individual pod.<br />Use CSI for light-weight local ephemeral volumes if the CSI driver is meant to<br />be used that way - see the documentation of the driver for<br />more information.<br />A pod can use both types of ephemeral volumes and<br />persistent volumes at the same time. |  |  |
| `image` _[ImageVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#imagevolumesource-v1-core)_ | image represents an OCI object (a container image or artifact) pulled and mounted on the kubelet's host machine.<br />The volume is resolved at pod startup depending on which PullPolicy value is provided:<br />- Always: the kubelet always attempts to pull the reference. Container creation will fail If the pull fails.<br />- Never: the kubelet never pulls the reference and only uses a local image or artifact. Container creation will fail if the reference isn't present.<br />- IfNotPresent: the kubelet pulls if the reference isn't already present on disk. Container creation will fail if the reference isn't present and the pull fails.<br />The volume gets re-resolved if the pod gets deleted and recreated, which means that new remote content will become available on pod recreation.<br />A failure to resolve or pull the image during pod startup will block containers from starting and may add significant latency. Failures will be retried using normal volume backoff and will be reported on the pod reason and message.<br />The types of objects that may be mounted by this volume are defined by the container runtime implementation on a host machine and at minimum must include all valid types supported by the container image field.<br />The OCI object gets mounted in a single directory (spec.containers[*].volumeMounts.mountPath) by merging the manifest layers in the same way as for container images.<br />The volume will be mounted read-only (ro) and non-executable files (noexec).<br />Sub path mounts for containers are not supported (spec.containers[*].volumeMounts.subpath).<br />The field spec.securityContext.fsGroupChangePolicy has no effect on this volume type. |  |  |


#### YQLAgentSpec







_Appears in:_
- [YtsaurusSpec](#ytsaurusspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `image` _string_ | Overrides coreImage for component. |  |  |
| `entrypointWrapper` _string array_ | Specifies wrapper for component container command. |  |  |
| `volumes` _[Volume](#volume) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#volumemount-v1-core) array_ |  |  |  |
| `readinessProbeParams` _[HealthcheckProbeParams](#healthcheckprobeparams)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ | Resources dedicated for component. Capacity is defined by requests, or limits for zero requests. |  |  |
| `instanceCount` _integer_ |  |  |  |
| `minReadyInstanceCount` _integer_ |  |  |  |
| `locations` _[LocationSpec](#locationspec) array_ |  |  |  |
| `volumeClaimTemplates` _[EmbeddedPersistentVolumeClaim](#embeddedpersistentvolumeclaim) array_ |  |  |  |
| `enableAntiAffinity` _boolean_ | Deprecated: use Affinity.PodAntiAffinity instead. |  |  |
| `monitoringPort` _integer_ |  |  |  |
| `metricExporter` _[MetricExporter](#metricexporter)_ |  |  |  |
| `loggers` _[TextLoggerSpec](#textloggerspec) array_ |  |  |  |
| `structuredLoggers` _[StructuredLoggerSpec](#structuredloggerspec) array_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `terminationGracePeriodSeconds` _integer_ | Optional duration in seconds the pod needs to terminate gracefully. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Component config for native RPC bus transport. |  |  |


#### Ytsaurus



Ytsaurus is the Schema for the ytsaurus API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `cluster.ytsaurus.tech/v1` | | |
| `kind` _string_ | `Ytsaurus` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[YtsaurusSpec](#ytsaurusspec)_ |  |  |  |
| `status` _[YtsaurusStatus](#ytsaurusstatus)_ |  |  |  |


#### YtsaurusSpec



YtsaurusSpec defines the desired state of Ytsaurus



_Appears in:_
- [Ytsaurus](#ytsaurus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `coreImage` _string_ |  |  |  |
| `clusterFeatures` _[ClusterFeatures](#clusterfeatures)_ |  |  |  |
| `jobImage` _string_ | Default docker image for user jobs. |  |  |
| `caRootBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted root certificates. Default kind="ConfigMap", key="ca-certificates.crt".<br />Will replace system CA root bundle for all server and job containers. |  |  |
| `caBundle` _[FileObjectReference](#fileobjectreference)_ | Reference to trusted native transport certificates. Default kind="ConfigMap", key="ca.crt".<br />By default will use system CA bundle, which could be set by caRootBundle. |  |  |
| `nativeTransport` _[RPCTransportSpec](#rpctransportspec)_ | Common config for native RPC bus transport. |  |  |
| `ephemeralCluster` _boolean_ | Allow prioritizing performance over data safety. Useful for tests and experiments. | false |  |
| `useIpv6` _boolean_ |  | false |  |
| `useIpv4` _boolean_ |  | false |  |
| `keepSocket` _boolean_ |  |  |  |
| `forceTcp` _boolean_ |  |  |  |
| `useShortNames` _boolean_ | Do not add resource name into names of resources under control.<br />When enabled resource should not share namespace with other Ytsaurus. | true |  |
| `usePorto` _boolean_ |  | false |  |
| `extraPodAnnotations` _object (keys:string, values:string)_ | Deprecated: Despite its name, it adds annotations to all resources. Use podAnnotations instead. |  |  |
| `configOverrides` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |
| `podLabels` _object (keys:string, values:string)_ | Labels for instance pods. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Annotations for instance pods. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | Node selector for instance and init job pods. |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ | Tolerations for instance and init job pods. |  |  |
| `runtimeClassName` _string_ | Runtime class for instance pods. |  |  |
| `hostNetwork` _boolean_ | Use the host's network namespace. |  |  |
| `setHostnameAsFqdn` _boolean_ | SetHostnameAsFQDN indicates whether to set the hostname as FQDN. | true |  |
| `dnsPolicy` _[DNSPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#dnspolicy-v1-core)_ | DNSPolicy defines how DNS will be configured. |  |  |
| `dnsConfig` _[PodDNSConfig](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#poddnsconfig-v1-core)_ | DNSConfig allows customizing the DNS settings. |  |  |
| `uiImage` _string_ |  |  |  |
| `requiresOperatorVersion` _string_ | requiresOperatorVersion is used to lock the YT spec to an Operator version or range.<br />Syntax: https://github.com/Masterminds/semver<br />Example: "~0.1.2" is equivalent to ">= 0.1.2, < 0.2.0". |  |  |
| `adminCredentials` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `oauthService` _[OauthServiceSpec](#oauthservicespec)_ |  |  |  |
| `isManaged` _boolean_ |  | true |  |
| `enableFullUpdate` _boolean_ |  | true |  |
| `updateSelector` _[UpdateSelector](#updateselector)_ | Deprecated: UpdateSelector is going to be removed soon. Please use UpdateSelectors instead. |  | Enum: [ Nothing MasterOnly DataNodesOnly TabletNodesOnly ExecNodesOnly StatelessOnly Everything] <br /> |
| `updatePlan` _[ComponentUpdateSelector](#componentupdateselector) array_ | Experimental: api may change.<br />Controls the components that should be updated during the update process. |  |  |
| `bootstrap` _[BootstrapSpec](#bootstrapspec)_ |  |  |  |
| `discovery` _[DiscoverySpec](#discoveryspec)_ |  |  |  |
| `primaryMasters` _[MastersSpec](#mastersspec)_ |  |  |  |
| `secondaryMasters` _[MastersSpec](#mastersspec) array_ |  |  |  |
| `masterCaches` _[MasterCachesSpec](#mastercachesspec)_ |  |  |  |
| `httpProxies` _[HTTPProxiesSpec](#httpproxiesspec) array_ |  |  | MinItems: 1 <br /> |
| `rpcProxies` _[RPCProxiesSpec](#rpcproxiesspec) array_ |  |  |  |
| `tcpProxies` _[TCPProxiesSpec](#tcpproxiesspec) array_ |  |  |  |
| `kafkaProxies` _[KafkaProxiesSpec](#kafkaproxiesspec) array_ |  |  |  |
| `dataNodes` _[DataNodesSpec](#datanodesspec) array_ |  |  | MinItems: 1 <br /> |
| `execNodes` _[ExecNodesSpec](#execnodesspec) array_ |  |  |  |
| `schedulers` _[SchedulersSpec](#schedulersspec)_ |  |  |  |
| `controllerAgents` _[ControllerAgentsSpec](#controlleragentsspec)_ |  |  |  |
| `tabletNodes` _[TabletNodesSpec](#tabletnodesspec) array_ |  |  |  |
| `strawberry` _[StrawberryControllerSpec](#strawberrycontrollerspec)_ |  |  |  |
| `queryTrackers` _[QueryTrackerSpec](#querytrackerspec)_ |  |  |  |
| `spyt` _[DeprecatedSpytSpec](#deprecatedspytspec)_ |  |  |  |
| `yqlAgents` _[YQLAgentSpec](#yqlagentspec)_ |  |  |  |
| `queueAgents` _[QueueAgentSpec](#queueagentspec)_ |  |  |  |
| `cypressProxies` _[CypressProxiesSpec](#cypressproxiesspec)_ |  |  |  |
| `bundleController` _[BundleControllerSpec](#bundlecontrollerspec)_ |  |  |  |
| `tabletBalancers` _[TabletBalancerSpec](#tabletbalancerspec)_ |  |  |  |
| `ui` _[UISpec](#uispec)_ |  |  |  |


#### YtsaurusStatus



YtsaurusStatus defines the observed state of Ytsaurus



_Appears in:_
- [Ytsaurus](#ytsaurus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[ClusterState](#clusterstate)_ |  | Created |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#condition-v1-meta) array_ |  |  |  |
| `observedGeneration` _integer_ | Reflects resource generation which was used for updating status. |  |  |
| `updateStatus` _[UpdateStatus](#updatestatus)_ |  |  |  |


