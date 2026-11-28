package components

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.ytsaurus.tech/yt/go/yson"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

func TestSpytQueryTrackerDynamicConfig(t *testing.T) {
	spyt := &Spyt{
		labeller: &labeller.Labeller{ClusterName: "minisaurus"},
		spyt: apiproxy.NewSpyt(&ytv1.Spyt{Spec: ytv1.SpytSpec{
			Image:         "ghcr.io/ytsaurus/spyt:2.11.2",
			SparkVersions: []string{"3.5.9", "4.2.1"},
		}}, nil, nil, nil),
	}
	command, err := spyt.buildQueryTrackerDynamicConfigCommand()
	require.NoError(t, err)
	_, configYson, found := strings.Cut(command, "/usr/bin/yt set -r //sys/query_tracker/config/query_tracker/spyt_engine '")
	require.True(t, found)
	require.True(t, strings.HasSuffix(configYson, "'"))

	var config struct {
		DefaultCluster string `yson:"default_cluster"`
		SparkVersion   string `yson:"spark_version"`
		SpytVersion    string `yson:"spyt_version"`
		ProxyConfig    struct {
			Clusters        []string       `yson:"clusters"`
			DefaultSettings map[string]any `yson:"default_settings"`
		} `yson:"proxy_config"`
	}
	require.NoError(t, yson.Unmarshal([]byte(strings.TrimSuffix(configYson, "'")), &config))
	require.Equal(t, "minisaurus", config.DefaultCluster)
	require.Equal(t, "4.2.1", config.SparkVersion)
	require.Equal(t, "2.11.2", config.SpytVersion)
	require.Equal(t, []string{"minisaurus"}, config.ProxyConfig.Clusters)
	require.NotContains(t, config.ProxyConfig.DefaultSettings, "spark_conf")
}

func TestSpytQueryTrackerDynamicConfigGuards(t *testing.T) {
	for _, tt := range []struct {
		name          string
		image         string
		sparkVersions []string
		wantError     string
	}{
		{name: "no versions", image: "spyt:2.11.2"},
		{name: "no image tag", image: "spyt", sparkVersions: []string{"3.2.2"}},
		{name: "no image", sparkVersions: []string{"3.2.2"}},
		{name: "incompatible latest version", image: "spyt:2.11.2", sparkVersions: []string{"4.2.1", "5.0.0"}, wantError: "only Spark 3.5.x and 4.x"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spyt := &Spyt{spyt: apiproxy.NewSpyt(&ytv1.Spyt{Spec: ytv1.SpytSpec{
				Image: tt.image, SparkVersions: tt.sparkVersions,
			}}, nil, nil, nil)}
			command, err := spyt.buildQueryTrackerDynamicConfigCommand()
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, command)
		})
	}
}
