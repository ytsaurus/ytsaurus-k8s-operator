package ytconfig

import (
	"fmt"
	"net"
	"path"
	"strconv"

	"k8s.io/utils/ptr"

	"go.ytsaurus.tech/yt/go/yson"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type NodeGenerator struct {
	baseLabeller *labeller.Labeller

	commonSpec           *ytv1.CommonSpec
	clusterFeatures      ytv1.ClusterFeatures
	masterConnectionSpec *ytv1.MasterConnectionSpec
	masterCachesSpec     *ytv1.MasterCachesSpec
	cypressProxiesSpec   *ytv1.CypressProxiesSpec

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
		clusterFeatures:        ptr.Deref(ytsaurus.Spec.ClusterFeatures, ytv1.ClusterFeatures{}),
		masterConnectionSpec:   &ytsaurus.Spec.PrimaryMasters.MasterConnectionSpec,
		masterInstanceCount:    ytsaurus.Spec.PrimaryMasters.InstanceCount,
		discoveryInstanceCount: ytsaurus.Spec.Discovery.InstanceCount,
		masterCachesSpec:       ytsaurus.Spec.MasterCaches,
		cypressProxiesSpec:     ytsaurus.Spec.CypressProxies,
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
		clusterFeatures:      ptr.Deref(commonSpec.ClusterFeatures, ytv1.ClusterFeatures{}),
		masterConnectionSpec: &ytsaurus.Spec.MasterConnectionSpec,
		masterCachesSpec:     &ytsaurus.Spec.MasterCachesSpec,
	}
}

func (g *NodeGenerator) GetClusterFeatures() ytv1.ClusterFeatures {
	return g.clusterFeatures
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

func (g *NodeGenerator) getCypressProxiesAddresses() []string {
	if g.cypressProxiesSpec == nil || (g.cypressProxiesSpec.Disable != nil && *g.cypressProxiesSpec.Disable) {
		return nil
	}
	return g.getComponentAddresses(consts.CypressProxyType, g.cypressProxiesSpec.InstanceCount, consts.CypressProxyRPCPort)
}

func (g *NodeGenerator) GetMaxReplicationFactor() int32 {
	return g.dataNodesInstanceCount
}

func (g *Generator) GetHTTPProxiesServiceName(role string) string {
	return g.GetComponentLabeller(consts.HttpProxyType, role).GetBalancerServiceName()
}

func getHttpProxyPortOverride(ytsaurus *ytv1.YtsaurusSpec, role string) *int32 {
	for _, p := range ytsaurus.HTTPProxies {
		if p.Role == role {
			return p.HttpPort
		}
	}
	return nil
}

func (g *Generator) GetHTTPProxiesServiceAddress(role string) string {
	port := getHttpProxyPortOverride(&g.ytsaurus.Spec, role)
	if port == nil || *port == consts.HTTPProxyHTTPPort {
		return g.GetComponentLabeller(consts.HttpProxyType, role).GetHeadlessServiceAddress()
	}
	return fmt.Sprintf("%s:%d",
		g.GetComponentLabeller(consts.HttpProxyType, role).GetHeadlessServiceAddress(),
		*port,
	)
}

func (g *NodeGenerator) GetHTTPProxiesAddress(ytsaurus *ytv1.YtsaurusSpec, role string) string {
	port := getHttpProxyPortOverride(ytsaurus, role)
	if port == nil || *port == consts.HTTPProxyHTTPPort {
		return g.GetComponentLabeller(consts.HttpProxyType, role).GetBalancerServiceAddress()
	}
	return fmt.Sprintf("%s:%d",
		g.GetComponentLabeller(consts.HttpProxyType, role).GetBalancerServiceAddress(),
		*port,
	)
}

func (g *Generator) GetHTTPProxiesAddress(role string) string {
	return g.NodeGenerator.GetHTTPProxiesAddress(&g.ytsaurus.Spec, role)
}

func (g *Generator) GetRPCProxiesServiceName(role string) string {
	return g.GetComponentLabeller(consts.RpcProxyType, role).GetBalancerServiceName()
}

func (g *Generator) GetTCPProxiesServiceName(role string) string {
	return g.GetComponentLabeller(consts.TcpProxyType, role).GetBalancerServiceName()
}

func (g *Generator) GetKafkaProxiesServiceName(role string) string {
	return g.GetComponentLabeller(consts.KafkaProxyType, role).GetBalancerServiceName()
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
		"pod": fmt.Sprintf("{%s}", consts.ENV_K8S_POD_NAME),
	}
}

