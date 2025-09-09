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
	OverridesName = "test-overrides"
	MasterPodName = "ms-0"
	// RemoteResourceName is a name for test remote ytsaurus and nodes.
	// It is short because of error:
	// `Failed to create pod sandbox: failed to construct FQDN from pod hostname and cluster domain, FQDN
	// <...> is too long (64 characters is the max, 67 characters requested)`.
	// FIXME(khlebnikov): https://github.com/ytsaurus/ytsaurus-k8s-operator/issues/390
	RemoteResourceName = "rmt"
)

// Images are should be set by TEST_ENV include in Makefile
var (
	// NOTE: The same image is used for YTsaurus integration tests.
	YtsaurusJobImage = GetenvOr("YTSAURUS_JOB_IMAGE", "docker.io/library/python:3.8-slim")

	YtsaurusImage23_2 = GetenvOr("YTSAURUS_IMAGE_23_2", "ghcr.io/ytsaurus/ytsaurus:stable-23.2.0")
	YtsaurusImage24_1 = GetenvOr("YTSAURUS_IMAGE_24_1", "ghcr.io/ytsaurus/ytsaurus:stable-24.1.0")
	YtsaurusImage24_2 = GetenvOr("YTSAURUS_IMAGE_24_2", "ghcr.io/ytsaurus/ytsaurus:stable-24.2.1")
	YtsaurusImage25_1 = GetenvOr("YTSAURUS_IMAGE_25_1", "")
	YtsaurusImage25_2 = GetenvOr("YTSAURUS_IMAGE_25_2", "")

	YtsaurusImagePrevious = GetenvOr("YTSAURUS_IMAGE_PREVIOUS", YtsaurusImage24_1)
	YtsaurusImageCurrent  = GetenvOr("YTSAURUS_IMAGE_CURRENT", YtsaurusImage24_2)
	YtsaurusImageFuture   = GetenvOr("YTSAURUS_IMAGE_FUTURE", YtsaurusImage25_1)
	YtsaurusImageNightly  = GetenvOr("YTSAURUS_IMAGE_NIGHTLY", "")

	YtsaurusTLSReady = os.Getenv("YTSAURUS_TLS_READY") != ""

	QueryTrackerImagePrevious = GetenvOr("QUERY_TRACKER_IMAGE_PREVIOUS", "ghcr.io/ytsaurus/query-tracker:0.0.9")
	QueryTrackerImageCurrent  = GetenvOr("QUERY_TRACKER_IMAGE_CURRENT", "ghcr.io/ytsaurus/query-tracker:0.0.10")
	QueryTrackerImageFuture   = GetenvOr("QUERY_TRACKER_IMAGE_FUTURE", "")
	QueryTrackerImageNightly  = GetenvOr("QUERY_TRACKER_IMAGE_NIGHTLY", "")

	StrawberryImagePrevious = GetenvOr("STRAWBERRY_IMAGE_PREVIOUS", "ghcr.io/ytsaurus/strawberry:0.0.12")
	StrawberryImageCurrent  = GetenvOr("STRAWBERRY_IMAGE_CURRENT", "ghcr.io/ytsaurus/strawberry:0.0.13")
	StrawberryImageFuture   = GetenvOr("STRAWBERRY_IMAGE_FUTURE", "")
	StrawberryImageNightly  = GetenvOr("STRAWBERRY_IMAGE_NIGHTLY", "")

	ChytImagePrevious = GetenvOr("CHYT_IMAGE_PREVIOUS", "ghcr.io/ytsaurus/chyt:2.16.0")
	ChytImageCurrent  = GetenvOr("CHYT_IMAGE_CURRENT", "ghcr.io/ytsaurus/chyt:2.17.2")
	ChytImageFuture   = GetenvOr("CHYT_IMAGE_FUTURE", "")
	ChytImageNightly  = GetenvOr("CHYT_IMAGE_NIGHTLY", "")
)

var (
	masterVolumeSize   = resource.MustParse("5Gi")
	dataNodeVolumeSize = resource.MustParse("10Gi")
	execNodeVolumeSize = resource.MustParse("5Gi")
)

type YtsaurusBuilder struct {
	Namespace         string
	YtsaurusImage     string
	JobImage          *string
	SandboxImage      *string
	CRIService        *ytv1.CRIServiceType
	QueryTrackerImage string
	Ytsaurus          *ytv1.Ytsaurus
	Overrides         *corev1.ConfigMap

	// Set MinReadyInstanceCount for all components
	MinReadyInstanceCount *int
}

func (b *YtsaurusBuilder) CreateVolumeClaim(name string, size resource.Quantity) ytv1.EmbeddedPersistentVolumeClaim {
	return ytv1.EmbeddedPersistentVolumeClaim{
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
				CoreImage:        b.YtsaurusImage,
				JobImage:         b.JobImage,
			},
			EnableFullUpdate: true,
			IsManaged:        true,
			Discovery: ytv1.DiscoverySpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount:         1,
					MinReadyInstanceCount: b.MinReadyInstanceCount,
					Loggers:               b.CreateLoggersSpec(),
				},
			},
			PrimaryMasters: ytv1.MastersSpec{
				MasterConnectionSpec: ytv1.MasterConnectionSpec{
					CellTag: 1,
				},
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount:         1,
					MinReadyInstanceCount: b.MinReadyInstanceCount,
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
	b.WithMasterCaches()
	b.WithBootstrap()
	b.WithScheduler()
	b.WithControllerAgents()
	b.WithDataNodes()
	b.WithTabletNodes()
	b.WithExecNodes()
}

