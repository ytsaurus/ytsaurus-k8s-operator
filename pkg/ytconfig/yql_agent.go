package ytconfig

import (
	"k8s.io/utils/ptr"

	"go.ytsaurus.tech/yt/go/yson"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

// YQLGatewayAttr is a generic name/value setting with optional activation policy.
type YQLGatewayAttr struct {
	// Name is setting name.
	Name string `yson:"name"`
	// Value is setting value.
	Value string `yson:"value"`
	// Activation controls conditional activation. See TActivationPercentage.
	Activation any `yson:"activation,omitempty"`
}

// VanillaJobFile describes a file injected into vanilla DQ jobs.
type VanillaJobFile struct {
	// Name is a file name in the job sandbox.
	Name string `yson:"name"`
	// LocalPath is a local source file path.
	LocalPath string `yson:"local_path"`
}

// ProcessYqlPluginConfig configures process-isolated YQL plugin execution.
type ProcessYqlPluginConfig struct {
	// Enabled toggles process plugin mode.
	Enabled *bool `yson:"enabled,omitempty"`
	// SlotCount is the number of plugin slots.
	SlotCount int `yson:"slot_count,omitempty"`
	// SlotsRootPath is the root path for slot directories.
	SlotsRootPath string `yson:"slots_root_path,omitempty"`
	// CheckProcessActiveDelay is health-check interval for plugin processes.
	CheckProcessActiveDelay *yson.Duration `yson:"check_process_active_delay,omitempty"`
	// DefaultRequestTimeout is timeout for regular plugin requests.
	DefaultRequestTimeout *yson.Duration `yson:"default_request_timeout,omitempty"`
	// RunRequestTimeout is timeout for long-running run requests.
	RunRequestTimeout *yson.Duration `yson:"run_request_timeout,omitempty"`
	// LogManagerTemplate is a log manager config template.
	LogManagerTemplate map[string]any `yson:"log_manager_template,omitempty"`
}

// DQManagerConfig contains runtime settings for DQ manager and workers.
type DQManagerConfig struct {
	// InterconnectPort is an interconnect listener port.
	InterconnectPort *int `yson:"interconnect_port,omitempty"`
	// GrpcPort is a gRPC listener port.
	GrpcPort *int `yson:"grpc_port,omitempty"`
	// ActorThreads is number of actor runtime threads.
	ActorThreads *int `yson:"actor_threads,omitempty"`
	// UseIPv4 forces IPv4 usage for interconnect.
	UseIPv4 *bool `yson:"use_ipv4,omitempty"`
	// YTBackends lists configured DQ YT backends.
	YTBackends []DQYTBackend `yson:"yt_backends,omitempty"`
	// YTCoordinator stores DQ coordinator settings.
	YTCoordinator *DQYTCoordinator `yson:"yt_coordinator,omitempty"`
	// AddressResolver stores optional address resolver config.
	AddressResolver map[string]any `yson:"address_resolver,omitempty"`
	// InterconnectSettings stores raw interconnect settings map.
	InterconnectSettings map[string]any `yson:"interconnect_settings,omitempty"`
}

// DQYTBackend describes a DQ backend executed on YT.
type DQYTBackend struct {
	// ClusterName is the target YT cluster name.
	ClusterName string `yson:"cluster_name,omitempty"`
	// JobsPerOperation limits jobs per spawned operation.
	JobsPerOperation int `yson:"jobs_per_operation,omitempty"`
	// MaxJobs limits total jobs for this backend.
	MaxJobs int `yson:"max_jobs,omitempty"`
	// VanillaJobLite points to a lightweight job binary.
	VanillaJobLite string `yson:"vanilla_job_lite,omitempty"`
	// VanillaJobCommand is the command used to start vanilla job.
	VanillaJobCommand string `yson:"vanilla_job_command,omitempty"`
	// VanillaJobFiles lists extra files uploaded to job sandbox.
	VanillaJobFiles []VanillaJobFile `yson:"vanilla_job_file,omitempty"`
	// Prefix is cypress path prefix for backend data.
	Prefix string `yson:"prefix,omitempty"`
	// UploadReplicationFactor is replication factor for uploaded files.
	UploadReplicationFactor int `yson:"upload_replication_factor,omitempty"`
	// TokenFile is path to token file used by backend jobs.
	TokenFile string `yson:"token_file,omitempty"`
	// User is YT user name for backend operations.
	User string `yson:"user,omitempty"`
	// Pool is scheduler pool for backend operations.
	Pool string `yson:"pool,omitempty"`
	// PoolTrees is list of scheduler pool trees.
	PoolTrees []string `yson:"pool_trees,omitempty"`
	// Owner is owner list written to backend artifacts.
	Owner []string `yson:"owner,omitempty"`
	// CPULimit is CPU limit per job.
	CPULimit int64 `yson:"cpu_limit,omitempty"`
	// WorkerCapacity is DQ worker capacity value.
	WorkerCapacity int `yson:"worker_capacity,omitempty"`
	// MemoryLimit is memory limit per job.
	MemoryLimit int64 `yson:"memory_limit,omitempty"`
	// CacheSize is local cache size for DQ runtime.
	CacheSize int64 `yson:"cache_size,omitempty"`
	// UseTmpFS enables tmpfs usage in job sandbox.
	UseTmpFS *bool `yson:"use_tmp_fs,omitempty"`
	// NetworkProject is network project to run jobs in.
	NetworkProject string `yson:"network_project,omitempty"`
	// CanUseComputeActor controls compute actor usage.
	CanUseComputeActor bool `yson:"can_use_compute_actor,omitempty"`
	// EnforceJobUTC enables UTC enforcement in jobs.
	EnforceJobUTC *bool `yson:"enforce_job_utc,omitempty"`
	// UseLocalLDLibraryPath enables local LD_LIBRARY_PATH setup.
	UseLocalLDLibraryPath *bool `yson:"use_local_l_d_library_path,omitempty"`
	// SchedulingTagFilter is YT scheduling tag filter formula.
	SchedulingTagFilter string `yson:"scheduling_tag_filter,omitempty"`
}

// DQYTCoordinator describes DQ coordinator settings.
type DQYTCoordinator struct {
	// ClusterName is the target YT cluster name.
	ClusterName string `yson:"cluster_name,omitempty"`
	// Prefix is cypress path prefix for coordinator data.
	Prefix string `yson:"prefix,omitempty"`
	// TokenFile is path to token file used by coordinator.
	TokenFile string `yson:"token_file,omitempty"`
	// User is YT user name for coordinator operations.
	User string `yson:"user,omitempty"`
	// DebugLogFile is optional debug log output path.
	DebugLogFile string `yson:"debug_log_file,omitempty"`
}

// DQGatewayAutoByHour defines probability override for a specific hour.
type DQGatewayAutoByHour struct {
	// Hour is hour in range [0, 23].
	Hour int `yson:"hour"`
	// Percentage is percentage value for the hour.
	Percentage int `yson:"percentage"`
}

// DQGatewayConfig mirrors NYql::TDqGatewayConfig.
type DQGatewayConfig struct {
	// DefaultAutoPercentage is default percentage for DQ auto mode.
	DefaultAutoPercentage int `yson:"default_auto_percentage,omitempty"`
	// DefaultAutoByHour defines per-hour auto mode percentages.
	DefaultAutoByHour []DQGatewayAutoByHour `yson:"default_auto_by_hour,omitempty"`
	// NoDefaultAutoForUsers disables default auto mode for listed users.
	NoDefaultAutoForUsers []string `yson:"no_default_auto_for_users,omitempty"`
	// DefaultAnalyzeQueryForUsers enables analyze_query for listed users.
	DefaultAnalyzeQueryForUsers []string `yson:"default_analyze_query_for_users,omitempty"`
	// DefaultSettings stores DQ gateway default settings.
	DefaultSettings []YQLGatewayAttr `yson:"default_settings,omitempty"`

	// WithHiddenPercentage is legacy hidden-mode percentage.
	WithHiddenPercentage int `yson:"with_hidden_percentage,omitempty"`
	// WithHiddenByHour is legacy per-hour hidden-mode percentage.
	WithHiddenByHour []DQGatewayAutoByHour `yson:"with_hidden_by_hour,omitempty"`
	// NoWithHiddenForUsers is legacy hidden-mode denylist.
	NoWithHiddenForUsers []string `yson:"no_with_hidden_for_users,omitempty"`
	// WithHiddenForUsers is legacy hidden-mode allowlist.
	WithHiddenForUsers []string `yson:"with_hidden_for_users,omitempty"`
	// HiddenActivation controls hidden-mode activation.
	HiddenActivation any `yson:"hidden_activation,omitempty"`
}

// YtflowClusterConfig describes one Ytflow cluster mapping.
type YtflowClusterConfig struct {
	// Name is YQL short cluster name.
	Name string `yson:"name,omitempty"`
	// RealName is physical cluster short name.
	RealName string `yson:"real_name,omitempty"`
	// ProxyURL is proxy URL for this cluster.
	ProxyURL string `yson:"proxy_url,omitempty"`
	// Token is explicit token value for cluster access.
	Token string `yson:"token,omitempty"`
	// Settings stores per-cluster settings.
	Settings []YQLGatewayAttr `yson:"settings,omitempty"`
}

// YtflowGatewayConfig describes Ytflow gateway settings.
type YtflowGatewayConfig struct {
	// GatewayThreads is number of gateway worker threads.
	GatewayThreads int `yson:"gateway_threads,omitempty"`
	// YtflowWorkerBin is path to ytflow worker executable.
	YtflowWorkerBin string `yson:"ytflow_worker_bin,omitempty"`
	// ClusterMapping lists available Ytflow clusters.
	ClusterMapping []YtflowClusterConfig `yson:"cluster_mapping,omitempty"`
	// DefaultSettings stores gateway default settings.
	DefaultSettings []YQLGatewayAttr `yson:"default_settings,omitempty"`
}

// PQClusterConfig describes one PersQueue cluster mapping.
type PQClusterConfig struct {
	// Name is cluster alias.
	Name string `yson:"name,omitempty"`
	// ClusterType is PQ cluster type value.
	ClusterType string `yson:"cluster_type,omitempty"`
	// Endpoint is data endpoint.
	Endpoint string `yson:"endpoint,omitempty"`
	// ConfigManagerEndpoint is config manager endpoint.
	ConfigManagerEndpoint string `yson:"config_manager_endpoint,omitempty"`
	// Token is explicit token value.
	Token string `yson:"token,omitempty"`
	// Database is database path.
	Database string `yson:"database,omitempty"`
	// TvmID is TVM client id.
	TvmID int `yson:"tvm_id,omitempty"`
	// UseSSL toggles TLS for endpoint communication.
	UseSSL *bool `yson:"use_ssl,omitempty"`
	// ServiceAccountID is cloud service account id.
	ServiceAccountID string `yson:"service_account_id,omitempty"`
	// ServiceAccountIDSignature is cloud service account signature.
	ServiceAccountIDSignature string `yson:"service_account_id_signature,omitempty"`
	// AddBearerToToken prepends "Bearer " to token value.
	AddBearerToToken *bool `yson:"add_bearer_to_token,omitempty"`
	// DatabaseID is cloud database identifier.
	DatabaseID string `yson:"database_id,omitempty"`
	// Settings stores per-cluster settings.
	Settings []YQLGatewayAttr `yson:"settings,omitempty"`
	// SharedReading toggles shared reading mode.
	SharedReading *bool `yson:"shared_reading,omitempty"`
	// ReconnectPeriod sets reconnect period string.
	ReconnectPeriod string `yson:"reconnect_period,omitempty"`
	// ReadGroup sets read group name.
	ReadGroup string `yson:"read_group,omitempty"`
}

// PQGatewayConfig describes PersQueue gateway settings.
type PQGatewayConfig struct {
	// ClusterMapping lists configured PQ clusters.
	ClusterMapping []PQClusterConfig `yson:"cluster_mapping,omitempty"`
	// DefaultToken is fallback token value.
	DefaultToken string `yson:"default_token,omitempty"`
	// DefaultSettings stores gateway default settings.
	DefaultSettings []YQLGatewayAttr `yson:"default_settings,omitempty"`
}

// SolomonShardPath identifies Solomon shard path.
type SolomonShardPath struct {
	// Project is project/cloud id.
	Project string `yson:"project"`
	// Cluster is cluster/folder id.
	Cluster string `yson:"cluster"`
}

// SolomonClusterConfig describes one Solomon cluster mapping.
type SolomonClusterConfig struct {
	// Name is cluster alias.
	Name string `yson:"name,omitempty"`
	// Cluster is cluster endpoint or name.
	Cluster string `yson:"cluster,omitempty"`
	// UseSSL toggles TLS for cluster communication.
	UseSSL *bool `yson:"use_ssl,omitempty"`
	// ClusterType is Solomon cluster type.
	ClusterType string `yson:"cluster_type,omitempty"`
	// Token is explicit token value.
	Token string `yson:"token,omitempty"`
	// ServiceAccountID is cloud service account id.
	ServiceAccountID string `yson:"service_account_id,omitempty"`
	// ServiceAccountIDSignature is cloud service account signature.
	ServiceAccountIDSignature string `yson:"service_account_id_signature,omitempty"`
	// Path is optional Solomon shard path.
	Path *SolomonShardPath `yson:"path,omitempty"`
	// Settings stores per-cluster settings.
	Settings []YQLGatewayAttr `yson:"settings,omitempty"`
}

// SolomonGatewayConfig describes Solomon gateway settings.
type SolomonGatewayConfig struct {
	// ClusterMapping lists configured Solomon clusters.
	ClusterMapping []SolomonClusterConfig `yson:"cluster_mapping,omitempty"`
	// DefaultSettings stores gateway default settings.
	DefaultSettings []YQLGatewayAttr `yson:"default_settings,omitempty"`
}

// AdditionalSystemLib describes an extra dynamic library path.
type AdditionalSystemLib struct {
	// File is a local path to the shared library.
	File string `yson:"file"`
}

// RemoteFilePattern describes a remote file URL matching rule.
type RemoteFilePattern struct {
	// Pattern is a regexp-like URL pattern.
	Pattern string `yson:"pattern,omitempty"`
	// Cluster is cluster name extracted from URL.
	Cluster string `yson:"cluster,omitempty"`
	// Path is cypress/object path extracted from URL.
	Path string `yson:"path,omitempty"`
}

// FileWithMD5 describes a file path with expected md5 checksum.
type FileWithMD5 struct {
	// File is a local path to file.
	File string `yson:"file,omitempty"`
	// MD5 is expected md5 checksum.
	MD5 string `yson:"md5,omitempty"`
}

// YTClusterMapping describes one YT cluster entry in gateway mapping.
type YTClusterMapping struct {
	// Name is YQL cluster alias.
	Name string `yson:"name,omitempty"`
	// Cluster is actual YT cluster/proxy name.
	Cluster string `yson:"cluster,omitempty"`
	// Default marks default cluster for queries without explicit cluster.
	Default bool `yson:"default,omitempty"`
	// YTToken is cluster-specific token.
	YTToken string `yson:"yt_token,omitempty"`
	// YTName is optional display name of cluster.
	YTName string `yson:"yt_name,omitempty"`
	// EnabledYtQlQueries enables YTQL queries on this cluster.
	EnabledYtQlQueries bool `yson:"enabled_yt_ql_queries,omitempty"`
	// EnabledSpytQueries enables SPYT queries on this cluster.
	EnabledSpytQueries bool `yson:"enabled_spyt_queries,omitempty"`
	// Settings stores per-cluster default YQL settings.
	Settings []YQLGatewayAttr `yson:"settings,omitempty"`
}

// YTGatewayConfig mirrors NYql::TYtGatewayConfig.
type YTGatewayConfig struct {
	// GatewayThreads is number of gateway threads.
	GatewayThreads int `yson:"gateway_threads,omitempty"`
	// YTLogLevel is YT gateway log verbosity level.
	YTLogLevel string `yson:"yt_log_level,omitempty"`
	// MRJobBin is path to map-reduce job binary.
	MRJobBin string `yson:"mr_job_bin,omitempty"`
	// MRJobBinMD5 is md5 of mr job binary.
	MRJobBinMD5 string `yson:"mr_job_bin_md5,omitempty"`
	// MRJobUDFsDir is path to directory with UDF libraries.
	MRJobUDFsDir string `yson:"mr_job_udfs_dir,omitempty"`
	// ExecuteUDFLocallyIfPossible enables local UDF execution optimization.
	ExecuteUDFLocallyIfPossible *bool `yson:"execute_udf_locally_if_possible,omitempty"`
	// LocalChainTest enables local chain test mode.
	LocalChainTest *bool `yson:"local_chain_test,omitempty"`
	// YTDebugLogFile is path to YT debug log file.
	YTDebugLogFile string `yson:"yt_debug_log_file,omitempty"`
	// YTDebugLogSize is max YT debug log file size.
	YTDebugLogSize int64 `yson:"yt_debug_log_size,omitempty"`
	// YTDebugLogAlwaysWrite forces writing debug log even if empty.
	YTDebugLogAlwaysWrite *bool `yson:"yt_debug_log_always_write,omitempty"`
	// LocalChainFile is path to local chain file.
	LocalChainFile string `yson:"local_chain_file,omitempty"`
	// MRJobSystemLibsWithMD5 lists additional system libs for mr jobs.
	MRJobSystemLibsWithMD5 []FileWithMD5 `yson:"mr_job_system_libs_with_md5,omitempty"`
	// RemoteFilePatterns lists URL patterns for remote files.
	RemoteFilePatterns []RemoteFilePattern `yson:"remote_file_patterns,omitempty"`
	// ClusterMapping lists YT clusters available to gateway.
	ClusterMapping []YTClusterMapping `yson:"cluster_mapping,omitempty"`
	// DefaultSettings stores gateway-level default settings.
	DefaultSettings []YQLGatewayAttr `yson:"default_settings,omitempty"`
}

// See TYqlPluginConfig in yt/yql/plugin/config.h
// Also yql/essentials/providers/common/proto/gateways_config.proto
type YQLAgent struct {
	// GatewayConfig stores generic YT gateway settings.
	GatewayConfig *YTGatewayConfig `yson:"gateway_config,omitempty"`

	// EnableDQ enables DQ execution path.
	EnableDQ *bool `yson:"enable_dq,omitempty"`

	// DQGatewayConfig stores DQ gateway settings.
	DQGatewayConfig *DQGatewayConfig `yson:"dq_gateway_config,omitempty"`

	// DQManagerConfig stores DQ runtime manager settings.
	DQManagerConfig *DQManagerConfig `yson:"dq_manager_config,omitempty"`

	// YtflowGatewayConfig stores Ytflow gateway settings.
	YtflowGatewayConfig *YtflowGatewayConfig `yson:"ytflow_gateway_config,omitempty"`

	// PQGatewayConfig stores PersQueue gateway settings.
	PQGatewayConfig *PQGatewayConfig `yson:"pq_gateway_config,omitempty"`

	// SolomonGatewayConfig stores Solomon gateway settings.
	SolomonGatewayConfig *SolomonGatewayConfig `yson:"solomon_gateway_config,omitempty"`

	// FileStorageConfig stores file storage subsystem settings.
	FileStorageConfig map[string]any `yson:"file_storage_config,omitempty"`

	// TvmConfig stores TVM-related settings.
	TvmConfig map[string]any `yson:"tvm_config,omitempty"`

	// YtAccessProviderConfig stores YT access provider settings.
	YTAccessProviderConfig map[string]any `yson:"yt_access_provider_config,omitempty"`

	// YTTokenPath is a path to YT token file.
	YTTokenPath string `yson:"yt_token_path,omitempty"`

	// OperationAttributes stores operation-level YSON attributes.
	OperationAttributes map[string]any `yson:"operation_attributes,omitempty"`

	// Libraries maps library aliases to library paths.
	Libraries map[string]string `yson:"libraries,omitempty"`

	// YqlPluginSharedLibrary points to a shared yql plugin library.
	YqlPluginSharedLibrary string `yson:"yql_plugin_shared_library,omitempty"`

	// AdditionalSystemLibs lists extra shared libraries to ship.
	AdditionalSystemLibs []AdditionalSystemLib `yson:"additional_system_libs,omitempty"`

	// ProcessPluginConfig stores out-of-process plugin settings.
	ProcessPluginConfig *ProcessYqlPluginConfig `yson:"process_plugin_config,omitempty"`

	// For backward compatibility.
	MRJobBinary        string            `yson:"mr_job_binary,omitempty"`
	UDFDirectory       string            `yson:"udf_directory,omitempty"`
	AdditionalClusters map[string]string `yson:"additional_clusters"`
	DefaultCluster     string            `yson:"default_cluster"`
}

type YQLAgentServer struct {
	CommonServer
	User     string   `yson:"user"`
	YQLAgent YQLAgent `yson:"yql_agent"`
}

func getYQLAgentLogging(spec *ytv1.YQLAgentSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		consts.GetServiceKebabCase(consts.YqlAgentType),
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultDebugLoggerSpec(), defaultStderrLoggerSpec()})
}

