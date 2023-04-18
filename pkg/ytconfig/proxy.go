package ytconfig

import (
	ytv1 "github.com/YTsaurus/yt-k8s-operator/api/v1"
	"github.com/YTsaurus/yt-k8s-operator/pkg/consts"
)

type CypressCookieManager struct{}
type CypressTokenAuthenticator struct {
	Secure bool `yson:"secure"`
}

type Coordinator struct {
	Enable bool `yson:"enable"`
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
}

type NativeClient struct {
	AddressResolver AddressResolver `yson:"address_resolver"`
	Logging         Logging         `yson:"logging"`
	Driver          Driver          `yson:"driver"`
}

type RPCProxyServer struct {
	CommonServer
	CypressTokenAuthenticator CypressTokenAuthenticator `yson:"cypress_token_authenticator"`
}

func getHTTPProxyServerCarcass(spec ytv1.HTTPProxiesSpec) (HTTPProxyServer, error) {
	var c HTTPProxyServer

	c.Auth.RequireAuthentication = true
	c.Auth.CypressTokenAuthenticator.Secure = true
	c.Coordinator.Enable = true

	c.RPCPort = consts.HTTPProxyRPCPort
	c.MonitoringPort = consts.HTTPProxyMonitoringPort
	c.Port = consts.HTTPProxyHTTPPort

	loggingBuilder := newLoggingBuilder(ytv1.FindFirstLocation(spec.InstanceGroup.Locations, ytv1.LocationTypeLogs), "http-proxy")
	c.Logging = loggingBuilder.addDefaultDebug().addDefaultStderr().logging

	return c, nil
}

func getRPCProxyServerCarcass(spec ytv1.RPCProxiesSpec) (RPCProxyServer, error) {
	var c RPCProxyServer

	c.CypressTokenAuthenticator.Secure = true

	c.RPCPort = consts.RPCProxyRPCPort
	c.MonitoringPort = consts.RPCProxyMonitoringPort

	loggingBuilder := newLoggingBuilder(ytv1.FindFirstLocation(spec.InstanceGroup.Locations, ytv1.LocationTypeLogs), "rpc-proxy")
	c.Logging = loggingBuilder.addDefaultDebug().addDefaultStderr().logging

	return c, nil
}

func getNativeClientCarcass() (NativeClient, error) {
	var c NativeClient

	loggingBuilder := newLoggingBuilder(nil, "client")
	c.Logging = loggingBuilder.logging

	return c, nil
}
