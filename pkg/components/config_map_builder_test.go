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
