package v1

import (
	"fmt"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ptr "k8s.io/utils/pointer"
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

func createLoggersSpec() []TextLoggerSpec {
	return []TextLoggerSpec{
		{
			BaseLoggerSpec: BaseLoggerSpec{
				MinLogLevel: LogLevelInfo,
				Name:        "info-stderr",
			},
			WriterType: LogWriterTypeStderr,
		},
	}
}

func CreateMinimalYtsaurusResource(namespace string) *Ytsaurus {
	return &Ytsaurus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      YtsaurusName,
			Namespace: namespace,
		},
		Spec: YtsaurusSpec{
			CommonSpec: CommonSpec{
				EphemeralCluster: true,
				UseShortNames:    true,
				CoreImage:        CoreImageFirst,
			},
			EnableFullUpdate: true,
			IsManaged:        true,
			Discovery: DiscoverySpec{
				InstanceSpec: InstanceSpec{
					InstanceCount: 1,
				},
			},
			PrimaryMasters: MastersSpec{
				MasterConnectionSpec: MasterConnectionSpec{
					CellTag: 1,
				},
				InstanceSpec: InstanceSpec{
					InstanceCount: 1,
					Locations: []LocationSpec{
						{
							LocationType: "MasterChangelogs",
							Path:         "/yt/master-data/master-changelogs",
						},
						{
							LocationType: "MasterSnapshots",
							Path:         "/yt/master-data/master-snapshots",
						},
					},
					VolumeClaimTemplates: []EmbeddedPersistentVolumeClaim{
						{
							EmbeddedObjectMetadata: EmbeddedObjectMetadata{
								Name: "master-data",
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
								Resources: corev1.VolumeResourceRequirements{
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
			HTTPProxies: []HTTPProxiesSpec{
				createHTTPProxiesSpec(),
			},
		},
	}
}

func CreateBaseYtsaurusResource(namespace string) *Ytsaurus {
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
func WithDataNodes(ytsaurus *Ytsaurus) *Ytsaurus {
	return WithDataNodesCount(ytsaurus, 3)
}

func WithDataNodesCount(ytsaurus *Ytsaurus, count int) *Ytsaurus {
	ytsaurus.Spec.DataNodes = []DataNodesSpec{
		{
			InstanceSpec: CreateDataNodeInstanceSpec(count),
		},
	}
	return ytsaurus
}

func WithTabletNodes(ytsaurus *Ytsaurus) *Ytsaurus {
	return WithTabletNodesCount(ytsaurus, 3)
}

func WithTabletNodesCount(ytsaurus *Ytsaurus, count int) *Ytsaurus {
	ytsaurus.Spec.TabletNodes = []TabletNodesSpec{
		{
			InstanceSpec: CreateTabletNodeSpec(count),
		},
	}
	return ytsaurus
}

func WithExecNodes(ytsaurus *Ytsaurus) *Ytsaurus {
	ytsaurus.Spec.ExecNodes = []ExecNodesSpec{
		{
			InstanceSpec: CreateExecNodeInstanceSpec(),
		},
	}
	return ytsaurus
}

func WithScheduler(ytsaurus *Ytsaurus) *Ytsaurus {
	ytsaurus.Spec.Schedulers = &SchedulersSpec{
		InstanceSpec: InstanceSpec{
			InstanceCount: 1,
		},
	}
	return ytsaurus
}

func WithControllerAgents(ytsaurus *Ytsaurus) *Ytsaurus {
	ytsaurus.Spec.ControllerAgents = &ControllerAgentsSpec{
		InstanceSpec: InstanceSpec{
			InstanceCount: 1,
		},
	}
	return ytsaurus
}

func WithBootstrap(ytsaurus *Ytsaurus) *Ytsaurus {
	ytsaurus.Spec.Bootstrap = &BootstrapSpec{
		TabletCellBundles: &BundlesBootstrapSpec{
			Sys: &BundleBootstrapSpec{
				TabletCellCount:        2,
				ChangelogPrimaryMedium: ptr.String("default"),
				SnapshotPrimaryMedium:  ptr.String("default"),
			},
			Default: &BundleBootstrapSpec{
				TabletCellCount:        2,
				ChangelogPrimaryMedium: ptr.String("default"),
				SnapshotPrimaryMedium:  ptr.String("default"),
			},
		},
	}
	return ytsaurus
}

func WithQueryTracker(ytsaurus *Ytsaurus) *Ytsaurus {
	ytsaurus.Spec.QueryTrackers = &QueryTrackerSpec{
		InstanceSpec: InstanceSpec{
			InstanceCount: 1,
		},
	}
	return ytsaurus
}

func WithRPCProxies(ytsaurus *Ytsaurus) *Ytsaurus {
	ytsaurus.Spec.RPCProxies = []RPCProxiesSpec{
		createRPCProxiesSpec(),
	}
	return ytsaurus
}

func createHTTPProxiesSpec() HTTPProxiesSpec {
	return HTTPProxiesSpec{
		ServiceType: "NodePort",
		InstanceSpec: InstanceSpec{
			InstanceCount: 1,
		},
		HttpNodePort: getPortFromEnv("E2E_HTTP_PROXY_INTERNAL_PORT"),
	}
}

func createRPCProxiesSpec() RPCProxiesSpec {
	stype := corev1.ServiceTypeNodePort
	return RPCProxiesSpec{
		ServiceType: &stype,
		InstanceSpec: InstanceSpec{
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

func CreateExecNodeInstanceSpec() InstanceSpec {
	execNodeVolumeSize, _ := resource.ParseQuantity("3Gi")
	execNodeCPU, _ := resource.ParseQuantity("1")
	execNodeMemory, _ := resource.ParseQuantity("2Gi")
	return InstanceSpec{
		InstanceCount: 1,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    execNodeCPU,
				corev1.ResourceMemory: execNodeMemory,
			},
		},
		Loggers: createLoggersSpec(),
		Locations: []LocationSpec{
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

func CreateDataNodeInstanceSpec(instanceCount int) InstanceSpec {
	return InstanceSpec{
		InstanceCount: int32(instanceCount),
		Locations: []LocationSpec{
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

func CreateTabletNodeSpec(instanceCount int) InstanceSpec {
	return InstanceSpec{
		InstanceCount: int32(instanceCount),
		Loggers:       createLoggersSpec(),
	}
}
