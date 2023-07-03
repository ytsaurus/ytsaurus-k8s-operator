package ytconfig

import (
	"encoding/json"
	"fmt"

	v1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"go.ytsaurus.tech/yt/go/yson"
	ptr "k8s.io/utils/pointer"
)

type GeneratorFunc func() ([]byte, error)

type Generator struct {
	ytsaurus      *v1.Ytsaurus
	clusterDomain string
}

func NewGenerator(ytsaurus *v1.Ytsaurus, clusterDomain string) *Generator {
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

func (g *Generator) GetMasterConfig() ([]byte, error) {
	c, err := getMasterServerCarcass(g.ytsaurus.Spec.PrimaryMasters)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)
	g.fillPrimaryMaster(&c.PrimaryMaster)

	// Special replication policies for small clusters with few data nodes.
	if g.ytsaurus.Spec.DataNodes[0].InstanceCount <= 1 {
		c.CypressManager.DefaultJournalReadQuorum = 1
		c.CypressManager.DefaultJournalWriteQuorum = 1
		c.CypressManager.DefaultTableReplicationFactor = 1
		c.CypressManager.DefaultFileReplicationFactor = 1
		c.CypressManager.DefaultJournalReplicationFactor = 1
	} else if g.ytsaurus.Spec.DataNodes[0].InstanceCount == 2 {
		c.CypressManager.DefaultTableReplicationFactor = 2
		c.CypressManager.DefaultFileReplicationFactor = 2
		c.CypressManager.DefaultJournalReplicationFactor = 2
	} else if g.ytsaurus.Spec.DataNodes[0].InstanceCount >= 5 {
		// More robust journal replication factor for big clusters.
		c.CypressManager.DefaultJournalReadQuorum = 3
		c.CypressManager.DefaultJournalWriteQuorum = 3
		c.CypressManager.DefaultJournalReplicationFactor = 5
	}

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

func (g *Generator) GetRPCProxyConfig(spec v1.RPCProxiesSpec) ([]byte, error) {
	if g.ytsaurus.Spec.Schedulers == nil {
		return []byte{}, nil
	}

	c, err := getRPCProxyServerCarcass(spec)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)
	return marshallYsonConfig(c)
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

func (g *Generator) GetDataNodeConfig(spec v1.DataNodesSpec) ([]byte, error) {
	c, err := getDataNodeServerCarcass(spec)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)

	return marshallYsonConfig(c)
}

func (g *Generator) GetExecNodeConfig(spec v1.ExecNodesSpec) ([]byte, error) {
	c, err := getExecNodeServerCarcass(
		spec,
		g.ytsaurus.Spec.UsePorto)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)

	return marshallYsonConfig(c)
}

func (g *Generator) GetTabletNodeConfig(spec v1.TabletNodesSpec) ([]byte, error) {
	c, err := getTabletNodeServerCarcass(spec)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)

	return marshallYsonConfig(c)
}

func (g *Generator) GetHTTPProxyConfig(spec v1.HTTPProxiesSpec) ([]byte, error) {
	c, err := getHTTPProxyServerCarcass(spec)
	if err != nil {
		return nil, err
	}

	g.fillDriver(&c.Driver)
	g.fillClusterConnection(&c.ClusterConnection)
	g.fillAddressResolver(&c.AddressResolver)

	return marshallYsonConfig(c)
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

func (g *Generator) GetYQLAgentConfig() ([]byte, error) {
	if g.ytsaurus.Spec.YQLAgents == nil {
		return []byte{}, nil
	}
	c, err := getYQLAgentServerCarcass(*g.ytsaurus.Spec.YQLAgents)
	if err != nil {
		return nil, err
	}
	g.fillCommonService(&c.CommonServer)
	c.YQLAgent.Clusters = map[string]string{
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

func (g *Generator) GetDiscoveryConfig() ([]byte, error) {
	c := getDiscoveryServerCarcass(g.ytsaurus.Spec.Discovery)
	c.DiscoveryServer.Addresses = g.getDiscoveryAddresses()

	return marshallYsonConfig(c)
}
