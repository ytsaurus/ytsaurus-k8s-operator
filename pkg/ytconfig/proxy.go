package ytconfig

import (
	"path"

	"go.ytsaurus.tech/yt/go/yson"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

type CypressCookieManager struct{}
type CypressUserManager struct{}
type CypressTokenAuthenticator struct {
	Secure bool `yson:"secure"`
}

type OauthService struct {
	Host               string  `yson:"host"`
	Port               int     `yson:"port"`
	Secure             bool    `yson:"secure"`
	UserInfoEndpoint   string  `yson:"user_info_endpoint"`
	UserInfoLoginField string  `yson:"user_info_login_field"`
	UserInfoErrorField *string `yson:"user_info_error_field,omitempty"`
}

type OauthCookieAuthenticator struct{}
type OauthTokenAuthenticator struct{}

type Coordinator struct {
	Enable            bool   `yson:"enable"`
	DefaultRoleFilter string `yson:"default_role_filter"`
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
}

type NativeClient struct {
	AddressResolver AddressResolver `yson:"address_resolver"`
	Logging         Logging         `yson:"logging"`
	Driver          Driver          `yson:"driver"`
}

type RPCProxyServer struct {
	CommonServer
	Role string `yson:"role"`
	Auth Auth   `yson:"auth"`
	//CypressUserManager        CypressUserManager        `yson:"cypress_user_manager"`
	//CypressTokenAuthenticator CypressTokenAuthenticator `yson:"cypress_token_authenticator"`
	//OauthService              *OauthService             `yson:"oauth_service,omitempty"`
	//OauthTokenAuthenticator   *OauthTokenAuthenticator  `yson:"oauth_token_authenticator,omitempty"`
	//RequireAuthentication     bool                      `yson:"require_authentication"`
}

type TCPProxyServer struct {
	CommonServer
	Role string `yson:"role"`
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

	c.RPCPort = consts.HTTPProxyRPCPort
	c.MonitoringPort = *spec.InstanceSpec.MonitoringPort
	c.Port = consts.HTTPProxyHTTPPort

	c.Role = spec.Role

	c.Logging = getHTTPProxyLogging(spec)

	// FIXME handle DisableHTTP

	if spec.Transport.HTTPSSecret != nil {
		c.HTTPSServer = &HTTPSServer{
			HTTPServer: HTTPServer{
				Port: consts.HTTPProxyHTTPSPort,
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

	c.Auth.RequireAuthentication = true
	c.Auth.CypressTokenAuthenticator.Secure = true

	c.RPCPort = consts.RPCProxyRPCPort
	c.MonitoringPort = *spec.InstanceSpec.MonitoringPort

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

func getTCPProxyServerCarcass(spec *ytv1.TCPProxiesSpec) (TCPProxyServer, error) {
	var c TCPProxyServer

	c.MonitoringPort = *spec.InstanceSpec.MonitoringPort

	c.Role = spec.Role
	c.Logging = getTCPProxyLogging(spec)

	return c, nil
}

func getNativeClientCarcass() (NativeClient, error) {
	var c NativeClient

	loggingBuilder := newLoggingBuilder(nil, "client")
	c.Logging = loggingBuilder.logging

	return c, nil
}
