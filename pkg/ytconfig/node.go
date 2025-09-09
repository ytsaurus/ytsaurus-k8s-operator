package ytconfig

import (
	"fmt"
	"math"
	"strings"
	"time"

	"go.ytsaurus.tech/yt/go/yson"

	"k8s.io/utils/ptr"

	"k8s.io/apimachinery/pkg/api/resource"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type NodeFlavor string

const (
	NodeFlavorData   NodeFlavor = "data"
	NodeFlavorExec   NodeFlavor = "exec"
	NodeFlavorTablet NodeFlavor = "tablet"
)

type DiskLocation struct {
	// Root directory for the location.
	Path string `yson:"path"`

	// Minimum size the disk partition must have to make this location usable.
	MinDiskSpace int64 `yson:"min_disk_space,omitempty"`

	// Name of the medium corresponding to disk type.
	MediumName string `yson:"medium_name,omitempty"`
}

type ChunkLocation struct {
	// Maximum space chunks are allowed to occupy.
	Quota int64 `yson:"quota,omitempty"`

	IOEngine *IOEngine `yson:"io_config,omitempty"`
}

type CacheLocation struct {
	DiskLocation
	ChunkLocation
}

type StoreLocation struct {
	DiskLocation
	ChunkLocation

	// Consider location full when available space is less than high watermark.
	HighWatermark int64 `yson:"high_watermark,omitempty"`
	// Consider location non-full when available space is more than low watermark.
	LowWatermark int64 `yson:"low_watermark,omitempty"`
	// Stop writes when available space becomes less than disable-writes watermark.
	DisableWritesWatermark int64 `yson:"disable_writes_watermark,omitempty"`
	// Start deleting trash when available space less than trash-cleanup watermark.
	TrashCleanupWatermark int64 `yson:"trash_cleanup_watermark"`
	// Maximum time before permanent deletion.
	MaxTrashTtl *int64 `yson:"max_trash_ttl,omitempty"`
}

type SlotLocation struct {
	DiskLocation

	// Maximum reported total disk capacity.
	DiskQuota *int64 `yson:"disk_quota,omitempty"`
	// Reserve subtracted from disk capacity.
	DiskUsageWatermark *int64 `yson:"disk_usage_watermark,omitempty"`

	// Enforce disk space limits using disk quota.
	EnableDiskQuota *bool `yson:"enable_disk_quota,omitempty"`
}

type ResourceLimits struct {
	TotalMemory      int64    `yson:"total_memory,omitempty"`
	TotalCpu         *float32 `yson:"total_cpu,omitempty"`
	NodeDedicatedCpu *float32 `yson:"node_dedicated_cpu,omitempty"`
}

type DataNode struct {
	StoreLocations []StoreLocation `yson:"store_locations"`
	CacheLocations []CacheLocation `yson:"cache_locations"`
	BlockCache     BlockCache      `yson:"block_cache"`
	BlocksExtCache Cache           `yson:"blocks_ext_cache"`
	ChunkMetaCache Cache           `yson:"chunk_meta_cache"`
	BlockMetaCache Cache           `yson:"block_meta_cache"`
}

type JobEnvironmentType string

const (
	JobEnvironmentTypeSimple JobEnvironmentType = "simple"
	JobEnvironmentTypePorto  JobEnvironmentType = "porto"
	JobEnvironmentTypeCRI    JobEnvironmentType = "cri"
)

type GpuInfoSourceType string

const (
	GpuInfoSourceTypeNvGpuManager GpuInfoSourceType = "nv_gpu_manager"
	GpuInfoSourceTypeNvidiaSmi    GpuInfoSourceType = "nvidia_smi"
	GpuInfoSourceTypeGpuAgent     GpuInfoSourceType = "gpu_agent"
)

type CriExecutor struct {
	RetryingChannel

	RuntimeEndpoint string        `yson:"runtime_endpoint,omitempty"`
	ImageEndpoint   string        `yson:"image_endpoint,omitempty"`
	Namespace       string        `yson:"namespace"`
	BaseCgroup      string        `yson:"base_cgroup"`
	RuntimeHandler  string        `yson:"runtime_handler,omitempty"`
	CpuPeriod       yson.Duration `yson:"cpu_period,omitempty"`
}

type CriImageCache struct {
	Capacity *int64 `yson:"capacity,omitempty"`

	ManagedPrefixes   []string `yson:"managed_prefixes,omitempty"`
	UnmanagedPrefixes []string `yson:"managed_prefixes,omitempty"`
	PinnedImages      []string `yson:"pinned_image,omitempty"`

	AlwaysPullLatest                *bool         `yson:"always_pull_latest,omitempty"`
	PullPeriod                      yson.Duration `yson:"pullPeriod,omitempty"`
	ImageSizeEstimation             *int64        `yson:"image_size_estimation,omitempty"`
	ImageCompressionRatioEstimation *float32      `yson:"image_compression_ratio_estimation,omitempty"`
	YoungerSizeFraction             *float32      `yson:"younger_size_fraction,omitempty"`
}

type CriJobEnvironment struct {
	CriExecutor          *CriExecutor   `yson:"cri_executor,omitempty"`
	CriImageCache        *CriImageCache `yson:"cri_image_cache,omitempty"`
	JobProxyImage        string         `yson:"job_proxy_image,omitempty"`
	JobProxyBindMounts   []BindMount    `yson:"job_proxy_bind_mounts,omitempty"`
	UseJobProxyFromImage *bool          `yson:"use_job_proxy_from_image,omitempty"`
}

type JobEnvironment struct {
	Type     JobEnvironmentType `yson:"type,omitempty"`
	StartUID int                `yson:"start_uid,omitempty"`

	// FIXME(khlebnikov): Add "inline" tag into yson or remove polymorphism in config.
	CriJobEnvironment
}

type SlotManager struct {
	Locations      []SlotLocation `yson:"locations"`
	JobEnvironment JobEnvironment `yson:"job_environment"`

	DoNotSetUserId      *bool `yson:"do_not_set_user_id,omitempty"`
	EnableTmpfs         *bool `yson:"enable_tmpfs,omitempty"`
	DetachedTmpfsUmount *bool `yson:"detached_tmpfs_umount,omitempty"`
}

type JobResourceLimits struct {
	UserSlots *int `yson:"user_slots,omitempty"`
}

type GpuAgentSpec struct {
	Address     string `yson:"address,omitempty"`
	ServiceName string `yson:"service_name,omitempty"`
}

type GpuInfoSource struct {
	Type GpuInfoSourceType `yson:"type"`
	GpuAgentSpec
}

const GpuAgentPort = 23105

type GpuManager struct {
	GpuInfoSource GpuInfoSource `yson:"gpu_info_source"`
}

type JobController struct {
	ResourceLimitsLegacy *JobResourceLimits `yson:"resource_limits,omitempty"`
	GpuManagerLegacy     *GpuManager        `yson:"gpu_manager,omitempty"`
}

type JobResourceManager struct {
	ResourceLimits JobResourceLimits `yson:"resource_limits"`
}

type EnvironmentVariable struct {
	Name                string  `yson:"name"`
	Value               *string `yson:"value,omitempty"`
	FileName            *string `yson:"file_name,omitempty"`
	EnvironmentVariable *string `yson:"environment_variable,omitempty"`
	Export              *bool   `yson:"export,omitempty"`
}

type JobProxy struct {
	JobProxyAuthenticationManager  Auth                  `yson:"job_proxy_authentication_manager"`
	JobProxyLogging                JobProxyLogging       `yson:"job_proxy_logging"`
	EnvironmentVariables           []EnvironmentVariable `yson:"environment_variables,omitempty"`
	ForwardAllEnvironmentVariables *bool                 `yson:"forward_all_environment_variables,omitempty"`
	ClusterConnection              *ClusterConnection    `yson:"cluster_connection,omitempty"`
	SupervisorConnection           *BusClient            `yson:"supervisor_connection,omitempty"`
}

type LogDump struct {
	BufferSize    int64  `yson:"buffer_size"`
	LogWriterName string `yson:"log_writer_name"`
}

type JobProxyLogManager struct {
	Directory                     string        `yson:"directory"`
	ShardingKeyLength             int           `yson:"sharding_key_length"`
	LogsStoragePeriod             yson.Duration `yson:"logs_storage_period"`
	DirectoryTraversalConcurrency int           `yson:"directory_traversal_concurrency"`
	LogDump                       LogDump       `yson:"log_dump"`
}

type ExecNode struct {
	SlotManager   SlotManager   `yson:"slot_manager"`
	GpuManager    GpuManager    `yson:"gpu_manager"`
	JobController JobController `yson:"job_controller"`
	JobProxy      JobProxy      `yson:"job_proxy"`

	JobProxyAuthenticationManagerLegacy  *Auth    `yson:"job_proxy_authentication_manager,omitempty"`
	JobProxyLoggingLegacy                *Logging `yson:"job_proxy_logging,omitempty"`
	DoNotSetUserIdLegacy                 *bool    `yson:"do_not_set_user_id,omitempty"`
	ForwardAllEnvironmentVariablesLegacy *bool    `yson:"forward_all_environment_variables,omitempty"`

	// NOTE: Non-legacy "use_artifact_binds" moved into dynamic config.
	UseArtifactBindsLegacy *bool              `yson:"use_artifact_binds,omitempty"`
	JobProxyLogManager     JobProxyLogManager `yson:"job_proxy_log_manager"`
}

type Cache struct {
	Capacity int64 `yson:"capacity"`
}

type BlockCache struct {
	Compressed   Cache `yson:"compressed_data"`
	Uncompressed Cache `yson:"uncompressed_data"`
}

type TabletNode struct {
	VersionedChunkMetaCache Cache `yson:"versioned_chunk_meta_cache"`
}

type NodeServer struct {
	CommonServer
	Flavors        []NodeFlavor   `yson:"flavors"`
	ResourceLimits ResourceLimits `yson:"resource_limits,omitempty"`
	Tags           []string       `yson:"tags,omitempty"`
	Rack           string         `yson:"rack,omitempty"`
	SkynetHttpPort int32          `yson:"skynet_http_port"`
}

type DataNodeServer struct {
	NodeServer
	DataNode DataNode `yson:"data_node"`
}

type ExecNodeServer struct {
	NodeServer
	JobResourceManager   JobResourceManager `yson:"job_resource_manager"`
	ExecNode             ExecNode           `yson:"exec_node"`
	DataNode             DataNode           `yson:"data_node"`
	TabletNode           TabletNode         `yson:"tablet_node"`
	CachingObjectService Cache              `yson:"caching_object_service"`
}

type TabletNodeServer struct {
	NodeServer
	// TabletNode `yson:"tablet_node"`
	CachingObjectService Cache `yson:"caching_object_service"`
}

func findVolumeMountForPath(locationPath string, spec ytv1.InstanceSpec) *corev1.VolumeMount {
	for _, mount := range spec.VolumeMounts {
		if strings.HasPrefix(locationPath, mount.MountPath) {
			return &mount
		}
	}
	return nil
}

func findVolumeClaimTemplate(volumeName string, spec ytv1.InstanceSpec) *ytv1.EmbeddedPersistentVolumeClaim {
	for _, claim := range spec.VolumeClaimTemplates {
		if claim.Name == volumeName {
			return &claim
		}
	}
	return nil
}

func findVolume(volumeName string, spec ytv1.InstanceSpec) *corev1.Volume {
	for _, volume := range spec.Volumes {
		if volume.Name == volumeName {
			return &volume
		}
	}
	return nil
}

func findQuotaForLocation(location ytv1.LocationSpec, spec ytv1.InstanceSpec) *int64 {
	if quota := location.Quota; quota != nil {
		return ptr.To(quota.Value())
	}

	mount := findVolumeMountForPath(location.Path, spec)
	if mount == nil {
		return nil
	}

	if claim := findVolumeClaimTemplate(mount.Name, spec); claim != nil {
		storage := claim.Spec.Resources.Requests.Storage()
		if storage != nil && !storage.IsZero() {
			return ptr.To(storage.Value())
		}
	} else if volume := findVolume(mount.Name, spec); volume != nil {
		if volume.EmptyDir != nil && volume.EmptyDir.SizeLimit != nil {
			return ptr.To(volume.EmptyDir.SizeLimit.Value())
		}
	}

	return nil
}

func fillClusterNodeServerCarcass(n *NodeServer, flavor NodeFlavor, spec ytv1.ClusterNodesSpec, is *ytv1.InstanceSpec) {
	switch flavor {
	case NodeFlavorData:
		n.RPCPort = consts.DataNodeRPCPort
		n.SkynetHttpPort = consts.DataNodeSkynetPort
		n.MonitoringPort = consts.DataNodeMonitoringPort
	case NodeFlavorExec:
		n.RPCPort = consts.ExecNodeRPCPort
		n.SkynetHttpPort = consts.ExecNodeSkynetPort
		n.MonitoringPort = consts.ExecNodeMonitoringPort
	case NodeFlavorTablet:
		n.RPCPort = consts.TabletNodeRPCPort
		n.SkynetHttpPort = consts.TabletNodeSkynetPort
		n.MonitoringPort = consts.TabletNodeMonitoringPort
	}

	if is.MonitoringPort != nil {
		n.MonitoringPort = *is.MonitoringPort
	}

	n.Flavors = []NodeFlavor{flavor}
	n.Tags = spec.Tags
	n.Rack = spec.Rack
}

func getDataNodeResourceLimits(spec *ytv1.DataNodesSpec) ResourceLimits {
	var resourceLimits ResourceLimits

	var cpu float32 = 0
	resourceLimits.NodeDedicatedCpu = &cpu
	resourceLimits.TotalCpu = &cpu

	memoryRequest := spec.Resources.Requests.Memory()
	memoryLimit := spec.Resources.Limits.Memory()
	if memoryRequest != nil && !memoryRequest.IsZero() {
		resourceLimits.TotalMemory = memoryRequest.Value()
	} else if memoryLimit != nil && !memoryLimit.IsZero() {
		resourceLimits.TotalMemory = memoryLimit.Value()
	}

	return resourceLimits
}

func getDataNodeLogging(spec *ytv1.DataNodesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		consts.GetServiceKebabCase(consts.DataNodeType),
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getDataNodeServerCarcass(spec *ytv1.DataNodesSpec) (DataNodeServer, error) {
	var c DataNodeServer
	fillClusterNodeServerCarcass(&c.NodeServer, NodeFlavorData, spec.ClusterNodesSpec, &spec.InstanceSpec)

	c.ResourceLimits = getDataNodeResourceLimits(spec)

	for _, location := range ytv1.FindAllLocations(spec.Locations, ytv1.LocationTypeChunkStore) {
		storeLocation := StoreLocation{
			DiskLocation: DiskLocation{
				Path:       location.Path,
				MediumName: location.Medium,
			},
		}

		if quota := findQuotaForLocation(location, spec.InstanceSpec); quota != nil {
			storeLocation.Quota = *quota

			if location.LowWatermark != nil {
				storeLocation.LowWatermark = location.LowWatermark.Value()
				storeLocation.MinDiskSpace = storeLocation.LowWatermark
			} else {
				gb := float64(1024 * 1024 * 1024)
				storeLocation.LowWatermark = int64(math.Min(0.1*float64(storeLocation.Quota), float64(25)*gb))
			}

			// These are just simple heuristics.
			storeLocation.HighWatermark = storeLocation.LowWatermark / 2
			storeLocation.DisableWritesWatermark = storeLocation.HighWatermark / 2
			storeLocation.TrashCleanupWatermark = storeLocation.LowWatermark
			storeLocation.MaxTrashTtl = location.MaxTrashMilliseconds
		}
		c.DataNode.StoreLocations = append(c.DataNode.StoreLocations, storeLocation)
	}

	if len(c.DataNode.StoreLocations) == 0 {
		return c, fmt.Errorf("error creating data node config: no storage locations provided")
	}

	c.Logging = getDataNodeLogging(spec)

	return c, nil
}

func getResourceQuantity(resources *corev1.ResourceRequirements, name corev1.ResourceName) resource.Quantity {
	if request, ok := resources.Requests[name]; ok && !request.IsZero() {
		return request
	}
	if limit, ok := resources.Limits[name]; ok && !limit.IsZero() {
		return limit
	}
	return resource.Quantity{}
}

func getExecNodeResourceLimits(spec *ytv1.ExecNodesSpec) ResourceLimits {
	var resourceLimits ResourceLimits

	nodeMemory := getResourceQuantity(&spec.Resources, corev1.ResourceMemory)
	nodeCpu := getResourceQuantity(&spec.Resources, corev1.ResourceCPU)

	totalMemory := nodeMemory
	totalCpu := nodeCpu

	if spec.JobResources != nil {
		totalMemory.Add(getResourceQuantity(spec.JobResources, corev1.ResourceMemory))
		totalCpu.Add(getResourceQuantity(spec.JobResources, corev1.ResourceCPU))

		resourceLimits.NodeDedicatedCpu = ptr.To(float32(nodeCpu.AsApproximateFloat64()))
	} else {
		// TODO(khlebnikov): Add better defaults.
		resourceLimits.NodeDedicatedCpu = ptr.To(float32(0))
	}

	resourceLimits.TotalMemory = totalMemory.Value()
	if !totalCpu.IsZero() {
		resourceLimits.TotalCpu = ptr.To(float32(totalCpu.AsApproximateFloat64()))
	}

	return resourceLimits
}

func getExecNodeLogging(spec *ytv1.ExecNodesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		consts.GetServiceKebabCase(consts.ExecNodeType),
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func fillJobEnvironment(execNode *ExecNode, spec *ytv1.ExecNodesSpec, commonSpec *ytv1.CommonSpec) error {
	envSpec := spec.JobEnvironment
	jobEnv := &execNode.SlotManager.JobEnvironment

	jobEnv.StartUID = consts.StartUID

	if envSpec != nil && envSpec.CRI != nil {
		cri := NewCRIConfigGenerator(spec)

		jobEnv.Type = JobEnvironmentTypeCRI

		if jobImage := commonSpec.JobImage; jobImage != nil {
			jobEnv.JobProxyImage = *jobImage
		} else {
			jobEnv.JobProxyImage = ptr.Deref(spec.Image, commonSpec.CoreImage)
		}

		jobEnv.UseJobProxyFromImage = ptr.To(false)

		endpoint := "unix://" + cri.GetSocketPath()

		jobEnv.CriExecutor = &CriExecutor{
			RuntimeEndpoint: endpoint,
			ImageEndpoint:   endpoint,
			Namespace:       ptr.Deref(envSpec.CRI.CRINamespace, consts.CRINamespace),
			BaseCgroup:      ptr.Deref(envSpec.CRI.BaseCgroup, consts.CRIBaseCgroup),
		}

		if timeout := envSpec.CRI.APIRetryTimeoutSeconds; timeout != nil {
			jobEnv.CriExecutor.RetryingChannel = RetryingChannel{
				RetryBackoffTime: yson.Duration(time.Second),
				RetryAttempts:    *timeout,
				RetryTimeout:     yson.Duration(time.Duration(*timeout) * time.Second),
			}
		}

		if location := ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeImageCache); location != nil {
			jobEnv.CriImageCache = &CriImageCache{
				Capacity:            findQuotaForLocation(*location, spec.InstanceSpec),
				ImageSizeEstimation: envSpec.CRI.ImageSizeEstimation,
				AlwaysPullLatest:    envSpec.CRI.AlwaysPullLatestImage,
			}
			if ratio := envSpec.CRI.ImageCompressionRatioEstimation; ratio != nil {
				jobEnv.CriImageCache.ImageCompressionRatioEstimation = ptr.To(float32(*ratio))
			}
			if period := envSpec.CRI.ImagePullPeriodSeconds; period != nil {
				jobEnv.CriImageCache.PullPeriod = yson.Duration(time.Duration(*period) * time.Second)
			}
		}

		// NOTE: Default was "false", now it's "true" and option was moved into dynamic config.
		execNode.UseArtifactBindsLegacy = ptr.To(ptr.Deref(envSpec.UseArtifactBinds, true))
		if !*execNode.UseArtifactBindsLegacy {
			// Bind mount chunk cache into job containers if artifact are passed via symlinks.
			for _, location := range ytv1.FindAllLocations(spec.Locations, ytv1.LocationTypeChunkCache) {
				jobEnv.JobProxyBindMounts = append(jobEnv.JobProxyBindMounts, BindMount{
					InternalPath: location.Path,
					ExternalPath: location.Path,
					ReadOnly:     true,
				})
			}
		}

		// FIXME(khlebnikov): For now running jobs as non-root is more likely broken.
		execNode.SlotManager.DoNotSetUserId = ptr.To(ptr.Deref(envSpec.DoNotSetUserId, true))

		// Enable tmpfs if exec node can mount and propagate into job container.
		execNode.SlotManager.EnableTmpfs = ptr.To(func() bool {
			if !spec.Privileged {
				return false
			}
			if !ptr.Deref(envSpec.Isolated, true) {
				return true
			}
			for _, location := range ytv1.FindAllLocations(spec.Locations, ytv1.LocationTypeSlots) {
				mount := findVolumeMountForPath(location.Path, spec.InstanceSpec)
				if mount == nil || mount.MountPropagation == nil || *mount.MountPropagation != corev1.MountPropagationBidirectional {
					return false
				}
			}
			return true
		}())

		// Forward environment variables set in docker image from job proxy to user job process.
		execNode.JobProxy.ForwardAllEnvironmentVariables = ptr.To(true)
	} else if commonSpec.UsePorto {
		jobEnv.Type = JobEnvironmentTypePorto
		execNode.SlotManager.EnableTmpfs = ptr.To(true)
		// TODO(psushin): volume locations, root fs binds, etc.
	} else {
		jobEnv.Type = JobEnvironmentTypeSimple
		execNode.SlotManager.EnableTmpfs = ptr.To(spec.Privileged)
	}

	return nil
}

func getExecNodeServerCarcass(spec *ytv1.ExecNodesSpec, commonSpec *ytv1.CommonSpec) (ExecNodeServer, error) {
	var c ExecNodeServer
	fillClusterNodeServerCarcass(&c.NodeServer, NodeFlavorExec, spec.ClusterNodesSpec, &spec.InstanceSpec)

	c.ResourceLimits = getExecNodeResourceLimits(spec)

	for _, location := range ytv1.FindAllLocations(spec.Locations, ytv1.LocationTypeChunkCache) {
		cacheLocation := CacheLocation{
			DiskLocation: DiskLocation{
				Path:       location.Path,
				MediumName: location.Medium,
			},
		}

		if location.LowWatermark != nil {
			cacheLocation.MinDiskSpace = location.LowWatermark.Value()
		}

		if quota := findQuotaForLocation(location, spec.InstanceSpec); quota != nil {
			cacheLocation.Quota = *quota
		}

		c.DataNode.CacheLocations = append(c.DataNode.CacheLocations, cacheLocation)
	}

	if len(c.DataNode.CacheLocations) == 0 {
		return c, fmt.Errorf("error creating exec node config: no cache locations provided")
	}

	for _, location := range ytv1.FindAllLocations(spec.Locations, ytv1.LocationTypeSlots) {
		slotLocation := SlotLocation{
			DiskLocation: DiskLocation{
				Path:       location.Path,
				MediumName: location.Medium,
			},
		}

		if location.LowWatermark != nil {
			slotLocation.MinDiskSpace = location.LowWatermark.Value()
		}

		if quota := findQuotaForLocation(location, spec.InstanceSpec); quota != nil {
			slotLocation.DiskQuota = quota

			// These are just simple heuristics.
			slotLocation.DiskUsageWatermark = ptr.To(min(*quota/10, consts.MaxSlotLocationReserve))
		} else {
			// Do not reserve anything if total disk size is unknown.
			slotLocation.DiskUsageWatermark = ptr.To(int64(0))
		}

		// Disk quota isn't generally available for k8s volumes yet.
		slotLocation.EnableDiskQuota = ptr.To(false)

		c.ExecNode.SlotManager.Locations = append(c.ExecNode.SlotManager.Locations, slotLocation)
	}

	if len(c.ExecNode.SlotManager.Locations) == 0 {
		return c, fmt.Errorf("error creating exec node config: no slot locations provided")
	}

	if spec.JobEnvironment != nil && spec.JobEnvironment.UserSlots != nil {
		c.JobResourceManager.ResourceLimits.UserSlots = ptr.To(*spec.JobEnvironment.UserSlots)
	} else {
		// Dummy heuristic.
		jobCPU := ptr.Deref(c.ResourceLimits.TotalCpu, 0) - ptr.Deref(c.ResourceLimits.NodeDedicatedCpu, 0)
		c.JobResourceManager.ResourceLimits.UserSlots = ptr.To(int(5 * max(1, jobCPU)))
	}

	if err := fillJobEnvironment(&c.ExecNode, spec, commonSpec); err != nil {
		return c, err
	}

	gpuInfoSource := &c.ExecNode.GpuManager.GpuInfoSource
	if spec.JobEnvironment != nil && spec.JobEnvironment.Runtime != nil && spec.JobEnvironment.Runtime.Nvidia != nil {
		gpuInfoSource.Type = GpuInfoSourceTypeGpuAgent
		gpuInfoSource.Address = fmt.Sprintf("localhost:%d", GpuAgentPort)
		gpuInfoSource.ServiceName = "NYT.NGpuAgent.NProto.GpuAgent"
	} else {
		gpuInfoSource.Type = GpuInfoSourceTypeNvidiaSmi
	}

	c.Logging = getExecNodeLogging(spec)

	jobProxyLoggingBuilder := newJobProxyLoggingBuilder()
	if len(spec.JobProxyLoggers) > 0 {
		for _, loggerSpec := range spec.JobProxyLoggers {
			jobProxyLoggingBuilder.addLogger(loggerSpec)
		}
	} else {
		for _, defaultLoggerSpec := range []ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()} {
			jobProxyLoggingBuilder.addLogger(defaultLoggerSpec)
		}
	}
	jobProxyLoggingBuilder.logging.FlushPeriod = 3000
	jobProxyLogging := jobProxyLoggingBuilder.logging
	c.ExecNode.JobProxy.JobProxyLogging = JobProxyLogging{
		Logging:            jobProxyLogging,
		LogManagerTemplate: jobProxyLogging,
		Mode:               "simple",
	}

	c.ExecNode.JobProxy.JobProxyAuthenticationManager.RequireAuthentication = true
	c.ExecNode.JobProxy.JobProxyAuthenticationManager.CypressTokenAuthenticator.Secure = true

	// Configure JobProxyLogManager
	c.ExecNode.JobProxyLogManager.Directory = ChooseJobProxyLoggingPath(&spec.InstanceSpec)
	c.ExecNode.JobProxyLogManager.ShardingKeyLength = 2
	c.ExecNode.JobProxyLogManager.LogsStoragePeriod = yson.Duration(7 * 24 * time.Hour) // 1 week
	c.ExecNode.JobProxyLogManager.DirectoryTraversalConcurrency = 4
	c.ExecNode.JobProxyLogManager.LogDump = LogDump{
		BufferSize:    1024 * 1024, // 1MB
		LogWriterName: "debug",
	}

	// TODO(khlebnikov): Drop legacy fields depending on ytsaurus version.
	c.ExecNode.JobController.ResourceLimitsLegacy = &c.JobResourceManager.ResourceLimits
	c.ExecNode.JobController.GpuManagerLegacy = &c.ExecNode.GpuManager
	c.ExecNode.JobProxyLoggingLegacy = &jobProxyLogging
	c.ExecNode.JobProxyAuthenticationManagerLegacy = &c.ExecNode.JobProxy.JobProxyAuthenticationManager
	c.ExecNode.DoNotSetUserIdLegacy = c.ExecNode.SlotManager.DoNotSetUserId
	c.ExecNode.ForwardAllEnvironmentVariablesLegacy = c.ExecNode.JobProxy.ForwardAllEnvironmentVariables

	return c, nil
}

func getTabletNodeLogging(spec *ytv1.TabletNodesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		consts.GetServiceKebabCase(consts.TabletNodeType),
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getTabletNodeServerCarcass(spec *ytv1.TabletNodesSpec) (TabletNodeServer, error) {
	var c TabletNodeServer
	fillClusterNodeServerCarcass(&c.NodeServer, NodeFlavorTablet, spec.ClusterNodesSpec, &spec.InstanceSpec)

	var cpu float32 = 0
	c.ResourceLimits.NodeDedicatedCpu = &cpu
	c.ResourceLimits.TotalCpu = &cpu

	memoryRequest := spec.Resources.Requests.Memory()
	memoryLimit := spec.Resources.Limits.Memory()
	if memoryRequest != nil && !memoryRequest.IsZero() {
		c.ResourceLimits.TotalMemory = memoryRequest.Value()
	} else if memoryLimit != nil && !memoryLimit.IsZero() {
		c.ResourceLimits.TotalMemory = memoryLimit.Value()
	}

	c.Logging = getTabletNodeLogging(spec)

	return c, nil
}
