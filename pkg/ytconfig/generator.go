package ytconfig

import (
	"encoding/json"
	"fmt"
	v1 "github.com/YTsaurus/yt-k8s-operator/api/v1"
	"github.com/YTsaurus/yt-k8s-operator/pkg/consts"
	"go.ytsaurus.tech/yt/go/yson"
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
	names := make([]string, 0, g.ytsaurus.Spec.Masters.InstanceGroup.InstanceCount)
	for _, podName := range g.GetMasterPodNames() {
		names = append(names, fmt.Sprintf("%s.%s.%s.svc.%s:%d",
			podName,
			g.GetMastersServiceName(),
			g.ytsaurus.Namespace,
			g.clusterDomain,
			consts.MasterRpcPort))
	}
	return names
}

func (g *Generator) getDiscoveryAddresses() []string {
	names := make([]string, 0, g.ytsaurus.Spec.Discovery.InstanceGroup.InstanceCount)
	for _, podName := range g.GetDiscoveryPodNames() {
		names = append(names, fmt.Sprintf("%s.%s.%s.svc.%s:%d",
			podName,
			g.GetDiscoveryServiceName(),
			g.ytsaurus.Namespace,
			g.clusterDomain,
			consts.DiscoveryRpcPort))
	}
	return names
}

func (g *Generator) GetYqlAgentAddresses() []string {
	names := make([]string, 0, g.ytsaurus.Spec.YqlAgents.InstanceGroup.InstanceCount)
	for _, podName := range g.GetYqlAgentPodNames() {
		names = append(names, fmt.Sprintf("%s.%s.%s.svc.%s:%d",
			podName,
			g.GetYqlAgentServiceName(),
			g.ytsaurus.Namespace,
			g.clusterDomain,
			consts.YqlAgentRpcPort))
	}
	return names
}

func (g *Generator) fillDriver(c *Driver) {
	c.TimestampProviders.Addresses = g.getMasterAddresses()

	c.PrimaryMaster.Addresses = g.getMasterAddresses()
	c.PrimaryMaster.CellId = generateCellId(g.ytsaurus.Spec.CellTag)

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
	c.CellId = generateCellId(g.ytsaurus.Spec.CellTag)
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

func marshallJsonConfig(c interface{}) ([]byte, error) {
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
		g.GetHttpProxiesAddress(),
	}
	return marshallYsonConfig(c)
}

func (g *Generator) GetChytInitClusterConfig() ([]byte, error) {
	c := getChytInitCluster()
	c.Proxy = g.GetHttpProxiesAddress()
	return marshallYsonConfig(c)
}

func (g *Generator) GetMasterConfig() ([]byte, error) {
	c, err := getMasterServerCarcass(g.ytsaurus.Spec.Masters)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)
	g.fillPrimaryMaster(&c.PrimaryMaster)
	return marshallYsonConfig(c)
}

func (g *Generator) GetNativeClientConfig() ([]byte, error) {
	c, err := getNativeClientCarcass()
	if err != nil {
		return nil, err
	}

	g.fillDriver(&c.Driver)
	g.fillAddressResolver(&c.AddressResolver)
	c.Driver.ApiVersion = 4

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

	if g.ytsaurus.Spec.TabletNodes != nil {
		c.Scheduler.OperationsCleaner.EnableOperationArchivation = true
	}
	g.fillCommonService(&c.CommonServer)
	return marshallYsonConfig(c)
}

func (g *Generator) GetRpcProxyConfig() ([]byte, error) {
	if g.ytsaurus.Spec.Schedulers == nil {
		return []byte{}, nil
	}

	c, err := getRpcProxyServerCarcass(*g.ytsaurus.Spec.RpcProxies)
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

	g.fillCommonService(&c.CommonServer)
	return marshallYsonConfig(c)
}

func (g *Generator) GetDataNodeConfig() ([]byte, error) {
	c, err := getDataNodeServerCarcass(g.ytsaurus.Spec.DataNodes)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)

	return marshallYsonConfig(c)
}

func (g *Generator) GetExecNodeConfig() ([]byte, error) {
	if g.ytsaurus.Spec.ExecNodes == nil {
		return []byte{}, nil
	}

	c, err := getExecNodeServerCarcass(
		*g.ytsaurus.Spec.ExecNodes,
		g.ytsaurus.Spec.UsePorto)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)

	return marshallYsonConfig(c)
}

func (g *Generator) GetTabletNodeConfig() ([]byte, error) {
	if g.ytsaurus.Spec.TabletNodes == nil {
		return []byte{}, nil
	}

	c, err := getTabletNodeServerCarcass(*g.ytsaurus.Spec.TabletNodes)
	if err != nil {
		return nil, err
	}

	g.fillCommonService(&c.CommonServer)

	return marshallYsonConfig(c)
}

func (g *Generator) GetHttpProxyConfig() ([]byte, error) {
	c, err := getHttpProxyServerCarcass(g.ytsaurus.Spec.HttpProxies)
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

func (g *Generator) GetYqlAgentConfig() ([]byte, error) {
	if g.ytsaurus.Spec.YqlAgents == nil {
		return []byte{}, nil
	}
	c, err := getYqlAgentServerCarcass(*g.ytsaurus.Spec.YqlAgents)
	if err != nil {
		return nil, err
	}
	g.fillCommonService(&c.CommonServer)
	c.YqlAgent.AdditionalClusters = map[string]string{"yt": g.GetHttpProxiesServiceAddress()}

	return marshallYsonConfig(c)
}

func (g *Generator) GetWebUiConfig() ([]byte, error) {
	if g.ytsaurus.Spec.UI == nil {
		return []byte{}, nil
	}

	c := getUiClusterCarcass()
	c.Id = g.ytsaurus.Name
	c.Name = g.ytsaurus.Name
	c.Proxy = g.GetHttpProxiesAddress()
	c.PrimaryMaster.CellTag = g.ytsaurus.Spec.CellTag

	return marshallJsonConfig(WebUi{Clusters: []UiCluster{c}})
}

func (g *Generator) GetDiscoveryConfig() ([]byte, error) {
	c := getDiscoveryServerCarcass(g.ytsaurus.Spec.Discovery)
	c.DiscoveryServer.Addresses = g.getDiscoveryAddresses()

	return marshallYsonConfig(c)
}
