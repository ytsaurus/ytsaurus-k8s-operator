package components

import (
	"testing"

	"github.com/stretchr/testify/require"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/canonize"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/library/go/ptr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testClusterDomain = "fake.zone"

	testNamespace    = "fake"
	testYtsaurusName = "test"
)

var (
	testLogRotationPeriod int64 = 900000
	testTotalLogSize            = 10 * int64(1<<30)

	testStorageClassname = "yc-network-hdd"

	testResourceReqs = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewQuantity(20, resource.BinarySI),
			corev1.ResourceMemory: *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI),
		},
	}
	testJobResourceReqs = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewQuantity(99, resource.BinarySI),
			corev1.ResourceMemory: *resource.NewQuantity(99*1024*1024*1024, resource.BinarySI),
		},
	}

	testLocationChunkStore = ytv1.LocationSpec{
		LocationType: "ChunkStore",
		Path:         "/yt/hdd1/chunk-store",
		Medium:       "nvme",
	}
	testLocationChunkCache = ytv1.LocationSpec{
		LocationType: "ChunkCache",
		Path:         "/yt/hdd1/chunk-cache",
	}
	testLocationSlots = ytv1.LocationSpec{
		LocationType: "Slots",
		Path:         "/yt/hdd2/slots",
	}
	testLocationImageCache = ytv1.LocationSpec{
		LocationType: "ImageCache",
		Path:         "/yt/hdd1/images",
	}
	testVolumeMounts = []corev1.VolumeMount{
		{
			Name:      "hdd1",
			MountPath: "/yt/hdd1",
		},
	}
	testVolumeClaimTemplates = []ytv1.EmbeddedPersistentVolumeClaim{
		{
			EmbeddedObjectMetadata: ytv1.EmbeddedObjectMetadata{
				Name: "hdd1",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: &testStorageClassname,
			},
		},
	}
	testClusterNodeSpec = ytv1.ClusterNodesSpec{
		Tags: []string{"rack:xn-a"},
		Rack: "fake",
	}
)

func getCommonSpec() ytv1.CommonSpec {
	return ytv1.CommonSpec{
		UseIPv6: true,
	}
}

func getMasterConnectionSpec() ytv1.MasterConnectionSpec {
	return ytv1.MasterConnectionSpec{
		CellTag: 0,
	}
}

func GetYtsaurus() *ytv1.Ytsaurus {
	return &ytv1.Ytsaurus{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testYtsaurusName,
		},
		Spec: ytv1.YtsaurusSpec{
			CommonSpec: getCommonSpec(),

			PrimaryMasters: ytv1.MastersSpec{
				MasterConnectionSpec:   getMasterConnectionSpec(),
				MaxSnapshotCountToKeep: ptr.Int(1543),
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount:  1,
					MonitoringPort: ptr.Int32(consts.MasterMonitoringPort),

					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "master-data",
							MountPath: "/yt/master-data",
						},
					},

					Locations: []ytv1.LocationSpec{
						{
							LocationType: ytv1.LocationTypeMasterChangelogs,
							Path:         "/yt/master-data/master-changelogs",
						},
						{
							LocationType: ytv1.LocationTypeMasterSnapshots,
							Path:         "/yt/master-data/master-snapshots",
						},
					},

					VolumeClaimTemplates: []ytv1.EmbeddedPersistentVolumeClaim{
						{
							EmbeddedObjectMetadata: ytv1.EmbeddedObjectMetadata{
								Name: "master-data",
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: resource.MustParse("20Gi"),
									},
								},
							},
						},
					},

					Loggers: []ytv1.TextLoggerSpec{
						{
							BaseLoggerSpec: ytv1.BaseLoggerSpec{
								Name:        "info",
								MinLogLevel: ytv1.LogLevelInfo,
								Compression: ytv1.LogCompressionNone,
								Format:      ytv1.LogFormatPlainText,
							},
							WriterType: ytv1.LogWriterTypeFile,
						},
						{
							BaseLoggerSpec: ytv1.BaseLoggerSpec{
								Name:        "error",
								MinLogLevel: ytv1.LogLevelError,
								Compression: ytv1.LogCompressionNone,
								Format:      ytv1.LogFormatPlainText,
							},
							WriterType: ytv1.LogWriterTypeFile,
						},
						{
							BaseLoggerSpec: ytv1.BaseLoggerSpec{
								Name:        "debug",
								MinLogLevel: ytv1.LogLevelDebug,
								Compression: ytv1.LogCompressionZstd,
								Format:      ytv1.LogFormatPlainText,

								RotationPolicy: &ytv1.LogRotationPolicy{
									RotationPeriodMilliseconds: &testLogRotationPeriod,
									MaxTotalSizeToKeep:         &testTotalLogSize,
								},
							},
							WriterType: ytv1.LogWriterTypeFile,
							CategoriesFilter: &ytv1.CategoriesFilter{
								Type:   ytv1.CategoriesFilterTypeExclude,
								Values: []string{"Bus"},
							},
						},
					},
				},

				Sidecars: []string{
					"{name: sleep, image: fakeimage:stable, command: [/bin/sleep], args: [inf]}",
				},
			},
		},
	}
}

