package ytconfig

import (
	"fmt"
	"path"

	"go.ytsaurus.tech/yt/go/yson"
	corev1 "k8s.io/api/core/v1"
	ptr "k8s.io/utils/pointer"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

type ConfigFormat string

const (
	ConfigFormatYson               = "yson"
	ConfigFormatJson               = "json"
	ConfigFormatJsonWithJsPrologue = "json_with_js_prologue"
)

type YsonGeneratorFunc func() ([]byte, error)
type GeneratorDescriptor struct {
	// F must generate config in YSON.
	F YsonGeneratorFunc
	// Fmt is the desired serialization format for config map.
	// Note that conversion from YSON to Fmt (if needed) is performed as a very last
	// step of config generation pipeline.
	Fmt ConfigFormat
}

type Generator struct {
	BaseGenerator
	ytsaurus *ytv1.Ytsaurus
}

func NewGenerator(
	ytsaurus *ytv1.Ytsaurus,
	clusterDomain string,
) *Generator {
	baseGenerator := NewLocalBaseGenerator(ytsaurus, clusterDomain)
	return &Generator{
		BaseGenerator: *baseGenerator,
		ytsaurus:      ytsaurus,
	}
}
func (g *BaseGenerator) getMasterPodFqdnSuffix() string {
	return fmt.Sprintf("%s.%s.svc.%s",
		g.GetMastersServiceName(),
		g.key.Namespace,
		g.clusterDomain)
}

func (g *BaseGenerator) getMasterAddresses() []string {
	hosts := g.masterConnectionSpec.HostAddresses

	if len(hosts) == 0 {
		masterPodSuffix := g.getMasterPodFqdnSuffix()
		for _, podName := range g.GetMasterPodNames() {
			hosts = append(hosts, fmt.Sprintf("%s.%s",
				podName,
				masterPodSuffix,
			))
		}
	}

	addresses := make([]string, len(hosts))
	for idx, host := range hosts {
		addresses[idx] = fmt.Sprintf("%s:%d", host, consts.MasterRPCPort)
	}
	return addresses
}

func (g *BaseGenerator) getMasterHydraPeers() []HydraPeer {
	peers := make([]HydraPeer, 0, g.masterInstanceCount)
	for _, address := range g.getMasterAddresses() {
		peers = append(peers, HydraPeer{
			Address: address,
			Voting:  true,
		})
	}
	return peers
}

func (g *BaseGenerator) getDiscoveryAddresses() []string {
	names := make([]string, 0, g.discoveryInstanceCount)
	for _, podName := range g.GetDiscoveryPodNames() {
		names = append(names, fmt.Sprintf("%s.%s.%s.svc.%s:%d",
			podName,
			g.GetDiscoveryServiceName(),
			g.key.Namespace,
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

func (g *Generator) GetQueueAgentAddresses() []string {
	names := make([]string, 0, g.ytsaurus.Spec.QueueAgents.InstanceCount)
	for _, podName := range g.GetQueueAgentPodNames() {
		names = append(names, fmt.Sprintf("%s.%s.%s.svc.%s:%d",
			podName,
			g.GetQueueAgentServiceName(),
			g.ytsaurus.Namespace,
			g.clusterDomain,
			consts.QueueAgentRPCPort))
	}
	return names
}

func (g *Generator) fillDriver(c *Driver) {
	c.TimestampProviders.Addresses = g.getMasterAddresses()

	c.PrimaryMaster.Addresses = g.getMasterAddresses()
	c.PrimaryMaster.CellID = generateCellID(g.ytsaurus.Spec.PrimaryMasters.CellTag)
	g.fillPrimaryMaster(&c.PrimaryMaster)
}

func (g *BaseGenerator) fillAddressResolver(c *AddressResolver) {
	var retries = 1000

	c.EnableIPv4 = g.commonSpec.UseIPv4
	c.EnableIPv6 = g.commonSpec.UseIPv6
	if !c.EnableIPv6 && !c.EnableIPv4 {
		// In case when nothing is specified, we prefer IPv4 due to compatibility reasons.
		c.EnableIPv4 = true
	}
	c.Retries = &retries
}

func (g *BaseGenerator) fillPrimaryMaster(c *MasterCell) {
	c.Addresses = g.getMasterAddresses()
	c.Peers = g.getMasterHydraPeers()
	c.CellID = generateCellID(g.masterConnectionSpec.CellTag)
}

func (g *BaseGenerator) fillClusterConnection(c *ClusterConnection, s *ytv1.RPCTransportSpec) {
	g.fillPrimaryMaster(&c.PrimaryMaster)
	c.ClusterName = g.key.Name
	c.DiscoveryConnection.Addresses = g.getDiscoveryAddresses()
	g.fillClusterConnectionEncryption(c, s)
	if len(g.getMasterCachesAddresses()) == 0 {
		c.MasterCache.Addresses = g.getMasterAddresses()
	} else {
		c.MasterCache.Addresses = g.getMasterCachesAddresses()
	}
	c.MasterCache.CellID = generateCellID(g.masterConnectionSpec.CellTag)
}

func (g *BaseGenerator) fillCypressAnnotations(c *map[string]any) {
	*c = map[string]any{
		"k8s_pod_name":      "{K8S_POD_NAME}",
		"k8s_pod_namespace": "{K8S_POD_NAMESPACE}",
		"k8s_node_name":     "{K8S_NODE_NAME}",
	}
}

func (g *BaseGenerator) fillCommonService(c *CommonServer, s *ytv1.InstanceSpec) {
	// ToDo(psushin): enable porto resource tracker?
	g.fillAddressResolver(&c.AddressResolver)
	g.fillClusterConnection(&c.ClusterConnection, s.NativeTransport)
	g.fillCypressAnnotations(&c.CypressAnnotations)
	c.TimestampProviders.Addresses = g.getMasterAddresses()
}

func (g *Generator) fillBusEncryption(b *Bus, s *ytv1.RPCTransportSpec) {
	if s.TLSRequired {
		b.EncryptionMode = EncryptionModeRequired
	} else {
		b.EncryptionMode = EncryptionModeOptional
	}

	b.CertChain = &PemBlob{
		FileName: path.Join(consts.RPCSecretMountPoint, corev1.TLSCertKey),
	}
	b.PrivateKey = &PemBlob{
		FileName: path.Join(consts.RPCSecretMountPoint, corev1.TLSPrivateKeyKey),
	}
}

func (g *BaseGenerator) fillBusServer(c *CommonServer, s *ytv1.RPCTransportSpec) {
	if s == nil {
		// Use common bus transport config
		s = g.commonSpec.NativeTransport
	}
	if s == nil || s.TLSSecret == nil {
		return
	}

	if c.BusServer == nil {
		c.BusServer = &BusServer{}
	}

	// FIXME(khlebnikov): some clients does not support TLS yet
	if s.TLSRequired && s != g.commonSpec.NativeTransport {
		c.BusServer.EncryptionMode = EncryptionModeRequired
	} else {
		c.BusServer.EncryptionMode = EncryptionModeOptional
	}

	c.BusServer.CertChain = &PemBlob{
		FileName: path.Join(consts.BusSecretMountPoint, corev1.TLSCertKey),
	}
	c.BusServer.PrivateKey = &PemBlob{
		FileName: path.Join(consts.BusSecretMountPoint, corev1.TLSPrivateKeyKey),
	}
}

func (g *BaseGenerator) fillClusterConnectionEncryption(c *ClusterConnection, s *ytv1.RPCTransportSpec) {
	if s == nil {
		// Use common bus transport config
		s = g.commonSpec.NativeTransport
	}
	if s == nil || s.TLSSecret == nil {
		return
	}

	if c.BusClient == nil {
		c.BusClient = &Bus{}
	}

	if g.commonSpec.CABundle != nil {
		c.BusClient.CA = &PemBlob{
			FileName: path.Join(consts.CABundleMountPoint, consts.CABundleFileName),
		}
	} else {
		c.BusClient.CA = &PemBlob{
			FileName: consts.DefaultCABundlePath,
		}
	}

	if s.TLSRequired {
		c.BusClient.EncryptionMode = EncryptionModeRequired
	} else {
		c.BusClient.EncryptionMode = EncryptionModeOptional
	}

	if s.TLSInsecure {
		c.BusClient.VerificationMode = VerificationModeNone
	} else {
		c.BusClient.VerificationMode = VerificationModeFull
	}

	if s.TLSPeerAlternativeHostName != "" {
		c.BusClient.PeerAlternativeHostName = s.TLSPeerAlternativeHostName
	}
}

func marshallYsonConfig(c interface{}) ([]byte, error) {
	result, err := yson.MarshalFormat(c, yson.FormatPretty)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (g *Generator) GetClusterConnection() ([]byte, error) {
	var c ClusterConnection
	g.fillClusterConnection(&c, nil)
	return marshallYsonConfig(c)
}

func (g *Generator) GetStrawberryControllerConfig() ([]byte, error) {
	c := getStrawberryController()
	proxy := g.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)
	c.LocationProxies = []string{proxy}
	c.HTTPLocationAliases = map[string][]string{
		proxy: []string{g.ytsaurus.Name},
	}
	return marshallYsonConfig(c)
}

func (g *Generator) GetChytInitClusterConfig() ([]byte, error) {
	c := getChytInitCluster()
	c.Proxy = g.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)
	return marshallYsonConfig(c)
}

func (g *Generator) getMasterConfigImpl() (MasterServer, error) {
	spec := &g.ytsaurus.Spec.PrimaryMasters
	c, err := getMasterServerCarcass(spec)
	if err != nil {
		return MasterServer{}, err
	}
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	g.fillPrimaryMaster(&c.PrimaryMaster)
	configureMasterServerCypressManager(g.ytsaurus.Spec, &c.CypressManager)

	// COMPAT(l0kix2): remove that after we drop support for specifying host network without master host addresses.
	if g.ytsaurus.Spec.HostNetwork && len(spec.HostAddresses) == 0 {
		// Each master deduces its index within cell by looking up his FQDN in the
		// list of all master peers. Master peers are specified using their pod addresses,
		// therefore we must also switch masters from identifying themselves by FQDN addresses
		// to their pod addresses.

		// POD_NAME is set to pod name through downward API env var and substituted during
		// config postprocessing.
		c.AddressResolver.LocalhostNameOverride = ptr.String(
			fmt.Sprintf("%v.%v", "{K8S_POD_NAME}", g.getMasterPodFqdnSuffix()))
	}

	return c, nil
}

func (g *Generator) GetMasterConfig() ([]byte, error) {
	c, err := g.getMasterConfigImpl()
	if err != nil {
		return nil, err
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

func (g *Generator) getSchedulerConfigImpl() (SchedulerServer, error) {
	spec := g.ytsaurus.Spec.Schedulers
	c, err := getSchedulerServerCarcass(spec)
	if err != nil {
		return SchedulerServer{}, err
	}

	if g.ytsaurus.Spec.TabletNodes == nil || len(g.ytsaurus.Spec.TabletNodes) == 0 {
		c.Scheduler.OperationsCleaner.EnableOperationArchivation = ptr.Bool(false)
	}
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	return c, nil
}

func (g *Generator) GetSchedulerConfig() ([]byte, error) {
	if g.ytsaurus.Spec.Schedulers == nil {
		return []byte{}, nil
	}

	c, err := g.getSchedulerConfigImpl()
	if err != nil {
		return nil, err
	}

	return marshallYsonConfig(c)
}

func (g *Generator) getRPCProxyConfigImpl(spec *ytv1.RPCProxiesSpec) (RPCProxyServer, error) {
	c, err := getRPCProxyServerCarcass(spec)
	if err != nil {
		return RPCProxyServer{}, err
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)

	if g.ytsaurus.Spec.OauthService != nil {
		c.CypressUserManager = CypressUserManager{}
		c.OauthService = &OauthService{
			Host:               g.ytsaurus.Spec.OauthService.Host,
			Port:               g.ytsaurus.Spec.OauthService.Port,
			Secure:             g.ytsaurus.Spec.OauthService.Secure,
			UserInfoEndpoint:   g.ytsaurus.Spec.OauthService.UserInfo.Endpoint,
			UserInfoLoginField: g.ytsaurus.Spec.OauthService.UserInfo.LoginField,
			UserInfoErrorField: g.ytsaurus.Spec.OauthService.UserInfo.ErrorField,
		}
		c.OauthTokenAuthenticator = &OauthTokenAuthenticator{}
		c.RequireAuthentication = true
	}

	return c, nil
}

func (g *Generator) GetRPCProxyConfig(spec ytv1.RPCProxiesSpec) ([]byte, error) {
	c, err := g.getRPCProxyConfigImpl(&spec)
	if err != nil {
		return []byte{}, err
	}

	if spec.Transport.TLSSecret != nil {
		if c.BusServer == nil {
			c.BusServer = &BusServer{}
		}
		g.fillBusEncryption(&c.BusServer.Bus, &spec.Transport)
	}

	return marshallYsonConfig(c)
}

func (g *Generator) getTCPProxyConfigImpl(spec *ytv1.TCPProxiesSpec) (TCPProxyServer, error) {
	c, err := getTCPProxyServerCarcass(spec)
	if err != nil {
		return TCPProxyServer{}, err
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)

	return c, nil
}

func (g *Generator) GetTCPProxyConfig(spec ytv1.TCPProxiesSpec) ([]byte, error) {
	if g.ytsaurus.Spec.TCPProxies == nil {
		return []byte{}, nil
	}

	c, err := g.getTCPProxyConfigImpl(&spec)
	if err != nil {
		return []byte{}, err
	}

	return marshallYsonConfig(c)
}

func (g *Generator) getControllerAgentConfigImpl() (ControllerAgentServer, error) {
	spec := g.ytsaurus.Spec.ControllerAgents
	c, err := getControllerAgentServerCarcass(spec)
	if err != nil {
		return ControllerAgentServer{}, err
	}

	c.ControllerAgent.EnableTmpfs = g.ytsaurus.Spec.UsePorto
	c.ControllerAgent.UseColumnarStatisticsDefault = true

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)

	return c, nil
}

func (g *Generator) GetControllerAgentConfig() ([]byte, error) {
	if g.ytsaurus.Spec.ControllerAgents == nil {
		return []byte{}, nil
	}

	c, err := g.getControllerAgentConfigImpl()
	if err != nil {
		return []byte{}, err
	}

	return marshallYsonConfig(c)
}

func (g *NodeGenerator) getDataNodeConfigImpl(spec *ytv1.DataNodesSpec) (DataNodeServer, error) {
	c, err := getDataNodeServerCarcass(spec)
	if err != nil {
		return DataNodeServer{}, err
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	return c, nil
}

func (g *NodeGenerator) GetDataNodeConfig(spec ytv1.DataNodesSpec) ([]byte, error) {
	c, err := g.getDataNodeConfigImpl(&spec)
	if err != nil {
		return []byte{}, err
	}
	return marshallYsonConfig(c)
}

func (g *NodeGenerator) getExecNodeConfigImpl(spec *ytv1.ExecNodesSpec) (ExecNodeServer, error) {
	c, err := getExecNodeServerCarcass(
		spec,
		g.commonSpec.UsePorto)
	if err != nil {
		return c, err
	}
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	return c, nil
}

func (g *NodeGenerator) GetExecNodeConfig(spec ytv1.ExecNodesSpec) ([]byte, error) {
	c, err := g.getExecNodeConfigImpl(&spec)
	if err != nil {
		return []byte{}, err
	}
	return marshallYsonConfig(c)
}

func (g *NodeGenerator) getTabletNodeConfigImpl(spec *ytv1.TabletNodesSpec) (TabletNodeServer, error) {
	c, err := getTabletNodeServerCarcass(spec)
	if err != nil {
		return c, err
	}
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	return c, nil
}

func (g *NodeGenerator) GetTabletNodeConfig(spec ytv1.TabletNodesSpec) ([]byte, error) {
	c, err := g.getTabletNodeConfigImpl(&spec)
	if err != nil {
		return nil, err
	}
	return marshallYsonConfig(c)
}

func (g *Generator) getHTTPProxyConfigImpl(spec *ytv1.HTTPProxiesSpec) (HTTPProxyServer, error) {
	c, err := getHTTPProxyServerCarcass(spec)
	if err != nil {
		return c, err
	}

	g.fillDriver(&c.Driver)
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)

	if g.ytsaurus.Spec.OauthService != nil {
		c.Auth.OauthService = &OauthService{
			Host:               g.ytsaurus.Spec.OauthService.Host,
			Port:               g.ytsaurus.Spec.OauthService.Port,
			Secure:             g.ytsaurus.Spec.OauthService.Secure,
			UserInfoEndpoint:   g.ytsaurus.Spec.OauthService.UserInfo.Endpoint,
			UserInfoLoginField: g.ytsaurus.Spec.OauthService.UserInfo.LoginField,
			UserInfoErrorField: g.ytsaurus.Spec.OauthService.UserInfo.ErrorField,
		}
		c.Auth.OauthCookieAuthenticator = &OauthCookieAuthenticator{}
		c.Auth.OauthTokenAuthenticator = &OauthTokenAuthenticator{}
	}

	return c, nil
}

func (g *Generator) GetHTTPProxyConfig(spec ytv1.HTTPProxiesSpec) ([]byte, error) {
	c, err := g.getHTTPProxyConfigImpl(&spec)
	if err != nil {
		return nil, err
	}

	return marshallYsonConfig(c)
}

func (g *Generator) getQueryTrackerConfigImpl() (QueryTrackerServer, error) {
	spec := g.ytsaurus.Spec.QueryTrackers
	c, err := getQueryTrackerServerCarcass(spec)
	if err != nil {
		return c, err
	}
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)

	return c, nil
}

func (g *Generator) GetQueryTrackerConfig() ([]byte, error) {
	if g.ytsaurus.Spec.QueryTrackers == nil {
		return []byte{}, nil
	}

	c, err := g.getQueryTrackerConfigImpl()
	if err != nil {
		return nil, err
	}

	return marshallYsonConfig(c)
}

func (g *Generator) getQueueAgentConfigImpl() (QueueAgentServer, error) {
	spec := g.ytsaurus.Spec.QueueAgents
	c, err := getQueueAgentServerCarcass(spec)
	if err != nil {
		return c, err
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)

	return c, nil
}

func (g *Generator) GetQueueAgentConfig() ([]byte, error) {
	if g.ytsaurus.Spec.QueueAgents == nil {
		return []byte{}, nil
	}

	c, err := g.getQueueAgentConfigImpl()
	if err != nil {
		return nil, err
	}

	return marshallYsonConfig(c)
}

func (g *Generator) getYQLAgentConfigImpl() (YQLAgentServer, error) {
	spec := g.ytsaurus.Spec.YQLAgents
	c, err := getYQLAgentServerCarcass(spec)
	if err != nil {
		return c, err
	}
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)

	c.YQLAgent.GatewayConfig.ClusterMapping = []ClusterMapping{
		{
			Name:    g.ytsaurus.Name,
			Cluster: g.GetHTTPProxiesServiceAddress(consts.DefaultHTTPProxyRole),
			Default: true,
		},
	}

	// For backward compatibility.
	c.YQLAgent.AdditionalClusters = map[string]string{
		g.ytsaurus.Name: g.GetHTTPProxiesServiceAddress(consts.DefaultHTTPProxyRole),
	}
	c.YQLAgent.DefaultCluster = g.ytsaurus.Name

	return c, nil
}

func (g *Generator) GetYQLAgentConfig() ([]byte, error) {
	if g.ytsaurus.Spec.YQLAgents == nil {
		return []byte{}, nil
	}
	c, err := g.getYQLAgentConfigImpl()
	if err != nil {
		return nil, err
	}
	return marshallYsonConfig(c)
}

func (g *Generator) GetUIClustersConfig() ([]byte, error) {
	if g.ytsaurus.Spec.UI == nil {
		return []byte{}, nil
	}

	c := getUIClusterCarcass()
	c.ID = g.ytsaurus.Name
	c.Name = g.ytsaurus.Name
	c.Proxy = g.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)
	c.PrimaryMaster.CellTag = g.ytsaurus.Spec.PrimaryMasters.CellTag

	c.Theme = g.ytsaurus.Spec.UI.Theme
	c.Environment = g.ytsaurus.Spec.UI.Environment
	if g.ytsaurus.Spec.UI.Group != nil {
		c.Group = *g.ytsaurus.Spec.UI.Group
	}
	if g.ytsaurus.Spec.UI.Description != nil {
		c.Description = *g.ytsaurus.Spec.UI.Description
	}

	return marshallYsonConfig(UIClusters{
		Clusters: []UICluster{c},
	})
}

func (g *Generator) GetUICustomConfig() ([]byte, error) {
	if g.ytsaurus.Spec.UI == nil {
		return []byte{}, nil
	}

	c := UICustom{
		OdinBaseUrl: g.ytsaurus.Spec.UI.OdinBaseUrl,
	}

	return marshallYsonConfig(c)
}

func (g *Generator) getDiscoveryConfigImpl() (DiscoveryServer, error) {
	spec := &g.ytsaurus.Spec.Discovery
	c, err := getDiscoveryServerCarcass(spec)
	if err != nil {
		return c, err
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	c.DiscoveryServer.Addresses = g.getDiscoveryAddresses()
	return c, nil
}

func (g *Generator) GetDiscoveryConfig() ([]byte, error) {
	c, err := g.getDiscoveryConfigImpl()
	if err != nil {
		return nil, err
	}
	return marshallYsonConfig(c)
}

func (g *Generator) getMasterCachesConfigImpl() (MasterCacheServer, error) {
	spec := g.ytsaurus.Spec.MasterCaches
	c, err := getMasterCachesCarcass(spec)
	if err != nil {
		return MasterCacheServer{}, err
	}
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	return c, nil
}

func (g *Generator) GetMasterCachesConfig() ([]byte, error) {
	if g.ytsaurus.Spec.MasterCaches == nil {
		return []byte{}, nil
	}
	c, err := g.getMasterCachesConfigImpl()
	if err != nil {
		return nil, err
	}
	return marshallYsonConfig(c)
}

func (g *BaseGenerator) getMasterCachesPodFqdnSuffix() string {
	return fmt.Sprintf("%s.%s.svc.%s",
		g.GetMasterCachesServiceName(),
		g.key.Namespace,
		g.clusterDomain)
}

func (g *BaseGenerator) getMasterCachesAddresses() []string {
	if g.masterCachesSpec != nil {
		hosts := g.masterCachesSpec.HostAddresses
		if len(hosts) == 0 {
			masterCachesPodSuffix := g.getMasterCachesPodFqdnSuffix()
			for _, podName := range g.GetMasterCachesPodNames() {
				hosts = append(hosts, fmt.Sprintf("%s.%s",
					podName,
					masterCachesPodSuffix,
				))
			}
		}
		addresses := make([]string, len(hosts))
		for idx, host := range hosts {
			addresses[idx] = fmt.Sprintf("%s:%d", host, consts.MasterCachesRPCPort)
		}
		return addresses
	}
	return make([]string, 0)
}
