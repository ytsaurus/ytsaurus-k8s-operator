package validators

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/yaml"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

func TestSpytSparkVersionValidation(t *testing.T) {
	for _, tt := range []struct {
		name          string
		image         string
		sparkVersions []string
		wantError     string
	}{
		{name: "no versions", image: "spyt:2.11.2"},
		{name: "no image tag", image: "spyt", sparkVersions: []string{"3.2.2"}},
		{name: "valid config", image: "spyt:2.11.2", sparkVersions: []string{"3.5.9", "4.2.1"}},
		{name: "incompatible latest version", image: "spyt:2.11.2", sparkVersions: []string{"4.2.1", "5.0.0"}, wantError: "only Spark 3.5.x and 4.x"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			spyt := &ytv1.Spyt{Spec: ytv1.SpytSpec{Image: tt.image, SparkVersions: tt.sparkVersions}}
			validator := NewSpytValidator()
			_, createErr := validator.ValidateCreate(context.Background(), spyt)
			_, updateErr := validator.ValidateUpdate(context.Background(), &ytv1.Spyt{}, spyt)
			for _, err := range []error{createErr, updateErr} {
				if tt.wantError != "" {
					require.True(t, apierrors.IsInvalid(err))
					require.ErrorContains(t, err, "spec.sparkVersions")
					require.ErrorContains(t, err, tt.wantError)
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestSpytSampleSparkVersion(t *testing.T) {
	data, err := os.ReadFile("../config/samples/cluster_v1_spyt.yaml")
	require.NoError(t, err)
	var spyt ytv1.Spyt
	require.NoError(t, yaml.Unmarshal(data, &spyt))
	require.NotEmpty(t, spyt.Spec.SparkVersions)
	_, err = NewSpytValidator().ValidateCreate(context.Background(), &spyt)
	require.NoError(t, err)
}
