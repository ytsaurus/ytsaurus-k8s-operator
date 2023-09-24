package ytconfig

import (
	"fmt"
	ptr "k8s.io/utils/pointer"
	"math"
	"strings"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	v1 "k8s.io/api/core/v1"
)

type NodeFlavor string

const (
	NodeFlavorData   NodeFlavor = "data"
	NodeFlavorExec   NodeFlavor = "exec"
	NodeFlavorTablet NodeFlavor = "tablet"
)

type StoreLocation struct {
	Path                   string `yson:"path"`
	MediumName             string `yson:"medium_name"`
	Quota                  int64  `yson:"quota"`
	HighWatermark          int64  `yson:"high_watermark"`
	LowWatermark           int64  `yson:"low_watermark"`
	DisableWritesWatermark int64  `yson:"disable_writes_watermark"`
}

type ResourceLimits struct {
	TotalMemory      int64    `yson:"total_memory,omitempty"`
	TotalCpu         *float32 `yson:"total_cpu,omitempty"`
	NodeDedicatedCpu *float32 `yson:"node_dedicated_cpu,omitempty"`
}

type DiskLocation struct {
	Path string `yson:"path"`
}

type SlotLocation struct {
	Path               string `yson:"path"`
	DiskQuota          *int64 `yson:"disk_quota"`
	DiskUsageWatermark int64  `yson:"disk_usage_watermark"`
	MediumName         string `yson:"medium_name"`
}

type DataNode struct {
	StoreLocations []StoreLocation `yson:"store_locations"`
	CacheLocations []DiskLocation  `yson:"cache_locations"`
	BlockCache     BlockCache      `yson:"block_cache"`
	BlocksExtCache Cache           `yson:"blocks_ext_cache"`
	ChunkMetaCache Cache           `yson:"chunk_meta_cache"`
	BlockMetaCache Cache           `yson:"block_meta_cache"`
}

type JobEnvironmentType string

const (
	JobEnvironmentTypeSimple JobEnvironmentType = "simple"
	JobEnvironmentTypePorto  JobEnvironmentType = "porto"
)

type JobEnvironment struct {
	Type     JobEnvironmentType `yson:"type,omitempty"`
	StartUID int                `yson:"start_uid,omitempty"`
}

type SlotManager struct {
	Locations      []SlotLocation `yson:"locations"`
	JobEnvironment JobEnvironment `yson:"job_environment"`
}

type JobResourceLimits struct {
	UserSlots int `yson:"user_slots"`
}

type JobController struct {
	ResourceLimits JobResourceLimits `yson:"resource_limits"`
}

type ExecAgent struct {
	SlotManager   SlotManager   `yson:"slot_manager"`
	JobController JobController `yson:"job_controller"`
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
	ResourceLimits ResourceLimits `yson:"resource_limits, omitempty"`
	Tags           []string       `yson:"tags, omitempty"`
	Rack           string         `yson:"rack, omitempty"`
	SkynetHttpPort int32          `yson:"skynet_http_port"`
}

type DataNodeServer struct {
	NodeServer
	DataNode DataNode `yson:"data_node"`
}

type ExecNodeServer struct {
	NodeServer
	ExecAgent            ExecAgent  `yson:"exec_agent"`
	DataNode             DataNode   `yson:"data_node"`
	TabletNode           TabletNode `yson:"tablet_node"`
	CachingObjectService Cache      `yson:"caching_object_service"`
}

type TabletNodeServer struct {
	NodeServer
	// TabletNode TabletNode `yson:"tablet_node"`
	CachingObjectService Cache `yson:"caching_object_service"`
}

