package ytconfig

import (
	"path"

	"go.ytsaurus.tech/yt/go/yson"
	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

type CypressCookieManager struct{}
type CypressUserManager struct{}
type CypressTokenAuthenticator struct {
	Secure bool `yson:"secure"`
}

type OauthService struct {
	Host                 string                `yson:"host"`
	Port                 int                   `yson:"port"`
	Secure               bool                  `yson:"secure"`
	UserInfoEndpoint     string                `yson:"user_info_endpoint"`
	UserInfoLoginField   string                `yson:"user_info_login_field"`
	UserInfoErrorField   *string               `yson:"user_info_error_field,omitempty"`
	LoginTransformations []LoginTransformation `yson:"login_transformations,omitempty"`
}

type LoginTransformation struct {
	MatchPattern string `yson:"match_pattern,omitempty"`
	Replacement  string `yson:"replacement,omitempty"`
}

type OauthCookieAuthenticator struct {
	CreateUserIfNotExists *bool `yson:"create_user_if_not_exists,omitempty"`
}
type OauthTokenAuthenticator struct {
	CreateUserIfNotExists *bool `yson:"create_user_if_not_exists,omitempty"`
}

type Coordinator struct {
	Enable            bool   `yson:"enable"`
	DefaultRoleFilter string `yson:"default_role_filter"`
	ShowPorts         *bool  `yson:"show_ports,omitempty"`
}

type Auth struct {
	CypressCookieManager      CypressCookieManager      `yson:"cypress_cookie_manager"`
	CypressUserManager        CypressUserManager        `yson:"cypress_user_manager"`
	CypressTokenAuthenticator CypressTokenAuthenticator `yson:"cypress_token_authenticator"`
	OauthService              *OauthService             `yson:"oauth_service,omitempty"`
	OauthCookieAuthenticator  *OauthCookieAuthenticator `yson:"oauth_cookie_authenticator,omitempty"`
	OauthTokenAuthenticator   *OauthTokenAuthenticator  `yson:"oauth_token_authenticator,omitempty"`
	RequireAuthentication     bool                      `yson:"require_authentication"`
}

type HTTPServer struct {
	Port int `yson:"port"`
}

type HTTPSServerCredentials struct {
	CertChain    PemBlob       `yson:"cert_chain,omitempty"`
	PrivateKey   PemBlob       `yson:"private_key,omitempty"`
	UpdatePeriod yson.Duration `yson:"update_period,omitempty"`
}

type HTTPSServer struct {
	HTTPServer
	Credentials HTTPSServerCredentials `yson:"credentials"`
}

type HTTPProxyServer struct {
	CommonServer
	Port        int          `yson:"port"`
	Auth        Auth         `yson:"auth"`
	Coordinator Coordinator  `yson:"coordinator"`
	Driver      Driver       `yson:"driver"`
	Role        string       `yson:"role"`
	HTTPSServer *HTTPSServer `yson:"https_server,omitempty"`
	Addresses   *[][]string  `yson:"addresses,omitempty"`
}

type RPCProxyServer struct {
	CommonServer
	Role                      string                    `yson:"role"`
	CypressUserManager        CypressUserManager        `yson:"cypress_user_manager"`
	CypressTokenAuthenticator CypressTokenAuthenticator `yson:"cypress_token_authenticator"`
	OauthService              *OauthService             `yson:"oauth_service,omitempty"`
	OauthTokenAuthenticator   *OauthTokenAuthenticator  `yson:"oauth_token_authenticator,omitempty"`
	RequireAuthentication     *bool                     `yson:"require_authentication,omitempty"`
}

type TCPProxyServer struct {
	CommonServer
	Role string `yson:"role"`
}

type KafkaProxyServer struct {
	CommonServer
	Role string `yson:"role"`
	Port int    `yson:"port"`
	Auth Auth   `yson:"auth"`
}

func getHTTPProxyLogging(spec *ytv1.HTTPProxiesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"http-proxy",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getHTTPProxyServerCarcass(spec *ytv1.HTTPProxiesSpec) (HTTPProxyServer, error) {
	var c HTTPProxyServer

	c.Auth.RequireAuthentication = true
	c.Auth.CypressTokenAuthenticator.Secure = true
	c.Coordinator.Enable = true
	c.Coordinator.DefaultRoleFilter = consts.DefaultHTTPProxyRole
	if spec.HttpPort != nil && *spec.HttpPort != consts.HTTPProxyHTTPPort {
		c.Coordinator.ShowPorts = ptr.To(true)
	}

	c.RPCPort = consts.HTTPProxyRPCPort
	c.MonitoringPort = ptr.Deref(spec.InstanceSpec.MonitoringPort, consts.HTTPProxyMonitoringPort)
	c.Port = int(ptr.Deref(spec.HttpPort, int32(consts.HTTPProxyHTTPPort)))

	c.Role = spec.Role

	c.Logging = getHTTPProxyLogging(spec)

	// FIXME handle DisableHTTP

	if spec.Transport.HTTPSSecret != nil {
		c.HTTPSServer = &HTTPSServer{
			HTTPServer: HTTPServer{
				Port: int(ptr.Deref(spec.HttpsPort, int32(consts.HTTPProxyHTTPSPort))),
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

	return c, nil
}

func getRPCProxyLogging(spec *ytv1.RPCProxiesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"rpc-proxy",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getRPCProxyServerCarcass(spec *ytv1.RPCProxiesSpec) (RPCProxyServer, error) {
	var c RPCProxyServer

	c.CypressTokenAuthenticator.Secure = true

	c.RPCPort = consts.RPCProxyRPCPort
	c.MonitoringPort = ptr.Deref(spec.InstanceSpec.MonitoringPort, consts.RPCProxyMonitoringPort)

	c.Role = spec.Role
	c.Logging = getRPCProxyLogging(spec)

	return c, nil
}

func getTCPProxyLogging(spec *ytv1.TCPProxiesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"tcp-proxy",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getKafkaProxyLogging(spec *ytv1.KafkaProxiesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"kafka-proxy",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec(), defaultDebugLoggerSpec()})
}

func getTCPProxyServerCarcass(spec *ytv1.TCPProxiesSpec) (TCPProxyServer, error) {
	var c TCPProxyServer

	c.MonitoringPort = ptr.Deref(spec.InstanceSpec.MonitoringPort, consts.TCPProxyMonitoringPort)
	c.Role = spec.Role
	c.Logging = getTCPProxyLogging(spec)

	return c, nil
}

func getKafkaProxyServerCarcass(spec *ytv1.KafkaProxiesSpec) (KafkaProxyServer, error) {
	var c KafkaProxyServer

	c.MonitoringPort = ptr.Deref(spec.InstanceSpec.MonitoringPort, consts.KafkaProxyMonitoringPort)
	c.RPCPort = consts.KafkaProxyRPCPort

	c.Role = spec.Role
	c.Auth.RequireAuthentication = true
	c.Auth.CypressTokenAuthenticator.Secure = true
	c.Port = consts.KafkaProxyKafkaPort
	c.Logging = getKafkaProxyLogging(spec)

	return c, nil
}
