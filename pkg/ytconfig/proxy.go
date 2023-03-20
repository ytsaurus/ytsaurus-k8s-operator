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

type HttpProxyServer struct {
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

type RpcProxyServer struct {
	CommonServer
	CypressTokenAuthenticator CypressTokenAuthenticator `yson:"cypress_token_authenticator"`
}

func getHttpProxyServerCarcass(spec ytv1.HttpProxiesSpec) (HttpProxyServer, error) {
	var c HttpProxyServer

	c.Auth.RequireAuthentication = true
	c.Auth.CypressTokenAuthenticator.Secure = true
	c.Coordinator.Enable = true

	c.RpcPort = consts.HttpProxyRpcPort
	c.MonitoringPort = consts.HttpProxyMonitoringPort
	c.Port = consts.HttpProxyHttpPort

	loggingBuilder := newLoggingBuilder(ytv1.FindFirstLocation(spec.InstanceGroup.Locations, ytv1.LocationTypeLogs), "http-proxy")
	c.Logging = loggingBuilder.addDefaultDebug().addDefaultStderr().logging

	return c, nil
}

func getRpcProxyServerCarcass(spec ytv1.RpcProxiesSpec) (RpcProxyServer, error) {
	var c RpcProxyServer

	c.CypressTokenAuthenticator.Secure = true

	c.RpcPort = consts.RpcProxyRpcPort
	c.MonitoringPort = consts.RpcProxyMonitoringPort

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
