package ytconfig

import (
	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"go.ytsaurus.tech/yt/go/yson"
)

type AddressList struct {
	Addresses []string `yson:"addresses"`
}

type HydraPeer struct {
	Address string `yson:"address"`
	Voting  bool   `yson:"voting"`
}

type MasterCell struct {
	AddressList
	Peers  []HydraPeer `yson:"peers"`
	CellID string      `yson:"cell_id"`
}

type TimestampProviders struct {
	AddressList
}

type DiscoveryConnection struct {
	AddressList
}

type MasterCache struct {
	AddressList
	CellID                    string `yson:"cell_id"`
	EnableMasterCacheDiscover bool   `yson:"enable_master_cache_discovery"`
}

type Driver struct {
	TimestampProviders TimestampProviders `yson:"timestamp_provider,omitempty"`
	PrimaryMaster      MasterCell         `yson:"primary_master,omitempty"`
	APIVersion         int                `yson:"api_version,omitempty"`
}

type ClusterConnection struct {
	ClusterName         string              `yson:"cluster_name"`
	PrimaryMaster       MasterCell          `yson:"primary_master"`
	DiscoveryConnection DiscoveryConnection `yson:"discovery_connection,omitempty"`
	BusClient           *Bus                `yson:"bus_client,omitempty"`
	MasterCache         MasterCache         `yson:"master_cache"`
}

type AddressResolver struct {
	EnableIPv4 bool  `yson:"enable_ipv4"`
	EnableIPv6 bool  `yson:"enable_ipv6"`
	KeepSocket *bool `yson:"keep_socket,omitempty"`
	ForceTCP   *bool `yson:"force_tcp,omitempty"`
	Retries    *int  `yson:"retries,omitempty"`

	LocalhostNameOverride *string `yson:"localhost_name_override,omitempty"`
}

type StrawberryControllerFamiliesConfig struct {
	ControllerFamilies []string `yson:"controller_families,omitempty"`
	DefaultRouteFamily string   `yson:"default_route_family,omitempty"`
	ExternalProxy      *string  `yson:"external_proxy,omitempty"`
}

type SolomonExporter struct {
	Host         *string           `yson:"host,omitempty"`
	InstanceTags map[string]string `yson:"instance_tags,omitempty"`
}

type Stockpile struct {
	BufferSize                  int           `yson:"buffer_size,omitempty"`
	ThreadCount                 int           `yson:"thread_count"`
	Strategy                    string        `yson:"strategy,omitempty"`
	Period                      yson.Duration `yson:"period,omitempty"`
	TotalMemoryFractionOverride float32       `yson:"total_memory_fraction_override,omitempty"`
}

type PemBlob struct {
	FileName string `yson:"file_name,omitempty"`
	Value    string `yson:"value,omitempty"`
}

type EncryptionMode string

const (
	EncryptionModeDisabled EncryptionMode = "disabled"
	EncryptionModeOptional EncryptionMode = "optional"
	EncryptionModeRequired EncryptionMode = "required"
)

type VerificationMode string

const (
	VerificationModeNone VerificationMode = "none"
	VerificationModeCa   VerificationMode = "ca"
	VerificationModeFull VerificationMode = "full"
)

type Bus struct {
	EncryptionMode EncryptionMode `yson:"encryption_mode,omitempty"`
	CertChain      *PemBlob       `yson:"cert_chain,omitempty"`
	PrivateKey     *PemBlob       `yson:"private_key,omitempty"`
	CipherList     []string       `yson:"cipher_list,omitempty"`

	CA                      *PemBlob         `yson:"ca,omitempty"`
	VerificationMode        VerificationMode `yson:"verification_mode,omitempty"`
	PeerAlternativeHostName string           `yson:"peer_alternative_host_name,omitempty"`
}

type BusServer struct {
	Bus
}

// BasicServer is used as a basic config for basic components, such as clocks or discovery.
type BasicServer struct {
	AddressResolver AddressResolver `yson:"address_resolver"`
	SolomonExporter SolomonExporter `yson:"solomon_exporter"`
	Logging         Logging         `yson:"logging"`
	MonitoringPort  int32           `yson:"monitoring_port"`
	RPCPort         int32           `yson:"rpc_port"`
	BusServer       *BusServer      `yson:"bus_server,omitempty"`
	Stockpile       *Stockpile      `yson:"stockpile,omitempty"`
}

type CommonServer struct {
	BasicServer
	TimestampProviders TimestampProviders `yson:"timestamp_provider"`
	ClusterConnection  ClusterConnection  `yson:"cluster_connection"`
	CypressAnnotations map[string]any     `yson:"cypress_annotations,omitempty"`
}

type BindMount struct {
	ExternalPath string `yson:"external_path"`
	InternalPath string `yson:"internal_path"`
	ReadOnly     bool   `yson:"read_only"`
}

type RetryingChannel struct {
	RetryBackoffTime yson.Duration `yson:"retry_backoff_time,omitempty"`
	RetryAttempts    int32         `yson:"retry_attempts,omitempty"`
	RetryTimeout     yson.Duration `yson:"retry_timeout,omitempty"`
}

type IOEngine struct {
	EnableSync *bool `yson:"enable_sync,omitempty"`
}

func createLogging(spec *ytv1.InstanceSpec, componentName string, defaultLoggerSpecs []ytv1.TextLoggerSpec) Logging {
	loggingBuilder := newLoggingBuilder(ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeLogs), componentName)
	if len(spec.Loggers) > 0 {
		for _, loggerSpec := range spec.Loggers {
			loggingBuilder.addLogger(loggerSpec)
		}
	} else {
		for _, defaultLoggerSpec := range defaultLoggerSpecs {
			loggingBuilder.addLogger(defaultLoggerSpec)
		}
	}
	if len(spec.StructuredLoggers) > 0 {
		for _, loggerSpec := range spec.StructuredLoggers {
			loggingBuilder.addStructuredLogger(loggerSpec)
		}
	}
	loggingBuilder.logging.FlushPeriod = 3000
	return loggingBuilder.logging
}
