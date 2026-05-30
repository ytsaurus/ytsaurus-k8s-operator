package v1

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

var (
	_ interface{ Merge(HTTPProxiesSpec) (HTTPProxiesSpec, error) }   = HTTPProxiesSpec{}
	_ interface{ Merge(RPCProxiesSpec) (RPCProxiesSpec, error) }     = RPCProxiesSpec{}
	_ interface{ Merge(TCPProxiesSpec) (TCPProxiesSpec, error) }     = TCPProxiesSpec{}
	_ interface{ Merge(KafkaProxiesSpec) (KafkaProxiesSpec, error) } = KafkaProxiesSpec{}
	_ interface{ Merge(DataNodesSpec) (DataNodesSpec, error) }       = DataNodesSpec{}
	_ interface{ Merge(ExecNodesSpec) (ExecNodesSpec, error) }       = ExecNodesSpec{}
	_ interface{ Merge(TabletNodesSpec) (TabletNodesSpec, error) }   = TabletNodesSpec{}
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

func TestResolveInstanceGroupTemplates_MergesNamedLists(t *testing.T) {
	t.Parallel()

	spec := YtsaurusSpec{
		DataNodes: []DataNodesSpec{
			{
				InstanceGroupTemplateSpec: InstanceGroupTemplateSpec{Class: "common-data"},
				InstanceSpec: InstanceSpec{
					Volumes: []Volume{
						{
							Name: "cache",
							VolumeSource: VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "logs",
							VolumeSource: VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "base-logs",
								},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "cache", MountPath: "/var/cache"},
						{Name: "logs", MountPath: "/var/log/yt"},
					},
				},
			},
			{
				InstanceGroupTemplateSpec: InstanceGroupTemplateSpec{From: "common-data"},
				InstanceSpec: InstanceSpec{
					Volumes: []Volume{
						{
							Name: "cache",
							VolumeSource: VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{Name: "cache-config"},
								},
							},
						},
						{
							Name: "data",
							VolumeSource: VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "cache", MountPath: "/var/cache", ReadOnly: true},
						{Name: "data", MountPath: "/var/lib/yt"},
					},
				},
				Name: "a",
			},
		},
	}

	require.NoError(t, spec.ResolveInstanceGroupTemplates())
	require.Len(t, spec.DataNodes, 1)

	volumesByName := map[string]Volume{}
	for _, volume := range spec.DataNodes[0].Volumes {
		volumesByName[volume.Name] = volume
	}

	require.Len(t, volumesByName, 3)
	require.NotNil(t, volumesByName["cache"].ConfigMap)
	require.Nil(t, volumesByName["cache"].EmptyDir)
	require.Equal(t, "cache-config", volumesByName["cache"].ConfigMap.Name)
	require.Equal(t, "base-logs", volumesByName["logs"].Secret.SecretName)
	require.NotNil(t, volumesByName["data"].EmptyDir)

	mountsByPath := map[string]corev1.VolumeMount{}
	for _, mount := range spec.DataNodes[0].VolumeMounts {
		mountsByPath[mount.MountPath] = mount
	}

	require.Len(t, mountsByPath, 3)
	require.Equal(t, "cache", mountsByPath["/var/cache"].Name)
	require.True(t, mountsByPath["/var/cache"].ReadOnly)
	require.Equal(t, "logs", mountsByPath["/var/log/yt"].Name)
	require.Equal(t, "data", mountsByPath["/var/lib/yt"].Name)
}

func ptrTo[T any](value T) *T {
	return &value
}
