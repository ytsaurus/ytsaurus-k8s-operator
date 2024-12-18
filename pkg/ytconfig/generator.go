package ytconfig

import (
	"fmt"
	"path"

	"k8s.io/utils/ptr"

	"go.ytsaurus.tech/yt/go/yson"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type ConfigFormat string

const (
	ConfigFormatYson               = "yson"
	ConfigFormatJson               = "json"
	ConfigFormatJsonWithJsPrologue = "json_with_js_prologue"
	ConfigFormatToml               = "toml"
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

type NodeGenerator struct {
	baseLabeller *labeller.Labeller

	commonSpec           *ytv1.CommonSpec
	masterConnectionSpec *ytv1.MasterConnectionSpec
	masterCachesSpec     *ytv1.MasterCachesSpec

	masterInstanceCount    int32
	discoveryInstanceCount int32
	dataNodesInstanceCount int32
}

type Generator struct {
	NodeGenerator

	ytsaurus *ytv1.Ytsaurus
}

func NewGenerator(ytsaurus *ytv1.Ytsaurus, clusterDomain string) *Generator {
	return &Generator{
		NodeGenerator: *NewLocalNodeGenerator(ytsaurus, ytsaurus.Name, clusterDomain),
		ytsaurus:      ytsaurus,
	}
}

func NewLocalNodeGenerator(ytsaurus *ytv1.Ytsaurus, resourceName string, clusterDomain string) *NodeGenerator {
	var dataNodesInstanceCount int32
	for _, dataNodes := range ytsaurus.Spec.DataNodes {
		dataNodesInstanceCount += dataNodes.InstanceCount
	}

	return &NodeGenerator{
		baseLabeller: &labeller.Labeller{
			Namespace:     ytsaurus.GetNamespace(),
			ClusterName:   ytsaurus.GetName(),
			ResourceName:  resourceName,
			ClusterDomain: clusterDomain,
			Annotations:   ytsaurus.Spec.ExtraPodAnnotations,
			UseShortNames: ytsaurus.Spec.UseShortNames,
		},
		commonSpec:             &ytsaurus.Spec.CommonSpec,
		masterConnectionSpec:   &ytsaurus.Spec.PrimaryMasters.MasterConnectionSpec,
		masterInstanceCount:    ytsaurus.Spec.PrimaryMasters.InstanceCount,
		discoveryInstanceCount: ytsaurus.Spec.Discovery.InstanceCount,
		masterCachesSpec:       ytsaurus.Spec.MasterCaches,
		dataNodesInstanceCount: dataNodesInstanceCount,
	}
}

func NewRemoteNodeGenerator(ytsaurus *ytv1.RemoteYtsaurus, resourceName string, clusterDomain string, commonSpec *ytv1.CommonSpec) *NodeGenerator {
	return &NodeGenerator{
		baseLabeller: &labeller.Labeller{
			Namespace:     ytsaurus.GetNamespace(),
			ClusterName:   ytsaurus.GetName(),
			ResourceName:  resourceName,
			ClusterDomain: clusterDomain,
			Annotations:   commonSpec.ExtraPodAnnotations,
			UseShortNames: commonSpec.UseShortNames,
		},
		commonSpec:           commonSpec,
		masterConnectionSpec: &ytsaurus.Spec.MasterConnectionSpec,
		masterCachesSpec:     &ytsaurus.Spec.MasterCachesSpec,
	}
}

func (g *NodeGenerator) GetComponentLabeller(component consts.ComponentType, instanceGroup string) *labeller.Labeller {
	return g.baseLabeller.ForComponent(component, instanceGroup)
}

func (g *NodeGenerator) getComponentAddresses(ct consts.ComponentType, instanceCount int32, port int) []string {
	labeller := g.GetComponentLabeller(ct, "")
	addresses := make([]string, instanceCount)
	for i := range int(instanceCount) {
		addresses[i] = labeller.GetInstanceAddressPort(i, port)
	}
	return addresses
}

func (g *NodeGenerator) getMasterAddresses() []string {
	if hosts := g.masterConnectionSpec.HostAddresses; len(hosts) != 0 {
		addresses := make([]string, len(hosts))
		for idx, host := range hosts {
			addresses[idx] = fmt.Sprintf("%s:%d", host, consts.MasterRPCPort)
		}
		return addresses
	}
	return g.getComponentAddresses(consts.MasterType, g.masterInstanceCount, consts.MasterRPCPort)
}

func (g *NodeGenerator) getMasterCachesAddresses() []string {
	if g.masterCachesSpec == nil {
		return nil
	}
	if hosts := g.masterCachesSpec.HostAddresses; len(hosts) != 0 {
		addresses := make([]string, len(hosts))
		for idx, host := range hosts {
			addresses[idx] = fmt.Sprintf("%s:%d", host, consts.MasterCachesRPCPort)
		}
		return addresses
	}
	return g.getComponentAddresses(consts.MasterCacheType, g.masterCachesSpec.InstanceCount, consts.MasterCachesRPCPort)
}

func (g *NodeGenerator) getMasterHydraPeers() []HydraPeer {
	peers := make([]HydraPeer, 0, g.masterInstanceCount)
	for _, address := range g.getMasterAddresses() {
		peers = append(peers, HydraPeer{
			Address: address,
			Voting:  true,
		})
	}
	return peers
}

func (g *NodeGenerator) getDiscoveryAddresses() []string {
	return g.getComponentAddresses(consts.DiscoveryType, g.discoveryInstanceCount, consts.DiscoveryRPCPort)
}

func (g *NodeGenerator) GetMaxReplicationFactor() int32 {
	return g.dataNodesInstanceCount
}

func (g *Generator) GetHTTPProxiesServiceName(role string) string {
	return g.GetComponentLabeller(consts.HttpProxyType, role).GetBalancerServiceName()
}

func (g *Generator) GetHTTPProxiesServiceAddress(role string) string {
	return g.GetComponentLabeller(consts.HttpProxyType, role).GetHeadlessServiceAddress()
}

func (g *NodeGenerator) GetHTTPProxiesAddress(role string) string {
	return g.GetComponentLabeller(consts.HttpProxyType, role).GetBalancerServiceAddress()
}

func (g *Generator) GetRPCProxiesServiceName(role string) string {
	return g.GetComponentLabeller(consts.RpcProxyType, role).GetBalancerServiceName()
}

func (g *Generator) GetTCPProxiesServiceName(role string) string {
	return g.GetComponentLabeller(consts.TcpProxyType, role).GetBalancerServiceName()
}

func (g *NodeGenerator) GetStrawberryControllerServiceAddress() string {
	return g.GetComponentLabeller(consts.StrawberryControllerType, "").GetHeadlessServiceAddress()
}

func (g *Generator) GetYQLAgentAddresses() []string {
	return g.getComponentAddresses(consts.YqlAgentType, g.ytsaurus.Spec.YQLAgents.InstanceCount, consts.YQLAgentRPCPort)
}

func (g *Generator) GetQueryTrackerAddresses() []string {
	return g.getComponentAddresses(consts.QueryTrackerType, g.ytsaurus.Spec.QueryTrackers.InstanceCount, consts.QueryTrackerRPCPort)
}

func (g *Generator) GetQueueAgentAddresses() []string {
	return g.getComponentAddresses(consts.QueueAgentType, g.ytsaurus.Spec.QueueAgents.InstanceCount, consts.QueueAgentRPCPort)
}

func (g *NodeGenerator) fillIOEngine(ioEngine **IOEngine) {
	if g.commonSpec.EphemeralCluster {
		if *ioEngine == nil {
			*ioEngine = &IOEngine{}
		}
		(*ioEngine).EnableSync = ptr.To(false)
	}
}

func (g *NodeGenerator) fillDriver(c *Driver) {
	c.TimestampProviders.Addresses = g.getMasterAddresses()

	c.PrimaryMaster.Addresses = g.getMasterAddresses()
	c.PrimaryMaster.CellID = generateCellID(g.masterConnectionSpec.CellTag)
	g.fillPrimaryMaster(&c.PrimaryMaster)
}

func (g *NodeGenerator) fillAddressResolver(c *AddressResolver) {
	var retries = 1000
	c.EnableIPv4 = g.commonSpec.UseIPv4
	c.EnableIPv6 = g.commonSpec.UseIPv6
	c.KeepSocket = g.commonSpec.KeepSocket
	c.ForceTCP = g.commonSpec.ForceTCP

	if !c.EnableIPv6 && !c.EnableIPv4 {
		// In case when nothing is specified, we prefer IPv4 due to compatibility reasons.
		c.EnableIPv4 = true
	}
	c.Retries = &retries
}

func (g *NodeGenerator) fillSolomonExporter(c *SolomonExporter) {
	c.Host = ptr.To("{POD_SHORT_HOSTNAME}")
	c.InstanceTags = map[string]string{
		"pod": "{K8S_POD_NAME}",
	}
}

func (g *NodeGenerator) fillPrimaryMaster(c *MasterCell) {
	c.Addresses = g.getMasterAddresses()
	c.Peers = g.getMasterHydraPeers()
	c.CellID = generateCellID(g.masterConnectionSpec.CellTag)
}

func (g *NodeGenerator) fillClusterConnection(c *ClusterConnection, s *ytv1.RPCTransportSpec) {
	g.fillPrimaryMaster(&c.PrimaryMaster)
	c.ClusterName = g.baseLabeller.GetClusterName()
	c.DiscoveryConnection.Addresses = g.getDiscoveryAddresses()
	g.fillClusterConnectionEncryption(c, s)
	c.MasterCache.Addresses = g.getMasterCachesAddresses()
	if len(c.MasterCache.Addresses) == 0 {
		c.MasterCache.Addresses = c.PrimaryMaster.Addresses
	}
	c.MasterCache.CellID = generateCellID(g.masterConnectionSpec.CellTag)
}

func (g *NodeGenerator) fillCypressAnnotations(c *CommonServer) {
	c.CypressAnnotations = map[string]any{
		"k8s_pod_name":      "{K8S_POD_NAME}",
		"k8s_pod_namespace": "{K8S_POD_NAMESPACE}",
		"k8s_node_name":     "{K8S_NODE_NAME}",
		// for CMS and UI сompatibility
		"physical_host": "{K8S_NODE_NAME}",
	}
}

func (g *NodeGenerator) fillCommonService(c *CommonServer, s *ytv1.InstanceSpec) {
	// ToDo(psushin): enable porto resource tracker?
	g.fillAddressResolver(&c.AddressResolver)
	g.fillSolomonExporter(&c.SolomonExporter)
	g.fillClusterConnection(&c.ClusterConnection, s.NativeTransport)
	g.fillCypressAnnotations(c)
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

func (g *NodeGenerator) fillBusServer(c *CommonServer, s *ytv1.RPCTransportSpec) {
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

func (g *NodeGenerator) fillClusterConnectionEncryption(c *ClusterConnection, s *ytv1.RPCTransportSpec) {
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

func (g *Generator) fillStrawberryControllerFamiliesConfig(c *StrawberryControllerFamiliesConfig, s *ytv1.StrawberryControllerSpec) {
	if s.ControllerFamilies != nil {
		c.ControllerFamilies = s.ControllerFamilies
	} else {
		c.ControllerFamilies = consts.GetDefaultStrawberryControllerFamilies()
	}
	if s.DefaultRouteFamily != nil {
		c.DefaultRouteFamily = *s.DefaultRouteFamily
	} else {
		c.DefaultRouteFamily = consts.DefaultStrawberryControllerFamily
	}
	c.ExternalProxy = s.ExternalProxy
}

func (g *Generator) GetStrawberryControllerConfig() ([]byte, error) {
	var resolver AddressResolver
	g.fillAddressResolver(&resolver)

	var conFamConfig StrawberryControllerFamiliesConfig
	g.fillStrawberryControllerFamiliesConfig(&conFamConfig, g.ytsaurus.Spec.StrawberryController)

	c, err := getStrawberryController(conFamConfig, resolver)
	if err != nil {
		return nil, err
	}
	proxy := g.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)
	c.LocationProxies = []string{proxy}
	c.HTTPLocationAliases = map[string][]string{
		proxy: {g.ytsaurus.Name},
	}
	return marshallYsonConfig(c)
}

func (g *Generator) GetStrawberryInitClusterConfig() ([]byte, error) {
	var conFamConfig StrawberryControllerFamiliesConfig
	g.fillStrawberryControllerFamiliesConfig(&conFamConfig, g.ytsaurus.Spec.StrawberryController)
	c := getStrawberryInitCluster(conFamConfig)
	c.Proxy = g.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)
	return marshallYsonConfig(c)
}

func (g *Generator) getMasterConfigImpl(spec *ytv1.MastersSpec) (MasterServer, error) {
	c, err := getMasterServerCarcass(spec)
	if err != nil {
		return MasterServer{}, err
	}
	g.fillIOEngine(&c.Changelogs.IOEngine)
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	g.fillPrimaryMaster(&c.PrimaryMaster)
	configureMasterServerCypressManager(g.GetMaxReplicationFactor(), &c.CypressManager)

	// COMPAT(l0kix2): remove that after we drop support for specifying host network without master host addresses.
	if ptr.Deref(spec.HostNetwork, g.ytsaurus.Spec.HostNetwork) && len(spec.HostAddresses) == 0 {
		// Each master deduces its index within cell by looking up his FQDN in the
		// list of all master peers. Master peers are specified using their pod addresses,
		// therefore we must also switch masters from identifying themselves by FQDN addresses
		// to their pod addresses.

		// POD_NAME is set to pod name through downward API env var and substituted during
		// config postprocessing.
		l := g.GetComponentLabeller(consts.MasterType, "")
		c.AddressResolver.LocalhostNameOverride = ptr.To(
			fmt.Sprintf("%s.%s.%s.svc.%s",
				"{K8S_POD_NAME}",
				l.GetHeadlessServiceName(),
				l.GetNamespace(),
				l.GetClusterDomain()))
	}

	return c, nil
}

func (g *Generator) GetMasterConfig(spec *ytv1.MastersSpec) ([]byte, error) {
	c, err := g.getMasterConfigImpl(spec)
	if err != nil {
		return nil, err
	}
	return marshallYsonConfig(c)
}

func (g *NodeGenerator) GetNativeClientConfig() ([]byte, error) {
	c, err := getNativeClientCarcass()
	if err != nil {
		return nil, err
	}

	g.fillDriver(&c.Driver)
	g.fillAddressResolver(&c.AddressResolver)
	c.Driver.APIVersion = 4

	return marshallYsonConfig(c)
}

func (g *Generator) getSchedulerConfigImpl(spec *ytv1.SchedulersSpec) (SchedulerServer, error) {
	c, err := getSchedulerServerCarcass(spec)
	if err != nil {
		return SchedulerServer{}, err
	}

	if len(g.ytsaurus.Spec.TabletNodes) == 0 {
		c.Scheduler.OperationsCleaner.EnableOperationArchivation = ptr.To(false)
	}
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	return c, nil
}

func (g *Generator) GetSchedulerConfig(spec *ytv1.SchedulersSpec) ([]byte, error) {
	if spec == nil {
		return []byte{}, nil
	}

	c, err := g.getSchedulerConfigImpl(spec)
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
		c.OauthService = ptr.To(getOauthService(*g.ytsaurus.Spec.OauthService))
		c.CypressUserManager = CypressUserManager{}
		c.OauthTokenAuthenticator = &OauthTokenAuthenticator{}
		c.RequireAuthentication = ptr.To(true)
	}

	return c, nil
}

func getOauthService(oauthServiceSpec ytv1.OauthServiceSpec) OauthService {
	var loginTransformations []LoginTransformation
	for _, tr := range oauthServiceSpec.UserInfo.LoginTransformations {
		loginTransformations = append(loginTransformations, LoginTransformation{
			MatchPattern: tr.MatchPattern,
			Replacement:  tr.Replacement,
		})
	}

	return OauthService{
		Host:                 oauthServiceSpec.Host,
		Port:                 oauthServiceSpec.Port,
		Secure:               oauthServiceSpec.Secure,
		UserInfoEndpoint:     oauthServiceSpec.UserInfo.Endpoint,
		UserInfoLoginField:   oauthServiceSpec.UserInfo.LoginField,
		UserInfoErrorField:   oauthServiceSpec.UserInfo.ErrorField,
		LoginTransformations: loginTransformations,
	}
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

func (g *Generator) getControllerAgentConfigImpl(spec *ytv1.ControllerAgentsSpec) (ControllerAgentServer, error) {
	c, err := getControllerAgentServerCarcass(spec)
	if err != nil {
		return ControllerAgentServer{}, err
	}

	c.ControllerAgent.EnableTmpfs = true
	c.ControllerAgent.UseColumnarStatisticsDefault = true

	c.ControllerAgent.AlertManager.LowCpuUsageAlertStatisics = []string{
		"/job/cpu/system",
		"/job/cpu/user",
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)

	return c, nil
}

func (g *Generator) GetControllerAgentConfig(spec *ytv1.ControllerAgentsSpec) ([]byte, error) {
	if spec == nil {
		return []byte{}, nil
	}

	c, err := g.getControllerAgentConfigImpl(spec)
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

	for i := range c.DataNode.StoreLocations {
		g.fillIOEngine(&c.DataNode.StoreLocations[i].IOEngine)
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
	c, err := getExecNodeServerCarcass(spec, g.commonSpec)
	if err != nil {
		return c, err
	}
	for i := range c.DataNode.CacheLocations {
		g.fillIOEngine(&c.DataNode.CacheLocations[i].IOEngine)
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
		c.Auth.OauthService = ptr.To(getOauthService(*g.ytsaurus.Spec.OauthService))
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

func (g *Generator) getQueryTrackerConfigImpl(spec *ytv1.QueryTrackerSpec) (QueryTrackerServer, error) {
	c, err := getQueryTrackerServerCarcass(spec)
	if err != nil {
		return c, err
	}
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)

	return c, nil
}

func (g *Generator) GetQueryTrackerConfig(spec *ytv1.QueryTrackerSpec) ([]byte, error) {
	if spec == nil {
		return []byte{}, nil
	}

	c, err := g.getQueryTrackerConfigImpl(spec)
	if err != nil {
		return nil, err
	}

	return marshallYsonConfig(c)
}

func (g *Generator) getQueueAgentConfigImpl(spec *ytv1.QueueAgentSpec) (QueueAgentServer, error) {
	c, err := getQueueAgentServerCarcass(spec)
	if err != nil {
		return c, err
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)

	return c, nil
}

func (g *Generator) GetQueueAgentConfig(spec *ytv1.QueueAgentSpec) ([]byte, error) {
	if spec == nil {
		return []byte{}, nil
	}

	c, err := g.getQueueAgentConfigImpl(spec)
	if err != nil {
		return nil, err
	}

	return marshallYsonConfig(c)
}

func (g *Generator) getYQLAgentConfigImpl(spec *ytv1.YQLAgentSpec) (YQLAgentServer, error) {
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

func (g *Generator) GetYQLAgentConfig(spec *ytv1.YQLAgentSpec) ([]byte, error) {
	if spec == nil {
		return []byte{}, nil
	}
	c, err := g.getYQLAgentConfigImpl(spec)
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
	c.Secure = g.ytsaurus.Spec.UI.Secure
	c.ExternalProxy = g.ytsaurus.Spec.UI.ExternalProxy
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

func (g *Generator) GetUICustomSettings() *UICustomSettings {
	directDownload := g.ytsaurus.Spec.UI.DirectDownload
	if directDownload == nil {
		return nil
	}
	return &UICustomSettings{
		DirectDownload: directDownload,
	}
}

func (g *Generator) GetUICustomConfig() ([]byte, error) {
	if g.ytsaurus.Spec.UI == nil {
		return []byte{}, nil
	}

	c := UICustom{
		OdinBaseUrl: g.ytsaurus.Spec.UI.OdinBaseUrl,
		Settings:    g.GetUICustomSettings(),
	}

	return marshallYsonConfig(c)
}

func (g *Generator) getDiscoveryConfigImpl(spec *ytv1.DiscoverySpec) (DiscoveryServer, error) {
	c, err := getDiscoveryServerCarcass(spec)
	if err != nil {
		return c, err
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	c.DiscoveryServer.Addresses = g.getDiscoveryAddresses()
	return c, nil
}

func (g *Generator) GetDiscoveryConfig(spec *ytv1.DiscoverySpec) ([]byte, error) {
	c, err := g.getDiscoveryConfigImpl(spec)
	if err != nil {
		return nil, err
	}
	return marshallYsonConfig(c)
}

func (g *Generator) getMasterCachesConfigImpl(spec *ytv1.MasterCachesSpec) (MasterCacheServer, error) {
	c, err := getMasterCachesCarcass(spec)
	if err != nil {
		return MasterCacheServer{}, err
	}
	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	return c, nil
}

func (g *Generator) GetMasterCachesConfig(spec *ytv1.MasterCachesSpec) ([]byte, error) {
	if spec == nil {
		return []byte{}, nil
	}
	c, err := g.getMasterCachesConfigImpl(spec)
	if err != nil {
		return nil, err
	}
	return marshallYsonConfig(c)
}
