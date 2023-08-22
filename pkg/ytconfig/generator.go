package ytconfig

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"go.ytsaurus.tech/yt/go/yson"
	ptr "k8s.io/utils/pointer"
)

type GeneratorFunc func() ([]byte, error)
type ReloadCheckerFunc func([]byte) (bool, error)

type Generator struct {
	ytsaurus      *ytv1.Ytsaurus
	clusterDomain string
}

func NewGenerator(ytsaurus *ytv1.Ytsaurus, clusterDomain string) *Generator {
	return &Generator{
		ytsaurus:      ytsaurus,
		clusterDomain: clusterDomain,
	}
}

func (g *Generator) getMasterAddresses() []string {
	names := make([]string, 0, g.ytsaurus.Spec.PrimaryMasters.InstanceCount)
	for _, podName := range g.GetMasterPodNames() {
		names = append(names, fmt.Sprintf("%s.%s.%s.svc.%s:%d",
			podName,
			g.GetMastersServiceName(),
			g.ytsaurus.Namespace,
			g.clusterDomain,
			consts.MasterRPCPort))
	}
	return names
}

func (g *Generator) getDiscoveryAddresses() []string {
	names := make([]string, 0, g.ytsaurus.Spec.Discovery.InstanceCount)
	for _, podName := range g.GetDiscoveryPodNames() {
		names = append(names, fmt.Sprintf("%s.%s.%s.svc.%s:%d",
			podName,
			g.GetDiscoveryServiceName(),
			g.ytsaurus.Namespace,
			g.clusterDomain,
			consts.DiscoveryRPCPort))
	}
	return names
}

func (g *Generator) GetYQLAgentAddresses() []string {
	names := make([]string, 0, g.ytsaurus.Spec.YQLAgents.InstanceCount)
	for _, podName := range g.GetYQLAgentPodNames() {
		names = append(names, fmt.Sprintf("%s.%s.%s.svc.%s:%d",
			podName,
			g.GetYQLAgentServiceName(),
			g.ytsaurus.Namespace,
			g.clusterDomain,
			consts.YQLAgentRPCPort))
	}
	return names
}

func (g *Generator) fillDriver(c *Driver) {
	c.TimestampProviders.Addresses = g.getMasterAddresses()

	c.PrimaryMaster.Addresses = g.getMasterAddresses()
	c.PrimaryMaster.CellID = generateCellID(g.ytsaurus.Spec.PrimaryMasters.CellTag)

	c.MasterCache.enableMasterCacheDiscover = true
	g.fillPrimaryMaster(&c.MasterCache.MasterCell)
}

func (g *Generator) fillAddressResolver(c *AddressResolver) {
	var retries int = 1000

	c.EnableIPv4 = !g.ytsaurus.Spec.UseIPv6
	c.EnableIPv6 = g.ytsaurus.Spec.UseIPv6
	c.Retries = &retries
}

func (g *Generator) fillPrimaryMaster(c *MasterCell) {
	c.Addresses = g.getMasterAddresses()
	c.CellID = generateCellID(g.ytsaurus.Spec.PrimaryMasters.CellTag)
}

func (g *Generator) fillClusterConnection(c *ClusterConnection) {
	g.fillPrimaryMaster(&c.PrimaryMaster)
	c.ClusterName = g.ytsaurus.Name
	c.DiscoveryConnection.Addresses = g.getDiscoveryAddresses()
}

func (g *Generator) fillCommonService(c *CommonServer) {
	// ToDo(psushin): enable porto resource tracker?
	g.fillAddressResolver(&c.AddressResolver)
	g.fillClusterConnection(&c.ClusterConnection)
	c.TimestampProviders.Addresses = g.getMasterAddresses()
}

func marshallYsonConfig(c interface{}) ([]byte, error) {
	result, err := yson.MarshalFormat(c, yson.FormatPretty)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func marshallJSONConfig(c interface{}) ([]byte, error) {
	result, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (g *Generator) GetClusterConnection() ([]byte, error) {
	var c ClusterConnection
	g.fillClusterConnection(&c)
	return marshallYsonConfig(c)
}

func (g *Generator) GetChytControllerConfig() ([]byte, error) {
	c := getChytController()
	c.LocationProxies = []string{
		g.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole),
	}
	return marshallYsonConfig(c)
}

func (g *Generator) GetChytInitClusterConfig() ([]byte, error) {
	c := getChytInitCluster()
	c.Proxy = g.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)
	return marshallYsonConfig(c)
}

