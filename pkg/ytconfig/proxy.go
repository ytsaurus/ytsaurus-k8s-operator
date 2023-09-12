package ytconfig

import (
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

type CypressCookieManager struct{}
type CypressTokenAuthenticator struct {
	Secure bool `yson:"secure"`
}

type Coordinator struct {
	Enable            bool   `yson:"enable"`
	DefaultRoleFilter string `yson:"default_role_filter"`
}

type Auth struct {
	CypressCookieManager      CypressCookieManager      `yson:"cypress_cookie_manager"`
	CypressTokenAuthenticator CypressTokenAuthenticator `yson:"cypress_token_authenticator"`
	RequireAuthentication     bool                      `yson:"require_authentication"`
}

type HTTPProxyServer struct {
	CommonServer
	Port        int         `yson:"port"`
	Auth        Auth        `yson:"auth"`
	Coordinator Coordinator `yson:"coordinator"`
	Driver      Driver      `yson:"driver"`
	Role        string      `yson:"role"`
}

type NativeClient struct {
	AddressResolver AddressResolver `yson:"address_resolver"`
	Logging         Logging         `yson:"logging"`
	Driver          Driver          `yson:"driver"`
}

type RPCProxyServer struct {
	CommonServer
	CypressTokenAuthenticator CypressTokenAuthenticator `yson:"cypress_token_authenticator"`
	Role                      string                    `yson:"role"`
}

type TCPProxyServer struct {
	CommonServer
	Role string `yson:"role"`
}

func getHTTPProxyLogging(spec ytv1.HTTPProxiesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"http-proxy",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getHTTPProxyServerCarcass(spec ytv1.HTTPProxiesSpec) (HTTPProxyServer, error) {
	var c HTTPProxyServer

	c.Auth.RequireAuthentication = true
	c.Auth.CypressTokenAuthenticator.Secure = true
	c.Coordinator.Enable = true
	c.Coordinator.DefaultRoleFilter = consts.DefaultHTTPProxyRole

	c.RPCPort = consts.HTTPProxyRPCPort
	c.MonitoringPort = consts.HTTPProxyMonitoringPort
	c.Port = consts.HTTPProxyHTTPPort

	c.Role = spec.Role

	c.Logging = getHTTPProxyLogging(spec)

	return c, nil
}

func getRPCProxyLogging(spec ytv1.RPCProxiesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"rpc-proxy",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getRPCProxyServerCarcass(spec ytv1.RPCProxiesSpec) (RPCProxyServer, error) {
	var c RPCProxyServer

	c.CypressTokenAuthenticator.Secure = true

	c.RPCPort = consts.RPCProxyRPCPort
	c.MonitoringPort = consts.RPCProxyMonitoringPort

	c.Role = spec.Role
	c.Logging = getRPCProxyLogging(spec)

	return c, nil
}

func getTCPProxyLogging(spec ytv1.TCPProxiesSpec) Logging {
	return createLogging(
		&spec.InstanceSpec,
		"tcp-proxy",
		[]ytv1.TextLoggerSpec{defaultInfoLoggerSpec(), defaultStderrLoggerSpec()})
}

func getTCPProxyServerCarcass(spec ytv1.TCPProxiesSpec) (TCPProxyServer, error) {
	var c TCPProxyServer

	c.MonitoringPort = consts.TCPProxyMonitoringPort

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
