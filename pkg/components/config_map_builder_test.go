package components

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/canonize"
)

//go:embed testdata/TestConfigMerge/http_proxy_config_wo_override.yson
var hpConfigWithoutOverride string

//go:embed testdata/TestConfigMerge/http_proxy_config_override.yson
var hpConfigOverride string

func TestConfigMerge(t *testing.T) {
	merged, err := overrideYsonConfigs(
		[]byte(hpConfigWithoutOverride),
		[]byte(hpConfigOverride),
	)
	require.NoError(t, err)
	canonize.Assert(t, merged)
}

func TestRenderJSModuleWithOAuthEnvReferences(t *testing.T) {
	config := map[string]interface{}{
		"odinBaseUrl": "http://odin-webservice.odin.svc.cluster.local",
		"ytOAuthSettings": map[string]interface{}{
			"baseURL":             "https://id.example.com",
			"authPath":            "oauth/v2/authorize",
			"tokenPath":           "oauth/v2/token",
			"clientIdEnvName":     "OAUTH_CLIENT_ID",
			"clientSecretEnvName": "OAUTH_CLIENT_SECRET",
			"buttonLabel":         "Login via \"SSO\"",
			"callbackBaseUrl":     "https://yt.example.com",
		},
	}

	rendered, err := renderJSModule(config)
	require.NoError(t, err)
	require.Equal(t, `module.exports = {
  odinBaseUrl: "http://odin-webservice.odin.svc.cluster.local",
  ytOAuthSettings: {
    authPath: "oauth/v2/authorize",
    baseURL: "https://id.example.com",
    buttonLabel: "Login via \"SSO\"",
    callbackBaseUrl: "https://yt.example.com",
    clientId: process.env.OAUTH_CLIENT_ID,
    clientSecret: process.env.OAUTH_CLIENT_SECRET,
    tokenPath: "oauth/v2/token",
  },
}`, string(rendered))
}

func TestRenderJSModuleRejectsInvalidOAuthEnvReference(t *testing.T) {
	config := map[string]interface{}{
		"ytOAuthSettings": map[string]interface{}{
			"clientIdEnvName": "OAUTH-CLIENT-ID",
		},
	}

	_, err := renderJSModule(config)
	require.ErrorContains(t, err, "invalid UI OAuth environment variable name")
}
