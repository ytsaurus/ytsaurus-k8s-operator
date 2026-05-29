package v1

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveInstanceGroupTemplates_DataNodes(t *testing.T) {
	t.Parallel()

	replicas := int32(3)
	spec := YtsaurusSpec{
		DataNodes: []DataNodesSpec{
			{
				InstanceGroupTemplateSpec: InstanceGroupTemplateSpec{Class: "common-data"},
				InstanceSpec: InstanceSpec{
					InstanceCount: replicas,
					PodSpec: PodSpec{
						NodeSelector: map[string]string{"topology.kubernetes.io/zone": "common"},
					},
					Image: ptrTo("common-image"),
				},
				ClusterNodesSpec: ClusterNodesSpec{
					Tags: []string{"common"},
				},
			},
			{
				InstanceGroupTemplateSpec: InstanceGroupTemplateSpec{Class: "patched-data", From: "common-data"},
				InstanceSpec: InstanceSpec{
					MonitoringPort: ptrTo(int32(10101)),
				},
			},
			{
				InstanceGroupTemplateSpec: InstanceGroupTemplateSpec{From: "common-data"},
				ClusterNodesSpec: ClusterNodesSpec{
					Rack: "rack-a",
				},
				Name: "a",
			},
			{
				InstanceGroupTemplateSpec: InstanceGroupTemplateSpec{From: "patched-data"},
				ClusterNodesSpec: ClusterNodesSpec{
					Rack: "rack-b",
				},
				Name: "b",
			},
		},
	}

	require.NoError(t, spec.ResolveInstanceGroupTemplates())
	require.Len(t, spec.DataNodes, 2)

	require.Equal(t, "a", spec.DataNodes[0].Name)
	require.Equal(t, "rack-a", spec.DataNodes[0].Rack)
	require.Equal(t, replicas, spec.DataNodes[0].InstanceCount)
	require.Equal(t, "common-image", *spec.DataNodes[0].Image)
	require.Equal(t, map[string]string{"topology.kubernetes.io/zone": "common"}, spec.DataNodes[0].NodeSelector)
	require.Equal(t, []string{"common"}, spec.DataNodes[0].Tags)
	require.Empty(t, spec.DataNodes[0].Class)
	require.Empty(t, spec.DataNodes[0].From)

	require.Equal(t, "b", spec.DataNodes[1].Name)
	require.Equal(t, "rack-b", spec.DataNodes[1].Rack)
	require.Equal(t, replicas, spec.DataNodes[1].InstanceCount)
	require.Equal(t, "common-image", *spec.DataNodes[1].Image)
	require.Equal(t, int32(10101), *spec.DataNodes[1].MonitoringPort)
}

func TestResolveInstanceGroupTemplates_Errors(t *testing.T) {
	t.Parallel()

	t.Run("unknown class", func(t *testing.T) {
		t.Parallel()

		spec := YtsaurusSpec{
			DataNodes: []DataNodesSpec{{
				InstanceGroupTemplateSpec: InstanceGroupTemplateSpec{From: "missing"},
				Name:                      "a",
			}},
		}

		err := spec.ResolveInstanceGroupTemplates()
		require.Error(t, err)
		require.Equal(t, &InstanceGroupTemplateError{
			List:    "dataNodes",
			Index:   0,
			Field:   "from",
			Value:   "missing",
			Message: `unknown template class "missing"`,
		}, err)
	})

	t.Run("cyclic class", func(t *testing.T) {
		t.Parallel()

		spec := YtsaurusSpec{
			DataNodes: []DataNodesSpec{
				{InstanceGroupTemplateSpec: InstanceGroupTemplateSpec{Class: "a", From: "b"}},
				{InstanceGroupTemplateSpec: InstanceGroupTemplateSpec{Class: "b", From: "a"}},
			},
		}

		err := spec.ResolveInstanceGroupTemplates()
		require.Error(t, err)
		require.Equal(t, &InstanceGroupTemplateError{
			List:    "dataNodes",
			Index:   0,
			Field:   "from",
			Value:   "b",
			Message: "cyclic template inheritance",
		}, err)
	})
}

func ptrTo[T any](value T) *T {
	return &value
}
