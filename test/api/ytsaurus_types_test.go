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
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytcluster"
)

var testSpec = ytv1.YtsaurusSpec{
	CommonSpec: ytv1.CommonSpec{
		CoreImage: "img",
		UseIPv6:   true,
		UseIPv4:   false,
	},
	PrimaryMasters: ytv1.PrimaryMastersSpec{MastersSpec: ytv1.MastersSpec{
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
	}},
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

func TestComponentResourceSerialization(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, ytv1.AddToScheme(scheme))
	for _, member := range ytcluster.KnownYtsaurusComponentObjects() {
		gvks, _, err := scheme.ObjectKinds(member)
		require.NoError(t, err)
		t.Run(gvks[0].Kind, func(t *testing.T) {
			data := []byte(`{"spec":{"ytsaurus":{"name":"cluster"},"instanceCount":3},"status":{"state":"Ready","observedGeneration":2}}`)
			require.NoError(t, json.Unmarshal(data, member))
			require.Equal(t, "cluster", member.GetYtsaurusName())
			require.Equal(t, 3, member.GetComponentSpec().GetInstanceCount())
			require.Equal(t, ytv1.ComponentStateReady, member.GetComponentStatus().State)
			require.Equal(t, int64(2), member.GetStatusObservedGeneration())

			serialized, err := json.Marshal(member)
			require.NoError(t, err)
			var fields map[string]any
			require.NoError(t, json.Unmarshal(serialized, &fields))
			require.NotContains(t, fields, "ytsaurus")
			require.Equal(t, map[string]any{"name": "cluster"}, fields["spec"].(map[string]any)["ytsaurus"])
			require.Equal(t, "Ready", fields["status"].(map[string]any)["state"])
		})
	}
}
