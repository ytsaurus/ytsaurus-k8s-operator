package ytconfig

type AddressList struct {
	Addresses []string `yson:"addresses"`
}

type MasterCell struct {
	AddressList
	CellId string `yson:"cell_id"`
}

type TimestampProviders struct {
	AddressList
}

type DiscoveryConnection struct {
	AddressList
}

type MasterCache struct {
	MasterCell
	enableMasterCacheDiscover bool `yson:"enable_master_cache_discovery"`
}

type Driver struct {
	MasterCache        MasterCache        `yson:"master_cache,omitempty"`
	TimestampProviders TimestampProviders `yson:"timestamp_provider,omitempty"`
	PrimaryMaster      MasterCell         `yson:"primary_master,omitempty"`
	ApiVersion         int                `yson:"api_version,omitempty"`
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
	RpcPort         int32           `yson:"rpc_port"`
}

type CommonServer struct {
	BasicServer
	TimestampProviders TimestampProviders `yson:"timestamp_provider"`
	ClusterConnection  ClusterConnection  `yson:"cluster_connection"`
}
