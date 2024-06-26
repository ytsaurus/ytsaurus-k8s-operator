package testutil

import (
	"fmt"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ptr "k8s.io/utils/pointer"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

const (
	YtsaurusName = "test-ytsaurus"
	// RemoteResourceName is a name for test remote ytsaurus and nodes.
	// It is short because of error:
	// `Failed to create pod sandbox: failed to construct FQDN from pod hostname and cluster domain, FQDN
	// <...> is too long (64 characters is the max, 67 characters requested)`.
	RemoteResourceName = "tst-rmt"
	// Images should be in sync with TEST_IMAGES variable in Makefile
	// todo: come up with a more elegant solution
	CoreImageFirst   = "ytsaurus/ytsaurus-nightly:dev-23.1-9779e0140ff73f5a786bd5362313ef9a74fcd0de"
	CoreImageSecond  = "ytsaurus/ytsaurus-nightly:dev-23.1-28ccaedbf353b870bedafb6e881ecf386a0a3779"
	CoreImageNextVer = "ytsaurus/ytsaurus-nightly:dev-23.2-9c50056eacfa4fe213798a5b9ee828ae3acb1bca"
)

var (
	masterVolumeSize, _ = resource.ParseQuantity("5Gi")
)

func createLoggersSpec() []ytv1.TextLoggerSpec {
	return []ytv1.TextLoggerSpec{
		{
			BaseLoggerSpec: ytv1.BaseLoggerSpec{
				MinLogLevel: ytv1.LogLevelInfo,
				Name:        "info-stderr",
			},
			WriterType: ytv1.LogWriterTypeStderr,
		},
	}
}

func CreateMinimalYtsaurusResource(namespace string) *ytv1.Ytsaurus {
	return &ytv1.Ytsaurus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      YtsaurusName,
			Namespace: namespace,
		},
		Spec: ytv1.YtsaurusSpec{
			CommonSpec: ytv1.CommonSpec{
				EphemeralCluster: true,
				UseShortNames:    true,
				CoreImage:        CoreImageFirst,
			},
			EnableFullUpdate: true,
			IsManaged:        true,
			Discovery: ytv1.DiscoverySpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount: 1,
				},
			},
			PrimaryMasters: ytv1.MastersSpec{
				MasterConnectionSpec: ytv1.MasterConnectionSpec{
					CellTag: 1,
				},
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount: 1,
					Locations: []ytv1.LocationSpec{
						{
							LocationType: "MasterChangelogs",
							Path:         "/yt/master-data/master-changelogs",
						},
						{
							LocationType: "MasterSnapshots",
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
										corev1.ResourceStorage: masterVolumeSize,
									},
								},
							},
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "master-data",
							MountPath: "/yt/master-data",
						},
					},
					Loggers: createLoggersSpec(),
				},
			},
			HTTPProxies: []ytv1.HTTPProxiesSpec{
				createHTTPProxiesSpec(),
			},
		},
	}
}

func CreateBaseYtsaurusResource(namespace string) *ytv1.Ytsaurus {
	ytsaurus := CreateMinimalYtsaurusResource(namespace)
	ytsaurus = WithBootstrap(ytsaurus)
	ytsaurus = WithScheduler(ytsaurus)
	ytsaurus = WithControllerAgents(ytsaurus)
	ytsaurus = WithDataNodes(ytsaurus)
	ytsaurus = WithTabletNodes(ytsaurus)
	ytsaurus = WithExecNodes(ytsaurus)
	return ytsaurus
}

// TODO (l0kix2): merge with ytconfig build spec helpers.
func WithDataNodes(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	return WithDataNodesCount(ytsaurus, 3)
}

func WithDataNodesCount(ytsaurus *ytv1.Ytsaurus, count int) *ytv1.Ytsaurus {
	ytsaurus.Spec.DataNodes = []ytv1.DataNodesSpec{
		{
			InstanceSpec: CreateDataNodeInstanceSpec(count),
		},
	}
	return ytsaurus
}

func WithTabletNodes(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	return WithTabletNodesCount(ytsaurus, 3)
}