func (g *NodeGenerator) fillPrimaryMaster(c *MasterCell) {
	c.Addresses = g.getMasterAddresses()
	c.Peers = g.getMasterHydraPeers()
	c.CellID = generateCellID(g.masterConnectionSpec.CellTag)
}

func (g *NodeGenerator) fillClusterConnection(c *ClusterConnection, s *ytv1.RPCTransportSpec, keyring *Keyring) {
	g.fillPrimaryMaster(&c.PrimaryMaster)
	c.ClusterName = g.baseLabeller.GetClusterName()
	c.DiscoveryConnection.Addresses = g.getDiscoveryAddresses()
	g.fillClusterConnectionEncryption(c, s, keyring)
	c.MasterCache.Addresses = g.getMasterCachesAddresses()
	if len(c.MasterCache.Addresses) == 0 {
		c.MasterCache.Addresses = c.PrimaryMaster.Addresses
	}
	c.MasterCache.CellID = generateCellID(g.masterConnectionSpec.CellTag)
	cpAddresses := g.getCypressProxiesAddresses()
	if cpAddresses != nil {
		c.CypressProxy = &CypressProxy{
			AddressList{cpAddresses},
		}
	}
}

func (g *NodeGenerator) fillDriverConfig(c *Driver) {
	c.APIVersion = 4

	if g.clusterFeatures.RPCProxyHavePublicAddress {
		c.DefaultRpcProxyAddressType = ptr.To(AddressTypePublicRPC)
	}
}

func (g *NodeGenerator) fillCypressAnnotations(c *CommonServer) {
	c.CypressAnnotations = map[string]any{
		"k8s_pod_name":      fmt.Sprintf("{%s}", consts.ENV_K8S_POD_NAME),
		"k8s_pod_namespace": fmt.Sprintf("{%s}", consts.ENV_K8S_POD_NAMESPACE),
		"k8s_node_name":     fmt.Sprintf("{%s}", consts.ENV_K8S_NODE_NAME),
		// for CMS and UI сompatibility
		"physical_host": fmt.Sprintf("{%s}", consts.ENV_K8S_NODE_NAME),
	}
}

func (g *NodeGenerator) fillCommonService(c *CommonServer, s *ytv1.InstanceSpec) {
	// ToDo(psushin): enable porto resource tracker?
	g.fillAddressResolver(&c.AddressResolver)
	g.fillSolomonExporter(&c.SolomonExporter)
	keyring := getMountKeyring(g.commonSpec, s.NativeTransport)
	g.fillClusterConnection(&c.ClusterConnection, s.NativeTransport, keyring)
	g.fillCypressAnnotations(c)
	c.TimestampProviders.Addresses = g.getMasterAddresses()
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
	keyring := getMountKeyring(g.commonSpec, s)
	fillBusServer(c.BusServer, s, keyring)
}

func fillBusServer(b *BusServer, s *ytv1.RPCTransportSpec, keyring *Keyring) {
	if s.TLSRequired {
		b.EncryptionMode = EncryptionModeRequired
	} else {
		b.EncryptionMode = EncryptionModeOptional
	}

	b.CertChain = keyring.BusServerCertificate
	b.PrivateKey = keyring.BusServerPrivateKey

	if s.TLSRequired && !s.TLSInsecure {
		// Require and verify client certificate.
		b.VerificationMode = VerificationModeFull
		b.CA = keyring.BusCABundle
		b.PeerAlternativeHostName = s.TLSPeerAlternativeHostName
	} else {
		b.VerificationMode = VerificationModeNone
	}
}

