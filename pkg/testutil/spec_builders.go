package testutil

import (
	"fmt"
	"os"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

const (
	YtsaurusName  = "test-ytsaurus"
	MasterPodName = "ms-0"
	// RemoteResourceName is a name for test remote ytsaurus and nodes.
	// It is short because of error:
	// `Failed to create pod sandbox: failed to construct FQDN from pod hostname and cluster domain, FQDN
	// <...> is too long (64 characters is the max, 67 characters requested)`.
	// FIXME(khlebnikov): https://github.com/ytsaurus/ytsaurus-k8s-operator/issues/390
	RemoteResourceName = "rmt"
	// Images should be in sync with TEST_IMAGES variable in Makefile
	// todo: come up with a more elegant solution
	CoreImageFirst   = "ghcr.io/ytsaurus/ytsaurus:stable-23.2.0"
	CoreImageSecond  = "ghcr.io/ytsaurus/ytsaurus:stable-24.1.0"
	CoreImageNextVer = "ghcr.io/ytsaurus/ytsaurus-nightly:dev-24.2-2025-03-19-2973ab7cb36ed53ae3cbe9c37b8c7f55eb9c4e77"
)

var (
	masterVolumeSize, _ = resource.ParseQuantity("5Gi")
	dataNodeVolumeSize, _ = resource.ParseQuantity("5Gi")
	execNodeVolumeSize, _ = resource.ParseQuantity("5Gi")
)

type YtsaurusBuilder struct {
	Namespace string

	Ytsaurus *ytv1.Ytsaurus
}

func (b *YtsaurusBuilder) CreateVolumeClaim(name string, size resource.Quantity) ytv1.EmbeddedPersistentVolumeClaim {
	return ytv1.EmbeddedPersistentVolumeClaim {
		EmbeddedObjectMetadata: ytv1.EmbeddedObjectMetadata{
			Name: name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: size,
				},
			},
		},
	}
}

func (b *YtsaurusBuilder) CreateLoggersSpec() []ytv1.TextLoggerSpec {
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

func (b *YtsaurusBuilder) CreateMinimal() {
	b.Ytsaurus = &ytv1.Ytsaurus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      YtsaurusName,
			Namespace: b.Namespace,
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
					Loggers:       b.CreateLoggersSpec(),
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
					VolumeClaimTemplates:  []ytv1.EmbeddedPersistentVolumeClaim{
						b.CreateVolumeClaim("master-data", masterVolumeSize),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "master-data",
							MountPath: "/yt/master-data",
						},
					},
					Loggers: b.CreateLoggersSpec(),
				},
			},
			HTTPProxies: []ytv1.HTTPProxiesSpec{
				b.CreateHTTPProxiesSpec(),
			},
		},
	}
}

func (b *YtsaurusBuilder) WithBaseComponents() {
	b.WithBootstrap()
	b.WithScheduler()
	b.WithControllerAgents()
	b.WithDataNodes()
	b.WithTabletNodes()
	b.WithExecNodes()
}

func CreateBaseYtsaurusResource(namespace string) *ytv1.Ytsaurus {
	builder := YtsaurusBuilder{
		Namespace: namespace,
	}
	builder.CreateMinimal()
	builder.WithBaseComponents()
	return builder.Ytsaurus
}

// TODO (l0kix2): merge with ytconfig build spec helpers.
func (b *YtsaurusBuilder) WithDataNodes() {
	b.WithDataNodesCount(3, nil)
}

func (b *YtsaurusBuilder) WithNamedDataNodes(name *string) {
	b.WithDataNodesCount(3, name)
}

func (b *YtsaurusBuilder) WithDataNodesCount(count int32, name *string) {
	dataNodeSpec := ytv1.DataNodesSpec{
		InstanceSpec: b.CreateDataNodeInstanceSpec(count),
	}
	if name != nil {
		dataNodeSpec.Name = *name
	}
	b.Ytsaurus.Spec.DataNodes = append(b.Ytsaurus.Spec.DataNodes, dataNodeSpec)
}

func (b *YtsaurusBuilder) WithTabletNodes() {
	b.WithTabletNodesCount(3)
}

func (b *YtsaurusBuilder) WithTabletNodesCount(count int32) {
	b.Ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{
		{
			InstanceSpec: b.CreateTabletNodeSpec(count),
		},
	}
}

func (b *YtsaurusBuilder) WithExecNodes() {
	b.Ytsaurus.Spec.ExecNodes = []ytv1.ExecNodesSpec{
		b.CreateExecNodeSpec(),
	}
}