func (g *Generator) NeedMasterConfigReload(spec ytv1.MastersSpec, data []byte) (bool, error) {
	currentConfig := MasterServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getMasterLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetMasterConfig() ([]byte, error) {
	c, err := getMasterServerCarcass(g.ytsaurus.Spec.PrimaryMasters)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)
	g.fillPrimaryMaster(&c.PrimaryMaster)
	configureMasterServerCypressManager(g.ytsaurus.Spec, &c.CypressManager)

	return marshallYsonConfig(c)
}

func (g *Generator) GetNativeClientConfig() ([]byte, error) {
	c, err := getNativeClientCarcass()
	if err != nil {
		return nil, err
	}

	g.fillDriver(&c.Driver)
	g.fillAddressResolver(&c.AddressResolver)
	c.Driver.APIVersion = 4

	return marshallYsonConfig(c)
}

func (g *Generator) NeedSchedulerConfigReload(spec ytv1.SchedulersSpec, data []byte) (bool, error) {
	currentConfig := SchedulerServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getSchedulerLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetSchedulerConfig() ([]byte, error) {
	if g.ytsaurus.Spec.Schedulers == nil {
		return []byte{}, nil
	}

	c, err := getSchedulerServerCarcass(*g.ytsaurus.Spec.Schedulers)
	if err != nil {
		return nil, err
	}

	if g.ytsaurus.Spec.TabletNodes == nil {
		c.Scheduler.OperationsCleaner.EnableOperationArchivation = ptr.Bool(false)
	}
	g.fillCommonService(&c.CommonServer)
	return marshallYsonConfig(c)
}

func (g *Generator) NeedRPCProxyConfigReload(spec ytv1.RPCProxiesSpec, data []byte) (bool, error) {
	currentConfig := RPCProxyServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getRPCProxyLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetRPCProxyConfig(spec ytv1.RPCProxiesSpec) ([]byte, error) {
	c, err := getRPCProxyServerCarcass(spec)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)
	return marshallYsonConfig(c)
}

func (g *Generator) NeedTCPProxyConfigReload(spec ytv1.TCPProxiesSpec, data []byte) (bool, error) {
	currentConfig := TCPProxyServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getTCPProxyLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetTCPProxyConfig(spec ytv1.TCPProxiesSpec) ([]byte, error) {
	c, err := getTCPProxyServerCarcass(spec)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)
	return marshallYsonConfig(c)
}

func (g *Generator) NeedControllerAgentConfigReload(spec ytv1.ControllerAgentsSpec, data []byte) (bool, error) {
	currentConfig := ControllerAgentServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getControllerAgentLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetControllerAgentConfig() ([]byte, error) {
	if g.ytsaurus.Spec.ControllerAgents == nil {
		return []byte{}, nil
	}
	c, err := getControllerAgentServerCarcass(*g.ytsaurus.Spec.ControllerAgents)
	if err != nil {
		return nil, err
	}

	c.ControllerAgent.EnableTmpfs = g.ytsaurus.Spec.UsePorto
	c.ControllerAgent.UseColumnarStatisticsDefault = true

	g.fillCommonService(&c.CommonServer)
	return marshallYsonConfig(c)
}

func (g *Generator) NeedDataNodeConfigReload(spec ytv1.DataNodesSpec, data []byte) (bool, error) {
	currentConfig := DataNodeServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getDataNodeLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetDataNodeConfig(spec ytv1.DataNodesSpec) ([]byte, error) {
	c, err := getDataNodeServerCarcass(spec)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)

	return marshallYsonConfig(c)
}
func (g *Generator) NeedExecNodeConfigReload(spec ytv1.ExecNodesSpec, data []byte) (bool, error) {
	currentConfig := ExecNodeServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getExecNodeResourceLimits(spec), currentConfig.ResourceLimits) {
		return true, nil
	}

	if !cmp.Equal(getExecNodeLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetExecNodeConfig(spec ytv1.ExecNodesSpec) ([]byte, error) {
	c, err := getExecNodeServerCarcass(
		spec,
		g.ytsaurus.Spec.UsePorto)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)

	return marshallYsonConfig(c)
}

func (g *Generator) NeedTabletNodeConfigReload(spec ytv1.TabletNodesSpec, data []byte) (bool, error) {
	currentConfig := TabletNodeServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getTabletNodeLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetTabletNodeConfig(spec ytv1.TabletNodesSpec) ([]byte, error) {
	c, err := getTabletNodeServerCarcass(spec)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)

	return marshallYsonConfig(c)
}

func (g *Generator) NeedHTTPProxyConfigReload(spec ytv1.HTTPProxiesSpec, data []byte) (bool, error) {
	currentConfig := HTTPProxyServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getHTTPProxyLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetHTTPProxyConfig(spec ytv1.HTTPProxiesSpec) ([]byte, error) {
	c, err := getHTTPProxyServerCarcass(spec)
	if err != nil {
		return nil, err
	}

	g.fillDriver(&c.Driver)
	g.fillClusterConnection(&c.ClusterConnection)
	g.fillAddressResolver(&c.AddressResolver)

	return marshallYsonConfig(c)
}

func (g *Generator) NeedQueryTrackerConfigReload(spec ytv1.QueryTrackerSpec, data []byte) (bool, error) {
	currentConfig := QueryTrackerServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getQueryTrackerLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetQueryTrackerConfig() ([]byte, error) {
	if g.ytsaurus.Spec.QueryTrackers == nil {
		return []byte{}, nil
	}
	c, err := getQueryTrackerServerCarcass(*g.ytsaurus.Spec.QueryTrackers)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)

	return marshallYsonConfig(c)
}

func (g *Generator) NeedYQLAgentConfigReload(spec ytv1.YQLAgentSpec, data []byte) (bool, error) {
	currentConfig := YQLAgentServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getYQLAgentLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetYQLAgentConfig() ([]byte, error) {
	if g.ytsaurus.Spec.YQLAgents == nil {
		return []byte{}, nil
	}
	c, err := getYQLAgentServerCarcass(*g.ytsaurus.Spec.YQLAgents)
	if err != nil {
		return nil, err
	}
	g.fillCommonService(&c.CommonServer)
	c.YQLAgent.AdditionalClusters = map[string]string{
		g.ytsaurus.Name: g.GetHTTPProxiesServiceAddress(consts.DefaultHTTPProxyRole),
	}
	c.YQLAgent.DefaultCluster = g.ytsaurus.Name

	return marshallYsonConfig(c)
}

func (g *Generator) GetWebUIConfig() ([]byte, error) {
	if g.ytsaurus.Spec.UI == nil {
		return []byte{}, nil
	}

	c := getUIClusterCarcass()
	c.ID = g.ytsaurus.Name
	c.Name = g.ytsaurus.Name
	c.Proxy = g.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)
	c.PrimaryMaster.CellTag = g.ytsaurus.Spec.PrimaryMasters.CellTag

	return marshallJSONConfig(WebUI{Clusters: []UICluster{c}})
}

func (g *Generator) NeedDiscoveryConfigReload(spec ytv1.DiscoverySpec, data []byte) (bool, error) {
	currentConfig := DiscoveryServer{}

	if err := yson.Unmarshal(data, &currentConfig); err != nil {
		return false, err
	}

	if !cmp.Equal(getDiscoveryLogging(spec), currentConfig.Logging) {
		return true, nil
	}

	return false, nil
}

func (g *Generator) GetDiscoveryConfig() ([]byte, error) {
	c := getDiscoveryServerCarcass(g.ytsaurus.Spec.Discovery)

	g.fillCommonService(&c.CommonServer)
	c.DiscoveryServer.Addresses = g.getDiscoveryAddresses()

	return marshallYsonConfig(c)
}
