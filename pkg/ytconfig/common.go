package ytconfig

import ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"

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
	MasterCell
	EnableMasterCacheDiscover bool `yson:"enable_master_cache_discovery"`
}

type Driver struct {
	MasterCache        MasterCache        `yson:"master_cache,omitempty"`
	TimestampProviders TimestampProviders `yson:"timestamp_provider,omitempty"`
	PrimaryMaster      MasterCell         `yson:"primary_master,omitempty"`
	APIVersion         int                `yson:"api_version,omitempty"`
}

type ClusterConnection struct {
	ClusterName         string              `yson:"cluster_name"`
	PrimaryMaster       MasterCell          `yson:"primary_master"`
	DiscoveryConnection DiscoveryConnection `yson:"discovery_connection,omitempty"`
}

type AddressResolver struct {
	EnableIPv4 bool `yson:"enable_ipv4"`
	EnableIPv6 bool `yson:"enable_ipv6"`
	Retries    *int `yson:"retries,omitempty"`
}

// This is used as a basic config for basic components, such as clocks or discovery.
type BasicServer struct {
	AddressResolver AddressResolver `yson:"address_resolver"`
	Logging         Logging         `yson:"logging"`
	MonitoringPort  int32           `yson:"monitoring_port"`
	RPCPort         int32           `yson:"rpc_port"`
}

type CommonServer struct {
	BasicServer
	TimestampProviders TimestampProviders `yson:"timestamp_provider"`
	ClusterConnection  ClusterConnection  `yson:"cluster_connection"`
}

func createLogging(spec *ytv1.InstanceSpec, componentName string, defaultLoggerSpecs []ytv1.LoggerSpec) Logging {
	loggingBuilder := newLoggingBuilder(ytv1.FindFirstLocation(spec.Locations, ytv1.LocationTypeLogs), componentName)
	if spec.Loggers != nil && len(spec.Loggers) > 0 {
		for _, loggerSpec := range spec.Loggers {
			loggingBuilder.addLogger(loggerSpec)
		}
	} else {
		for _, defaultLoggerSpec := range defaultLoggerSpecs {
			loggingBuilder.addLogger(defaultLoggerSpec)
		}
	}
	return loggingBuilder.logging
}