func fillChytServer(spec *ytv1.HTTPProxiesSpec, srv *HTTPProxyServer) error {
	chytProxy := ptr.Deref(spec.ChytProxy, ytv1.CHYTProxySpec{})
	httpPort := ptr.Deref(chytProxy.HttpPort, consts.HTTPProxyChytHttpPort)

	srv.ChytHttpServer = &HTTPServer{
		Port: int(httpPort),
	}

	if spec.Transport.HTTPSSecret != nil {
		httpsPort := ptr.Deref(chytProxy.HttpPort, consts.HTTPProxyChytHttpsPort)
		srv.ChytHttpsServer = &HTTPSServer{
			HTTPServer: HTTPServer{
				Port: int(httpsPort),
			},
			Credentials: HTTPSServerCredentials{
				CertChain: PemBlob{
					FileName: path.Join(consts.HTTPSSecretMountPoint, corev1.TLSCertKey),
				},
				PrivateKey: PemBlob{
					FileName: path.Join(consts.HTTPSSecretMountPoint, corev1.TLSPrivateKeyKey),
				},
				UpdatePeriod: yson.Duration(consts.HTTPSSecretUpdatePeriod),
			},
		}
	}
	return nil
}

func (g *NodeGenerator) fillClusterConnectionEncryption(c *ClusterConnection, s *ytv1.RPCTransportSpec, keyring *Keyring) {
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

	if s.TLSRequired || !s.TLSInsecure {
		// Require and verify server certificate.
		c.BusClient.EncryptionMode = EncryptionModeRequired
		c.BusClient.VerificationMode = VerificationModeFull
		c.BusClient.CA = keyring.BusCABundle
		c.BusClient.PeerAlternativeHostName = s.TLSPeerAlternativeHostName
	} else {
		c.BusClient.EncryptionMode = EncryptionModeOptional
		c.BusClient.VerificationMode = VerificationModeNone
	}

	c.BusClient.CertChain = keyring.BusClientCertificate
	c.BusClient.PrivateKey = keyring.BusClientPrivateKey
}