func WithTabletNodesCount(ytsaurus *ytv1.Ytsaurus, count int) *ytv1.Ytsaurus {
	ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{
		{
			InstanceSpec: CreateTabletNodeSpec(count),
		},
	}
	return ytsaurus
}

func WithExecNodes(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.ExecNodes = []ytv1.ExecNodesSpec{
		{
			InstanceSpec: CreateExecNodeInstanceSpec(),
		},
	}
	return ytsaurus
}

func WithScheduler(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.Schedulers = &ytv1.SchedulersSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
		},
	}
	return ytsaurus
}

func WithControllerAgents(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.ControllerAgents = &ytv1.ControllerAgentsSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
		},
	}
	return ytsaurus
}

func WithBootstrap(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.Bootstrap = &ytv1.BootstrapSpec{
		TabletCellBundles: &ytv1.BundlesBootstrapSpec{
			Sys: &ytv1.BundleBootstrapSpec{
				TabletCellCount:        2,
				ChangelogPrimaryMedium: ptr.String("default"),
				SnapshotPrimaryMedium:  ptr.String("default"),
			},
			Default: &ytv1.BundleBootstrapSpec{
				TabletCellCount:        2,
				ChangelogPrimaryMedium: ptr.String("default"),
				SnapshotPrimaryMedium:  ptr.String("default"),
			},
		},
	}
	return ytsaurus
}

func WithQueryTracker(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
		},
	}
	return ytsaurus
}

func WithRPCProxies(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.RPCProxies = []ytv1.RPCProxiesSpec{
		createRPCProxiesSpec(),
	}
	return ytsaurus
}

func createHTTPProxiesSpec() ytv1.HTTPProxiesSpec {
	return ytv1.HTTPProxiesSpec{
		ServiceType: "NodePort",
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
		},
		HttpNodePort: getPortFromEnv("E2E_HTTP_PROXY_INTERNAL_PORT"),
	}
}

func createRPCProxiesSpec() ytv1.RPCProxiesSpec {
	stype := corev1.ServiceTypeNodePort
	return ytv1.RPCProxiesSpec{
		ServiceType: &stype,
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
		},
		NodePort: getPortFromEnv("E2E_RPC_PROXY_INTERNAL_PORT"),
	}
}

func getPortFromEnv(envvar string) *int32 {
	portStr := os.Getenv(envvar)
	if portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			panic(fmt.Sprintf("Invalid %s value", envvar))
		}
		return ptr.Int32(int32(port))
	}
	return nil
}

func CreateExecNodeInstanceSpec() ytv1.InstanceSpec {
	execNodeVolumeSize, _ := resource.ParseQuantity("3Gi")
	execNodeCPU, _ := resource.ParseQuantity("1")
	execNodeMemory, _ := resource.ParseQuantity("2Gi")
	return ytv1.InstanceSpec{
		InstanceCount: 1,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    execNodeCPU,
				corev1.ResourceMemory: execNodeMemory,
			},
		},
		Loggers: createLoggersSpec(),
		Locations: []ytv1.LocationSpec{
			{
				LocationType: "ChunkCache",
				Path:         "/yt/node-data/chunk-cache",
			},
			{
				LocationType: "Slots",
				Path:         "/yt/node-data/slots",
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "node-data",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						SizeLimit: &execNodeVolumeSize,
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "node-data",
				MountPath: "/yt/node-data",
			},
		},
	}
}

func CreateDataNodeInstanceSpec(instanceCount int) ytv1.InstanceSpec {
	return ytv1.InstanceSpec{
		InstanceCount: int32(instanceCount),
		Locations: []ytv1.LocationSpec{
			{
				LocationType: "ChunkStore",
				Path:         "/yt/node-data/chunk-store",
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "node-data",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						SizeLimit: &masterVolumeSize,
					},
				},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "node-data",
				MountPath: "/yt/node-data",
			},
		},
	}
}

func CreateTabletNodeSpec(instanceCount int) ytv1.InstanceSpec {
	return ytv1.InstanceSpec{
		InstanceCount: int32(instanceCount),
		Loggers:       createLoggersSpec(),
	}
}