func getExecNodeSpec(jobResources *corev1.ResourceRequirements) ytv1.ExecNodesSpec {
	rotationPolicyMS := int64(900000)
	rotationPolicyMaxTotalSize := int64(3145728)
	return ytv1.ExecNodesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:  50,
			MonitoringPort: ptr.Int32(consts.ExecNodeMonitoringPort),
			Resources:      testResourceReqs,
			Locations: []ytv1.LocationSpec{
				testLocationChunkCache,
				testLocationSlots,
			},
			VolumeMounts:         testVolumeMounts,
			VolumeClaimTemplates: testVolumeClaimTemplates,
		},
		JobResources:     jobResources,
		ClusterNodesSpec: testClusterNodeSpec,
		JobProxyLoggers: []ytv1.TextLoggerSpec{
			{
				BaseLoggerSpec: ytv1.BaseLoggerSpec{
					Name:        "debug",
					Format:      ytv1.LogFormatPlainText,
					MinLogLevel: ytv1.LogLevelDebug,
					Compression: ytv1.LogCompressionZstd,
					RotationPolicy: &ytv1.LogRotationPolicy{
						RotationPeriodMilliseconds: &rotationPolicyMS,
						MaxTotalSizeToKeep:         &rotationPolicyMaxTotalSize,
					},
				},
				WriterType: ytv1.LogWriterTypeFile,
				CategoriesFilter: &ytv1.CategoriesFilter{
					Type:   ytv1.CategoriesFilterTypeExclude,
					Values: []string{"Bus", "Concurrency"},
				},
			},
		},
		Name: "end-a",
	}
}

func withCri(spec ytv1.ExecNodesSpec, jobResources *corev1.ResourceRequirements, isolated bool) ytv1.ExecNodesSpec {
	spec.Locations = append(spec.Locations, testLocationImageCache)
	spec.JobResources = jobResources
	spec.JobEnvironment = &ytv1.JobEnvironmentSpec{
		UserSlots: ptr.Int(42),
		CRI: &ytv1.CRIJobEnvironmentSpec{
			SandboxImage:           ptr.String("registry.k8s.io/pause:3.8"),
			APIRetryTimeoutSeconds: ptr.Int32(120),
			CRINamespace:           ptr.String("yt"),
			BaseCgroup:             ptr.String("/yt"),
		},
		UseArtifactBinds: ptr.Bool(true),
		DoNotSetUserId:   ptr.Bool(true),
		Isolated:         ptr.Bool(isolated),
	}
	return spec
}

func TestDoBuildExecNodeBase(t *testing.T) {
	yt := GetYtsaurus()
	ytsaurus := apiProxy.NewYtsaurus(yt, nil, nil, nil)

	l := labeller.Labeller{
		ObjectMeta:     ptr.T(ytsaurus.GetResource().ObjectMeta),
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: "end-test",
		ComponentName:  "ExecNode",
	}

	g := ytconfig.NewLocalNodeGenerator(yt, testClusterDomain)

	cases := map[string]struct {
		JobResources *corev1.ResourceRequirements
		Isolated     bool
	}{
		"isolated-containers-without-job-resources": {
			JobResources: nil,
			Isolated:     true,
		},
		"isolated-containers-with-job-resources": {
			JobResources: &testJobResourceReqs,
			Isolated:     true,
		},
		"single-container-without-job-resources": {
			JobResources: nil,
			Isolated:     false,
		},
		"single-container-with-job-resources": {
			JobResources: &testJobResourceReqs,
			Isolated:     false,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			execNodeSpec := withCri(getExecNodeSpec(nil), test.JobResources, test.Isolated)

			srv := newServer(
				&l,
				ytsaurus,
				ptr.T(execNodeSpec.InstanceSpec),
				"/usr/bin/ytserver-node",
				"ytserver-exec-node.yson",
				"end-test",
				"end-test",
				func() ([]byte, error) {
					return g.GetExecNodeConfig(execNodeSpec)
				},
				WithContainerPorts(corev1.ContainerPort{
					Name:          consts.YTRPCPortName,
					ContainerPort: consts.ExecNodeRPCPort,
					Protocol:      corev1.ProtocolTCP,
				}),
			)

			bEN := baseExecNode{
				server:        srv,
				cfgen:         g,
				spec:          &execNodeSpec,
				sidecarConfig: nil,
			}

			err := bEN.doBuildBase()
			require.NoError(t, err)

			cfg, err := g.GetExecNodeConfig(execNodeSpec)
			require.NoError(t, err)
			canonize.Assert(t, cfg)
		})
	}
}
