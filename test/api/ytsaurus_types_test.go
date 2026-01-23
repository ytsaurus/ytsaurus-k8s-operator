package api_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/stretchr/testify/require"

	"k8s.io/utils/ptr"

	k8syaml "sigs.k8s.io/yaml"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

var testSpec = ytv1.YtsaurusSpec{
	CommonSpec: ytv1.CommonSpec{
		CoreImage: "img",
		UseIPv6:   true,
		UseIPv4:   false,
	},
	PrimaryMasters: ytv1.MastersSpec{
		InstanceSpec: ytv1.InstanceSpec{
			PodSpec: ytv1.PodSpec{
				Tolerations: []corev1.Toleration{
					{
						Key: "base-toleration",
					},
				},
			},
			VolumeClaimTemplates: []ytv1.EmbeddedPersistentVolumeClaim{
				{
					EmbeddedObjectMetadata: ytv1.EmbeddedObjectMetadata{
						Name: "master-data",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						StorageClassName: ptr.To[string]("className"),
						Resources: corev1.VolumeResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
		},
	},
	HTTPProxies: []ytv1.HTTPProxiesSpec{
		{
			InstanceSpec: ytv1.InstanceSpec{
				InstanceCount: 1,
			},
			ServiceType: corev1.ServiceTypeClusterIP,
		},
	},
}

type marshalFunc func(v any) ([]byte, error)
type unmarshalFunc func(data []byte, v any) error

func TestMarshallUnmarshall(t *testing.T) {
	t.Run("JSON", func(t *testing.T) {
		testMarshallUnmarshall(t, json.Marshal, json.Unmarshal)
	})
	t.Run("YAML-JSON", func(t *testing.T) {
		testMarshallUnmarshall(
			t,
			k8syaml.Marshal,
			func(data []byte, v any) error {
				return k8syaml.Unmarshal(data, v)
			},
		)
	})
}

func testMarshallUnmarshall(t *testing.T, marshall marshalFunc, unmarshall unmarshalFunc) {
	serialized, err := marshall(testSpec)
	require.NoError(t, err)

	deserializedSpec := &ytv1.YtsaurusSpec{}
	err = unmarshall(serialized, deserializedSpec)
	require.NoError(t, err)

	require.Empty(t, cmp.Diff(testSpec, *deserializedSpec))

	reSerialized, err := marshall(deserializedSpec)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(serialized, reSerialized))
}

func TestScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, ytv1.AddToScheme(scheme))
	for _, obj := range ytv1.KnownObjectTypes() {
		gvks, isUnversioned, err := scheme.ObjectKinds(obj)
		require.NoError(t, err)
		require.Len(t, gvks, 1)
		require.False(t, isUnversioned)
		t.Log("GVK", gvks[0])
		_, err = scheme.New(gvks[0])
		require.NoError(t, err)
		_, ok := obj.(client.Object)
		require.True(t, ok, "kind %v", gvks[0])
	}
}