func getYQLAgentServerCarcass(spec *ytv1.YQLAgentSpec) (YQLAgentServer, error) {
	c := YQLAgentServer{
		CommonServer: CommonServer{
			BasicServer: BasicServer{
				RPCPort:        consts.YQLAgentRPCPort,
				MonitoringPort: ptr.Deref(spec.MonitoringPort, consts.YQLAgentMonitoringPort),
			},
		},
		User: consts.YQLAgentUserName,
		YQLAgent: YQLAgent{
			YTTokenPath: getTokenVolumePath(consts.YQLAgentTokenVolumeName),
			GatewayConfig: &YTGatewayConfig{
				MRJobBin:     "/usr/bin/mrjob",
				MRJobUDFsDir: "/usr/lib/yql",
			},
			YqlPluginSharedLibrary: "/usr/lib/yql/libyqlplugin.so",
			AdditionalSystemLibs: []AdditionalSystemLib{
				{File: "/usr/lib/yql/libiconv.so"},
				{File: "/usr/lib/yql/liblibidn-dynamic.so"},
			},
		},
	}

	// For backward compatibility.
	c.YQLAgent.MRJobBinary = c.YQLAgent.GatewayConfig.MRJobBin
	c.YQLAgent.UDFDirectory = c.YQLAgent.GatewayConfig.MRJobUDFsDir

	c.Logging = getYQLAgentLogging(spec)

	return c, nil
}
