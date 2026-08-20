package components

import (
	_ "embed"
	"strings"
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

func TestConfigMergePreservesFieldOrder(t *testing.T) {
	merged, err := overrideYsonConfigs(
		[]byte(`{
			root_first = 1;
			nested = { nested_first = 1; nested_last = 2; };
			list = [{ list_first = 1; list_last = 2; }];
			root_last = 2;
		}`),
		[]byte(`{
			nested = { nested_first = 3; nested_new = 4; };
			root_new = 3;
		}`),
	)
	require.NoError(t, err)

	result := string(merged)
	require.Less(t, strings.Index(result, "root_first"), strings.Index(result, "nested="))
	require.Less(t, strings.Index(result, "nested="), strings.Index(result, "list="))
	require.Less(t, strings.Index(result, "list="), strings.Index(result, "root_last"))
	require.Less(t, strings.Index(result, "root_last"), strings.Index(result, "root_new"))
	require.Less(t, strings.Index(result, "nested_first"), strings.Index(result, "nested_last"))
	require.Less(t, strings.Index(result, "nested_last"), strings.Index(result, "nested_new"))
	require.Less(t, strings.Index(result, "list_first"), strings.Index(result, "list_last"))
	require.Contains(t, result, `"nested_first"=3`)
}