func (b *YtsaurusBuilder) WithScheduler() {
	b.Ytsaurus.Spec.Schedulers = &ytv1.SchedulersSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
			Loggers:       b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithControllerAgents() {
	b.Ytsaurus.Spec.ControllerAgents = &ytv1.ControllerAgentsSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
			Loggers:       b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithBootstrap() {
	b.Ytsaurus.Spec.Bootstrap = &ytv1.BootstrapSpec{
		TabletCellBundles: &ytv1.BundlesBootstrapSpec{
			Sys: &ytv1.BundleBootstrapSpec{
				TabletCellCount:        2,
				ChangelogPrimaryMedium: ptr.To("default"),
				SnapshotPrimaryMedium:  ptr.To("default"),
			},
			Default: &ytv1.BundleBootstrapSpec{
				TabletCellCount:        2,
				ChangelogPrimaryMedium: ptr.To("default"),
				SnapshotPrimaryMedium:  ptr.To("default"),
			},
		},
	}
}

func (b *YtsaurusBuilder) WithQueryTracker() {
	b.Ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
			Loggers:       b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithYqlAgent() {
	b.Ytsaurus.Spec.YQLAgents = &ytv1.YQLAgentSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
			Loggers:       b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithQueueAgent() {
	b.Ytsaurus.Spec.QueueAgents = &ytv1.QueueAgentSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
			Loggers:       b.CreateLoggersSpec(),
			// Older version doesn't have /usr/bin/ytserver-queue-agent
			Image: ptr.To(CoreImageSecond),
		},
	}
}

func (b *YtsaurusBuilder) WithRPCProxies() {
	b.Ytsaurus.Spec.RPCProxies = []ytv1.RPCProxiesSpec{
		b.CreateRPCProxiesSpec(),
	}
}

func (b *YtsaurusBuilder) CreateHTTPProxiesSpec() ytv1.HTTPProxiesSpec {
	return ytv1.HTTPProxiesSpec{
		ServiceType: "NodePort",
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
			Loggers:       b.CreateLoggersSpec(),
		},
		HttpNodePort: getPortFromEnv("E2E_HTTP_PROXY_INTERNAL_PORT"),
	}
}

func (b *YtsaurusBuilder) CreateRPCProxiesSpec() ytv1.RPCProxiesSpec {
	stype := corev1.ServiceTypeNodePort
	return ytv1.RPCProxiesSpec{
		ServiceType: &stype,
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
			Loggers:       b.CreateLoggersSpec(),
		},
		NodePort: getPortFromEnv("E2E_RPC_PROXY_INTERNAL_PORT"),
	}
}

func getPortFromEnv(envvar string) *int32 {
	portStr := os.Getenv(envvar)
	if portStr != "" {
		port, err := strconv.ParseInt(portStr, 10, 32)
		if err != nil {
			panic(fmt.Sprintf("Invalid %s value", envvar))
		}
		return ptr.To(int32(port))
	}
	return nil
}

func (b *YtsaurusBuilder) CreateExecNodeSpec() ytv1.ExecNodesSpec {
	execNodeCPU, _ := resource.ParseQuantity("1")
	execNodeMemory, _ := resource.ParseQuantity("2Gi")

	return ytv1.ExecNodesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 1,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    execNodeCPU,
					corev1.ResourceMemory: execNodeMemory,
				},
			},
			Loggers: b.CreateLoggersSpec(),
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
			VolumeClaimTemplates: []ytv1.EmbeddedPersistentVolumeClaim{
				b.CreateVolumeClaim("node-data", execNodeVolumeSize),
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "node-data",
					MountPath: "/yt/node-data",
				},
			},
		},
	}
}

func (b *YtsaurusBuilder) CreateDataNodeInstanceSpec(instanceCount int32) ytv1.InstanceSpec {
	return ytv1.InstanceSpec{
		InstanceCount: instanceCount,
		Loggers:       b.CreateLoggersSpec(),
		Locations: []ytv1.LocationSpec{
			{
				LocationType: "ChunkStore",
				Path:         "/yt/node-data/chunk-store",
			},
		},
		VolumeClaimTemplates: []ytv1.EmbeddedPersistentVolumeClaim{
			b.CreateVolumeClaim("node-data", dataNodeVolumeSize),
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "node-data",
				MountPath: "/yt/node-data",
			},
		},
	}
}

func (b *YtsaurusBuilder) CreateTabletNodeSpec(instanceCount int32) ytv1.InstanceSpec {
	return ytv1.InstanceSpec{
		InstanceCount: instanceCount,
		Loggers:       b.CreateLoggersSpec(),
	}
}
