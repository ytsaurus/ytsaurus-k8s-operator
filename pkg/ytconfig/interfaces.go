package ytconfig

import ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"

type CommonConfigPartsGenerator interface {
	FormatComponentStringWithDefault(string, string) string
}

type ClusterConnectionProvider interface {
	GetClusterConnection() ([]byte, error)
}

type NativeClientConfigProvider interface {
	GetNativeClientConfig() ([]byte, error)
}

type HTTPProxiesServiceAddressProvider interface {
	GetHTTPProxiesServiceAddress(string) string
}

type MasterNodeConfigGenerator interface {
	NativeClientConfigProvider
	ClusterConnectionProvider

	GetMasterConfig(spec ytv1.YtsaurusSpec) ([]byte, error)

	GetMastersStatefulSetName() string
	GetMastersServiceName() string
}

type DiscoveryNodeConfigGenerator interface {
	GetDiscoveryConfig(spec *ytv1.DiscoverySpec) ([]byte, error)

	GetDiscoveryStatefulSetName() string
	GetDiscoveryServiceName() string
}

type SchedulerNodeConfigGenerator interface {
	NativeClientConfigProvider
	HTTPProxiesServiceAddressProvider

	GetSchedulerConfig(schedulerSpec *ytv1.SchedulersSpec, tabletNodesSpec []ytv1.TabletNodesSpec) ([]byte, error)

	GetSchedulerStatefulSetName() string
	GetSchedulerServiceName() string
}

type ControllerAgentConfigGenerator interface {
	GetControllerAgentConfig(spec *ytv1.ControllerAgentsSpec) ([]byte, error)
}

type MasterCacheNodeConfigGenerator interface {
	GetMasterCachesConfig(spec *ytv1.MasterCachesSpec) ([]byte, error)

	GetMasterCachesStatefulSetName() string
	GetMasterCachesServiceName() string
}

type DataNodeConfigGenerator interface {
	CommonConfigPartsGenerator

	GetDataNodeConfig(ytv1.DataNodesSpec) ([]byte, error)

	GetDataNodesStatefulSetName(string) string
	GetDataNodesServiceName(string) string
}

type TabletNodeConfigGenerator interface {
	CommonConfigPartsGenerator

	GetTabletNodeConfig(spec ytv1.TabletNodesSpec) ([]byte, error)

	GetTabletNodesStatefulSetName(string) string
	GetTabletNodesServiceName(string) string
}

type ExecNodeConfigGenerator interface {
	CommonConfigPartsGenerator

	GetExecNodeConfig(spec ytv1.ExecNodesSpec) ([]byte, error)

	GetExecNodesStatefulSetName(string) string
	GetExecNodesServiceName(string) string
}

type HTTPProxyNodeConfigGenerator interface {
	CommonConfigPartsGenerator

	GetHTTPProxyConfig(spec ytv1.HTTPProxiesSpec) ([]byte, error)

	GetHTTPProxiesStatefulSetName(string) string
	GetHTTPProxiesServiceName(string) string
	GetHTTPProxiesHeadlessServiceName(string) string
}

type RPCProxyNodeConfigGenerator interface {
	CommonConfigPartsGenerator

	GetRPCProxyConfig(spec ytv1.RPCProxiesSpec) ([]byte, error)

	GetRPCProxiesStatefulSetName(string) string
	GetRPCProxiesServiceName(string) string
	GetRPCProxiesHeadlessServiceName(string) string
}

type TCPNodeConfigGenerator interface {
	CommonConfigPartsGenerator

	GetTCPProxyConfig(spec *ytv1.TCPProxiesSpec) ([]byte, error)

	GetTCPProxiesStatefulSetName(string) string
	GetTCPProxiesServiceName(string) string
	GetTCPProxiesHeadlessServiceName(string) string
}

type QueryTrackerNodeConfigGenerator interface {
	NativeClientConfigProvider
	HTTPProxiesServiceAddressProvider

	GetQueryTrackerConfig(spec *ytv1.QueryTrackerSpec) ([]byte, error)

	GetQueryTrackerStatefulSetName() string
	GetQueryTrackerServiceName() string
}

type QueueAgentNodeConfigGenerator interface {
	NativeClientConfigProvider
	HTTPProxiesServiceAddressProvider

	GetQueueAgentConfig(spec *ytv1.QueueAgentSpec) ([]byte, error)

	GetQueueAgentStatefulSetName() string
	GetQueueAgentServiceName() string

	GetQueueAgentAddresses(spec *ytv1.QueueAgentSpec) []string
}

type YQLAgentNodeConfigGenerator interface {
	NativeClientConfigProvider

	GetYQLAgentConfig(spec *ytv1.YQLAgentSpec) ([]byte, error)

	GetYQLAgentStatefulSetName() string
	GetYQLAgentServiceName() string

	GetYQLAgentAddresses(spec *ytv1.YQLAgentSpec) []string
}

type ChytNodeConfigGenerator interface {
	NativeClientConfigProvider

	GetHTTPProxiesAddress(string) string

	GetStrawberryControllerServiceAddress() string
}

type StrawberryControllerNodeConfigGenerator interface {
	NativeClientConfigProvider

	GetChytInitClusterConfig(spec ytv1.StrawberryControllerSpec) ([]byte, error)

	GetStrawberryControllerConfig() ([]byte, error)
}