func findVolumeMountForPath(locationPath string, spec ytv1.InstanceSpec) *v1.VolumeMount {
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

func findVolume(volumeName string, spec ytv1.InstanceSpec) *v1.Volume {
	for _, volume := range spec.Volumes {
		if volume.Name == volumeName {
			return &volume
		}
	}
	return nil
}

func findQuotaForPath(locationPath string, spec ytv1.InstanceSpec) *int64 {
	mount := findVolumeMountForPath(locationPath, spec)
	if mount == nil {
		return nil
	}

	if claim := findVolumeClaimTemplate(mount.Name, spec); claim != nil {
		storage := claim.Spec.Resources.Requests.Storage()
		if storage != nil {
			value := storage.Value()
			return &value
		} else {
			return nil
		}
	}

	if volume := findVolume(mount.Name, spec); volume != nil {
		if volume.EmptyDir != nil && volume.EmptyDir.SizeLimit != nil {
			value := volume.EmptyDir.SizeLimit.Value()
			return &value
		}
	}

	return nil
}

func fillClusterNodeServerCarcass(n *NodeServer, spec ytv1.ClusterNodesSpec, flavor NodeFlavor) {
	switch flavor {
	case NodeFlavorData:
		n.RPCPort = consts.DataNodeRPCPort
		n.MonitoringPort = consts.DataNodeMonitoringPort
		n.SkynetHttpPort = consts.DataNodeSkynetPort
	case NodeFlavorExec:
		n.RPCPort = consts.ExecNodeRPCPort
		n.MonitoringPort = consts.ExecNodeMonitoringPort
		n.SkynetHttpPort = consts.ExecNodeSkynetPort
	case NodeFlavorTablet:
		n.RPCPort = consts.TabletNodeRPCPort
		n.MonitoringPort = consts.TabletNodeMonitoringPort
		n.SkynetHttpPort = consts.TabletNodeSkynetPort
	}

	n.Flavors = []NodeFlavor{flavor}
	n.Tags = spec.Tags
	n.Rack = spec.Rack
}

func getDataNodeResourceLimits(spec ytv1.DataNodesSpec) ResourceLimits {
	var resourceLimits ResourceLimits

	var cpu float32 = 0
	resourceLimits.NodeDedicatedCpu = &cpu
	resourceLimits.TotalCpu = &cpu

	memory := spec.Resources.Requests.Memory()
	if memory != nil {
		resourceLimits.TotalMemory = memory.Value()
	}
	return resourceLimits
}

func getDataNodeLogging(spec ytv1.DataNodesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"data-node",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getDataNodeServerCarcass(spec ytv1.DataNodesSpec) (DataNodeServer, error) {
	var c DataNodeServer
	fillClusterNodeServerCarcass(&c.NodeServer, spec.ClusterNodesSpec, NodeFlavorData)

	c.ResourceLimits = getDataNodeResourceLimits(spec)

	for _, location := range ytv1.FindAllLocations(spec.Locations, ytv1.LocationTypeChunkStore) {
		quota := findQuotaForPath(location.Path, spec.InstanceSpec)
		storeLocation := StoreLocation{
			MediumName: location.Medium,
			Path:       location.Path,
		}
		if quota != nil {
			storeLocation.Quota = *quota

			// These are just simple heuristics.
			gb := float64(1024 * 1024 * 1024)
			storeLocation.LowWatermark = int64(math.Min(0.1*float64(storeLocation.Quota), float64(5)*gb))
			storeLocation.HighWatermark = storeLocation.LowWatermark / 2
			storeLocation.DisableWritesWatermark = storeLocation.HighWatermark / 2
		}
		c.DataNode.StoreLocations = append(c.DataNode.StoreLocations, storeLocation)
	}

	if len(c.DataNode.StoreLocations) == 0 {
		return c, fmt.Errorf("error creating data node config: no storage locations provided")
	}

	c.Logging = getDataNodeLogging(spec)

	return c, nil
}

func getExecNodeResourceLimits(spec ytv1.ExecNodesSpec) ResourceLimits {
	var resourceLimits ResourceLimits
	resourceLimits.NodeDedicatedCpu = ptr.Float32Ptr(0)
	cpuLimit := spec.Resources.Limits.Cpu()
	cpuRequest := spec.Resources.Requests.Cpu()

	if cpuLimit != nil {
		value := float32(cpuLimit.Value())
		resourceLimits.TotalCpu = &value
	} else if cpuRequest != nil {
		value := float32(cpuRequest.Value())
		resourceLimits.TotalCpu = &value
	}

	memoryRequest := spec.Resources.Requests.Memory()
	memoryLimit := spec.Resources.Limits.Memory()
	if memoryLimit != nil {
		resourceLimits.TotalMemory = memoryLimit.Value()
	} else if memoryRequest != nil {
		resourceLimits.TotalMemory = memoryRequest.Value()
	}

	return resourceLimits
}

func getExecNodeLogging(spec ytv1.ExecNodesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"exec-node",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getExecNodeServerCarcass(spec ytv1.ExecNodesSpec, usePorto bool) (ExecNodeServer, error) {
	var c ExecNodeServer
	fillClusterNodeServerCarcass(&c.NodeServer, spec.ClusterNodesSpec, NodeFlavorExec)

	c.ResourceLimits = getExecNodeResourceLimits(spec)

	for _, location := range ytv1.FindAllLocations(spec.Locations, ytv1.LocationTypeChunkCache) {
		c.DataNode.CacheLocations = append(c.DataNode.CacheLocations, DiskLocation{
			Path: location.Path,
		})
	}

	if len(c.DataNode.CacheLocations) == 0 {
		return c, fmt.Errorf("error creating exec node config: no cache locations provided")
	}

	for _, location := range ytv1.FindAllLocations(spec.Locations, ytv1.LocationTypeSlots) {
		slotLocation := SlotLocation{
			Path:       location.Path,
			MediumName: location.Medium,
		}
		quota := findQuotaForPath(location.Path, spec.InstanceSpec)
		if quota != nil {
			slotLocation.DiskQuota = quota

			// These are just simple heuristics.
			gb := float64(1024 * 1024 * 1024)
			slotLocation.DiskUsageWatermark = int64(math.Min(0.1*float64(*quota), float64(10)*gb))
		}
		c.ExecAgent.SlotManager.Locations = append(c.ExecAgent.SlotManager.Locations, slotLocation)
	}

	if len(c.ExecAgent.SlotManager.Locations) == 0 {
		return c, fmt.Errorf("error creating exec node config: no slot locations provided")
	}

	if c.ResourceLimits.TotalCpu != nil {
		// Dummy heuristic.
		c.ExecAgent.JobController.ResourceLimits.UserSlots = int(5 * *c.ResourceLimits.TotalCpu)
	}

	c.ExecAgent.SlotManager.JobEnvironment.StartUID = consts.StartUID
	if usePorto {
		c.ExecAgent.SlotManager.JobEnvironment.Type = JobEnvironmentTypePorto
		// ToDo(psushin): volume locations, root fs binds, etc.
	} else {
		c.ExecAgent.SlotManager.JobEnvironment.Type = JobEnvironmentTypeSimple
	}

	c.Logging = getExecNodeLogging(spec)

	return c, nil
}

func getTabletNodeLogging(spec ytv1.TabletNodesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"tablet-node",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getTabletNodeServerCarcass(spec ytv1.TabletNodesSpec) (TabletNodeServer, error) {
	var c TabletNodeServer
	fillClusterNodeServerCarcass(&c.NodeServer, spec.ClusterNodesSpec, NodeFlavorTablet)

	var cpu float32 = 0
	c.ResourceLimits.NodeDedicatedCpu = &cpu
	c.ResourceLimits.TotalCpu = &cpu

	memory := spec.Resources.Requests.Memory()
	if memory != nil {
		c.ResourceLimits.TotalMemory = memory.Value()
	}

	c.Logging = getTabletNodeLogging(spec)

	return c, nil
}