func marshallYsonConfig(c interface{}) ([]byte, error) {
	result, err := yson.MarshalFormat(c, yson.FormatPretty)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (g *Generator) GetClusterConnection() ClusterConnection {
	var c ClusterConnection
	keyring := getVaultKeyring(g.commonSpec, nil)
	g.fillClusterConnection(&c, nil, keyring)
	return c
}

func (g *Generator) GetClusterConnectionConfig() ([]byte, error) {
	c := g.GetClusterConnection()
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

	var busServer *BusServer
	if g.commonSpec.NativeTransport != nil {
		busServer = &BusServer{}
		vaultKeyring := getVaultKeyring(g.commonSpec, nil)
		fillBusServer(busServer, g.commonSpec.NativeTransport, vaultKeyring)
	}

	keyring := getMountKeyring(g.commonSpec, nil)
	c, err := getStrawberryController(conFamConfig, resolver, busServer, keyring)
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

	c.BusClient = c.ClusterConnection.BusClient

	if addresses := g.getDiscoveryAddresses(); len(addresses) > 0 {
		c.DiscoveryServer = &DiscoveryConnection{
			AddressList{
				Addresses: addresses,
			},
		}
	}

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
			fmt.Sprintf("{%s}.%s.%s.svc.%s",
				consts.ENV_K8S_POD_NAME,
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

func getNativeClientConfigCarcass() (NativeClientConfig, error) {
	var c NativeClientConfig

	loggingBuilder := newLoggingBuilder(nil, "client")
	c.Logging = loggingBuilder.logging

	return c, nil
}

func (g *NodeGenerator) GetNativeClientConfig() ([]byte, error) {
	c, err := getNativeClientConfigCarcass()
	if err != nil {
		return nil, err
	}

	keyring := getMountKeyring(g.commonSpec, nil)
	g.fillClusterConnection(&c.Driver.ClusterConnection, nil, keyring)
	g.fillAddressResolver(&c.AddressResolver)
	g.fillDriverConfig(&c.Driver.Driver)

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

	var publicBusServer *BusServer

	if g.clusterFeatures.RPCProxyHavePublicAddress {
		g.fillBusServer(&c.CommonServer, spec.NativeTransport)

		c.PublicRPCPort = consts.RPCProxyPublicRPCPort
		if c.PublicBusServer == nil {
			c.PublicBusServer = &BusServer{}
		}
		publicBusServer = c.PublicBusServer
	} else {
		c.RPCPort = consts.RPCProxyPublicRPCPort

		if spec.Transport.TLSSecret != nil {
			if c.BusServer == nil {
				c.BusServer = &BusServer{}
			}
			publicBusServer = c.BusServer
		}
	}

	if publicBusServer != nil && spec.Transport.TLSSecret != nil {
		if spec.Transport.TLSRequired {
			publicBusServer.EncryptionMode = EncryptionModeRequired
		} else {
			publicBusServer.EncryptionMode = EncryptionModeOptional
		}
		publicBusServer.CertChain = &PemBlob{
			FileName: path.Join(consts.RPCProxySecretMountPoint, corev1.TLSCertKey),
		}
		publicBusServer.PrivateKey = &PemBlob{
			FileName: path.Join(consts.RPCProxySecretMountPoint, corev1.TLSPrivateKeyKey),
		}
	}

	oauthService := g.ytsaurus.Spec.OauthService
	if oauthService != nil {
		c.OauthService = ptr.To(getOauthService(*oauthService))
		c.CypressUserManager = CypressUserManager{}
		var createUserIfNotExist *bool
		if oauthService.DisableUserCreation != nil {
			createUserIfNotExist = ptr.To(!*oauthService.DisableUserCreation)
		}
		c.OauthTokenAuthenticator = &OauthTokenAuthenticator{
			CreateUserIfNotExists: createUserIfNotExist,
		}
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

func (g *Generator) getKafkaProxyConfigImpl(spec *ytv1.KafkaProxiesSpec) (KafkaProxyServer, error) {
	c, err := getKafkaProxyServerCarcass(spec)
	if err != nil {
		return KafkaProxyServer{}, err
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)

	return c, nil
}

func (g *Generator) GetKafkaProxyConfig(spec ytv1.KafkaProxiesSpec) ([]byte, error) {
	if g.ytsaurus.Spec.KafkaProxies == nil {
		return []byte{}, nil
	}

	c, err := g.getKafkaProxyConfigImpl(&spec)
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

	if c.ClusterConnection.BusClient != nil {
		var clusterConnection ClusterConnection

		keyring := getMountKeyring(g.commonSpec, spec.NativeTransport)
		jobProxyKeyring := getVaultKeyring(g.commonSpec, spec.NativeTransport)
		g.fillClusterConnection(&clusterConnection, spec.NativeTransport, jobProxyKeyring)

		forwardSecret := func(node, proxy *PemBlob) {
			if node != nil && proxy != nil {
				c.ExecNode.JobProxy.EnvironmentVariables = append(
					c.ExecNode.JobProxy.EnvironmentVariables,
					EnvironmentVariable{
						Name:     proxy.EnvironmentVariable,
						FileName: &node.FileName,
						Export:   ptr.To(false),
					},
				)
			}
		}

		if c.ClusterConnection.BusClient.VerificationMode != VerificationModeNone {
			forwardSecret(keyring.BusCABundle, jobProxyKeyring.BusCABundle)
		}
		forwardSecret(keyring.BusClientCertificate, jobProxyKeyring.BusClientCertificate)
		forwardSecret(keyring.BusClientPrivateKey, jobProxyKeyring.BusClientPrivateKey)

		c.ExecNode.JobProxy.ClusterConnection = &clusterConnection

		var localAddress string
		if g.commonSpec.UseIPv6 {
			localAddress = net.JoinHostPort("::1", strconv.Itoa(int(c.RPCPort)))
		} else {
			localAddress = net.JoinHostPort("127.0.0.1", strconv.Itoa(int(c.RPCPort)))
		}

		c.ExecNode.JobProxy.SupervisorConnection = &BusClient{
			Address: localAddress,
			Bus:     *clusterConnection.BusClient,
		}
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

func (g *NodeGenerator) getOffshoreNodeProxiesConfigImpl(spec *ytv1.OffshoreNodeProxiesSpec) (OffshoreNodeProxyServer, error) {
	c, err := getOffshoreNodeProxiesCarcass(spec)
	if err != nil {
		return OffshoreNodeProxyServer{}, err
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	c.BusClient = c.ClusterConnection.BusClient
	return c, nil
}

func (g *NodeGenerator) GetOffshoreNodeProxiesConfig(spec ytv1.OffshoreNodeProxiesSpec) ([]byte, error) {
	c, err := g.getOffshoreNodeProxiesConfigImpl(&spec)
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

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)

	g.fillDriverConfig(&c.Driver)
	c.Driver.APIVersion = 0

	oauthService := g.ytsaurus.Spec.OauthService
	if oauthService != nil {
		c.Auth.OauthService = ptr.To(getOauthService(*oauthService))
		var createUserIfNotExist *bool
		if oauthService.DisableUserCreation != nil {
			createUserIfNotExist = ptr.To(!*oauthService.DisableUserCreation)
		}
		c.Auth.OauthCookieAuthenticator = &OauthCookieAuthenticator{
			CreateUserIfNotExists: createUserIfNotExist,
		}
		c.Auth.OauthTokenAuthenticator = &OauthTokenAuthenticator{
			CreateUserIfNotExists: createUserIfNotExist,
		}
	}

	if g.clusterFeatures.HTTPProxyHaveChytAddress {
		err := fillChytServer(spec, &c)
		if err != nil {
			return c, err
		}
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

	c.BusClient = c.ClusterConnection.BusClient

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

func (g *Generator) getBundleControllerConfigImpl(spec *ytv1.BundleControllerSpec) (BundleControllerServer, error) {
	c, err := getBundleControllerServerCarcass(spec)
	if err != nil {
		return BundleControllerServer{}, err
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	c.BundleController.Cluster = g.ytsaurus.Name

	return c, nil
}

func (g *Generator) GetBundleControllerConfig(spec *ytv1.BundleControllerSpec) ([]byte, error) {
	c, err := g.getBundleControllerConfigImpl(spec)
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
	c.BusClient = c.ClusterConnection.BusClient
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
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	c.BusClient = c.ClusterConnection.BusClient
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

func (g *Generator) getCypressProxiesConfigImpl(spec *ytv1.CypressProxiesSpec) (CypressProxyServer, error) {
	c, err := getCypressProxiesCarcass(spec)
	if err != nil {
		return CypressProxyServer{}, err
	}

	g.fillCommonService(&c.CommonServer, &spec.InstanceSpec)
	g.fillBusServer(&c.CommonServer, spec.NativeTransport)
	c.BusClient = c.ClusterConnection.BusClient
	return c, nil
}

func (g *Generator) GetCypressProxiesConfig(spec *ytv1.CypressProxiesSpec) ([]byte, error) {
	if spec == nil {
		return []byte{}, nil
	}
	c, err := g.getCypressProxiesConfigImpl(spec)
	if err != nil {
		return nil, err
	}
	return marshallYsonConfig(c)
}

func (g *Generator) GetComponentNames(component consts.ComponentType) ([]string, error) {
	var names []string
	switch component {
	case consts.ClusterConnectionType:
		names = append(names, "")
	case consts.NativeClientConfigType:
		names = append(names, "")
	case consts.ControllerAgentType:
		if g.ytsaurus.Spec.ControllerAgents != nil {
			names = append(names, "")
		}
	case consts.CypressProxyType:
		if g.ytsaurus.Spec.CypressProxies != nil {
			names = append(names, "")
		}
	case consts.DataNodeType:
		for _, spec := range g.ytsaurus.Spec.DataNodes {
			names = append(names, spec.Name)
		}
	case consts.DiscoveryType:
		names = append(names, "")
	case consts.ExecNodeType:
		for _, spec := range g.ytsaurus.Spec.ExecNodes {
			names = append(names, spec.Name)
		}
	case consts.HttpProxyType:
		for _, spec := range g.ytsaurus.Spec.HTTPProxies {
			names = append(names, spec.Role)
		}
	case consts.MasterCacheType:
		if g.ytsaurus.Spec.MasterCaches != nil {
			names = append(names, "")
		}
	case consts.MasterType:
		names = append(names, "")
	case consts.QueryTrackerType:
		if g.ytsaurus.Spec.QueryTrackers != nil {
			names = append(names, "")
		}
	case consts.QueueAgentType:
		if g.ytsaurus.Spec.QueueAgents != nil {
			names = append(names, "")
		}
	case consts.RpcProxyType:
		for _, spec := range g.ytsaurus.Spec.RPCProxies {
			names = append(names, spec.Role)
		}
	case consts.SchedulerType:
		if g.ytsaurus.Spec.Schedulers != nil {
			names = append(names, "")
		}
	case consts.StrawberryControllerType:
		if g.ytsaurus.Spec.StrawberryController != nil {
			names = append(names, "")
		}
	case consts.TabletNodeType:
		for _, spec := range g.ytsaurus.Spec.TabletNodes {
			names = append(names, spec.Name)
		}
	case consts.TcpProxyType:
		for _, spec := range g.ytsaurus.Spec.TCPProxies {
			names = append(names, spec.Role)
		}
	case consts.KafkaProxyType:
		for _, spec := range g.ytsaurus.Spec.KafkaProxies {
			names = append(names, spec.Role)
		}
	case consts.YqlAgentType:
		if g.ytsaurus.Spec.YQLAgents != nil {
			names = append(names, "")
		}
	case consts.BundleControllerType:
		if g.ytsaurus.Spec.BundleController != nil {
			names = append(names, "")
		}
	default:
		return nil, fmt.Errorf("unknown component %v", component)
	}

	return names, nil
}

func (g *Generator) GetComponentConfig(component consts.ComponentType, name string) ([]byte, error) {
	switch component {
	case consts.ClusterConnectionType:
		if name == "" {
			return g.GetClusterConnectionConfig()
		}
	case consts.NativeClientConfigType:
		if name == "" {
			return g.GetNativeClientConfig()
		}
	case consts.ControllerAgentType:
		if name == "" {
			return g.GetControllerAgentConfig(g.ytsaurus.Spec.ControllerAgents)
		}
	case consts.CypressProxyType:
		if name == "" {
			return g.GetCypressProxiesConfig(g.ytsaurus.Spec.CypressProxies)
		}
	case consts.DataNodeType:
		for _, spec := range g.ytsaurus.Spec.DataNodes {
			if spec.Name == name {
				return g.GetDataNodeConfig(spec)
			}
		}
	case consts.DiscoveryType:
		if name == "" {
			return g.GetDiscoveryConfig(&g.ytsaurus.Spec.Discovery)
		}
	case consts.ExecNodeType:
		for _, spec := range g.ytsaurus.Spec.ExecNodes {
			if spec.Name == name {
				return g.GetExecNodeConfig(spec)
			}
		}
	case consts.HttpProxyType:
		for _, spec := range g.ytsaurus.Spec.HTTPProxies {
			if spec.Role == name {
				return g.GetHTTPProxyConfig(spec)
			}
		}
	case consts.MasterCacheType:
		if name == "" {
			return g.GetMasterCachesConfig(g.ytsaurus.Spec.MasterCaches)
		}
	case consts.MasterType:
		if name == "" {
			return g.GetMasterConfig(&g.ytsaurus.Spec.PrimaryMasters)
		}
	case consts.QueryTrackerType:
		if name == "" {
			return g.GetQueryTrackerConfig(g.ytsaurus.Spec.QueryTrackers)
		}
	case consts.QueueAgentType:
		if name == "" {
			return g.GetQueueAgentConfig(g.ytsaurus.Spec.QueueAgents)
		}
	case consts.RpcProxyType:
		for _, spec := range g.ytsaurus.Spec.RPCProxies {
			if spec.Role == name {
				return g.GetRPCProxyConfig(spec)
			}
		}
	case consts.SchedulerType:
		if name == "" {
			return g.GetSchedulerConfig(g.ytsaurus.Spec.Schedulers)
		}
	case consts.StrawberryControllerType:
		if name == "" {
			return g.GetStrawberryControllerConfig()
		}
	case consts.TabletNodeType:
		for _, spec := range g.ytsaurus.Spec.TabletNodes {
			if spec.Name == name {
				return g.GetTabletNodeConfig(spec)
			}
		}
	case consts.TcpProxyType:
		for _, spec := range g.ytsaurus.Spec.TCPProxies {
			if spec.Role == name {
				return g.GetTCPProxyConfig(spec)
			}
		}
	case consts.KafkaProxyType:
		for _, spec := range g.ytsaurus.Spec.KafkaProxies {
			if spec.Role == name {
				return g.GetKafkaProxyConfig(spec)
			}
		}
	case consts.YqlAgentType:
		if name == "" {
			return g.GetYQLAgentConfig(g.ytsaurus.Spec.YQLAgents)
		}
	case consts.BundleControllerType:
		if name == "" {
			return g.GetBundleControllerConfig(g.ytsaurus.Spec.BundleController)
		}
	}

	return nil, fmt.Errorf("unknown component %v name %v", component, name)
}