func CreateBaseYtsaurusResource(namespace string) *ytv1.Ytsaurus {
	builder := YtsaurusBuilder{
		Namespace:         namespace,
		YtsaurusImage:     YtsaurusImageCurrent,
		JobImage:          ptr.To(YtsaurusJobImage),
		QueryTrackerImage: QueryTrackerImageCurrent,
	}
	builder.CreateMinimal()
	builder.WithBaseComponents()
	return builder.Ytsaurus
}

func (b *YtsaurusBuilder) WithOverrides() {
	b.Overrides = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OverridesName,
			Namespace: b.Namespace,
		},
		Data: map[string]string{
			"place-holder": "",
		},
	}
	b.Ytsaurus.Spec.ConfigOverrides = &corev1.LocalObjectReference{
		Name: OverridesName,
	}
}

func (b *YtsaurusBuilder) WithMasterCaches() {
	b.Ytsaurus.Spec.MasterCaches = &ytv1.MasterCachesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Loggers:               b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithClusterFeatures() {
	b.Ytsaurus.Spec.ClusterFeatures = &ytv1.ClusterFeatures{
		HTTPProxyHaveChytAddress: true,
	}
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
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Loggers:               b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithControllerAgents() {
	b.Ytsaurus.Spec.ControllerAgents = &ytv1.ControllerAgentsSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Loggers:               b.CreateLoggersSpec(),
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
			Image:                 ptr.To(b.QueryTrackerImage),
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Loggers:               b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithYqlAgent() {
	b.Ytsaurus.Spec.YQLAgents = &ytv1.YQLAgentSpec{
		InstanceSpec: ytv1.InstanceSpec{
			Image:                 ptr.To(b.QueryTrackerImage),
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Loggers:               b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithQueueAgent() {
	image := b.YtsaurusImage
	// Older version doesn't have /usr/bin/ytserver-queue-agent
	if image == YtsaurusImage23_2 {
		image = YtsaurusImage24_1
	}
	b.Ytsaurus.Spec.QueueAgents = &ytv1.QueueAgentSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Loggers:               b.CreateLoggersSpec(),
			Image:                 ptr.To(image),
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
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Loggers:               b.CreateLoggersSpec(),
		},
		HttpNodePort: getPortFromEnv("E2E_HTTP_PROXY_INTERNAL_PORT"),
	}
}

func (b *YtsaurusBuilder) CreateRPCProxiesSpec() ytv1.RPCProxiesSpec {
	stype := corev1.ServiceTypeNodePort
	return ytv1.RPCProxiesSpec{
		ServiceType: &stype,
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Loggers:               b.CreateLoggersSpec(),
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
	return ytv1.ExecNodesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Loggers:               b.CreateLoggersSpec(),
			Locations: []ytv1.LocationSpec{
				{
					LocationType: ytv1.LocationTypeChunkCache,
					Path:         "/yt/node-data/chunk-cache",
				},
				{
					LocationType: ytv1.LocationTypeSlots,
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

func (b *YtsaurusBuilder) SetupCRIJobEnvironment(node *ytv1.ExecNodesSpec) {
	node.Locations = append(node.Locations, ytv1.LocationSpec{
		LocationType: ytv1.LocationTypeImageCache,
		Path:         "/yt/node-data/image-cache",
	})
	node.JobEnvironment = &ytv1.JobEnvironmentSpec{
		UserSlots: ptr.To(4),
		CRI: &ytv1.CRIJobEnvironmentSpec{
			CRIService:   b.CRIService,
			SandboxImage: b.SandboxImage,
		},
	}
}

func (b *YtsaurusBuilder) WithCRIJobEnvironment() {
	for i := range b.Ytsaurus.Spec.ExecNodes {
		b.SetupCRIJobEnvironment(&b.Ytsaurus.Spec.ExecNodes[i])
	}
}

func (b *YtsaurusBuilder) CreateDataNodeInstanceSpec(instanceCount int32) ytv1.InstanceSpec {
	return ytv1.InstanceSpec{
		InstanceCount:         instanceCount,
		MinReadyInstanceCount: b.MinReadyInstanceCount,
		Loggers:               b.CreateLoggersSpec(),
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
		InstanceCount:         instanceCount,
		MinReadyInstanceCount: b.MinReadyInstanceCount,
		Loggers:               b.CreateLoggersSpec(),
	}
}

func (b *YtsaurusBuilder) WithStrawberryController() {
	b.Ytsaurus.Spec.StrawberryController = &ytv1.StrawberryControllerSpec{
		Image: ptr.To(StrawberryImageCurrent),
	}
}

func (b *YtsaurusBuilder) CreateChyt() *ytv1.Chyt {
	chyt := ytv1.Chyt{
		ObjectMeta: metav1.ObjectMeta{
			Name:      YtsaurusName,
			Namespace: b.Namespace,
		},
		Spec: ytv1.ChytSpec{
			Ytsaurus: &corev1.LocalObjectReference{
				Name: YtsaurusName,
			},
			Image:              ChytImageCurrent,
			MakeDefault:        true,
			CreatePublicClique: ptr.To(true),
		},
	}
	return &chyt
}
