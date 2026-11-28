package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLatestSpytQueryTrackerSparkVersion(t *testing.T) {
	for _, tt := range []struct {
		name          string
		sparkVersions []string
		wantVersion   string
		wantError     string
	}{
		{name: "no versions"},
		{name: "default version", sparkVersions: []string{""}},
		{name: "Spark 3.5", sparkVersions: []string{"3.5.9"}, wantVersion: "3.5.9"},
		{name: "Spark 4", sparkVersions: []string{"4.2.1"}, wantVersion: "4.2.1"},
		{name: "ascending", sparkVersions: []string{"3.5.9", "4.2.1"}, wantVersion: "4.2.1"},
		{name: "descending", sparkVersions: []string{"4.2.1", "3.5.9"}, wantVersion: "4.2.1"},
		{name: "numeric patch", sparkVersions: []string{"3.5.9", "3.5.10"}, wantVersion: "3.5.10"},
		{name: "numeric minor", sparkVersions: []string{"4.2.1", "4.10.0"}, wantVersion: "4.10.0"},
		{name: "release over prerelease", sparkVersions: []string{"4.2.1-rc1", "4.2.1"}, wantVersion: "4.2.1"},
		{name: "older unsupported version", sparkVersions: []string{"3.2.2", "3.5.9"}, wantVersion: "3.5.9"},
		{name: "empty with explicit version", sparkVersions: []string{"", "4.2.1"}, wantVersion: "4.2.1"},
		{name: "incompatible latest version", sparkVersions: []string{"4.2.1", "5.0.0"}, wantError: "only Spark 3.5.x and 4.x"},
		{name: "unsupported minor", sparkVersions: []string{"3.4.4"}, wantError: "only Spark 3.5.x and 4.x"},
		{name: "unsupported major", sparkVersions: []string{"5.0.0"}, wantError: "only Spark 3.5.x and 4.x"},
		{name: "invalid version", sparkVersions: []string{"invalid", "4.2.1"}, wantError: "invalid Spark version"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LatestSpytQueryTrackerSparkVersion(tt.sparkVersions)
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantVersion, got)
		})
	}
}
