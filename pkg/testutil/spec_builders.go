package testutil

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/utils/ptr"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/version"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

const (
	CellTag       = 1
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

type YtsaurusImages struct {
	YtsaurusVersion     version.Version
	QueryTrackerVersion version.Version

	Job          string
	Core         string
	Strawberry   string
	Chyt         string
	QueryTracker string
}

// Images are should be set by TEST_ENV include in Makefile
var (
	YtsaurusJobImage    = GetenvOr("YTSAURUS_JOB_IMAGE", "mirror.gcr.io/library/python:3.12-slim")
	YtsaurusRegistry    = GetenvOr("YTSAURUS_REGISTRY", "ghcr.io/ytsaurus")
	YtsaurusPrevVersion = os.Getenv("TEST_YTSAURUS_PREV_VERSION")
	YtsaurusCurrVersion = os.Getenv("TEST_YTSAURUS_VERSION")
	YtsaurusNextVersion = os.Getenv("TEST_YTSAURUS_NEXT_VERSION")
	YtsaurusLateVersion = os.Getenv("TEST_YTSAURUS_LATE_VERSION")

	Images map[string]YtsaurusImages = map[string]YtsaurusImages{}

	PrevImages, TestImages, NextImages YtsaurusImages
)

type ImageEntry struct {
	Epoch, Version, Image string
}

func init() {
	images := map[string]map[string]ImageEntry{}
	names := []string{"YTSAURUS", "STRAWBERRY", "CHYT", "QUERY_TRACKER"}
	epochs := []string{"PAST", "PREVIOUS", "CURRENT", "COMING", "FUTURE", "NIGHTLY"}

	if YtsaurusCurrVersion == "" {
		YtsaurusCurrVersion = "CURRENT"
	}

	// Seed some default versions if nothing is set in environment.
	defaultVersions := map[string]string{
		"YTSAURUS_VERSION_CURRENT":      "25.2.2",
		"STRAWBERRY_VERSION_CURRENT":    "0.0.15",
		"CHYT_VERSION_CURRENT":          "2.17.4",
		"QUERY_TRACKER_VERSION_CURRENT": "0.1.2",
	}

	for _, name := range names {
		images[name] = map[string]ImageEntry{}
		for _, epoch := range epochs {
			ver := GetenvOr(name+"_VERSION_"+epoch, defaultVersions[name+"_VERSION_"+epoch])
			if ver == "" {
				continue
			}

			tag := ver
			if name == "YTSAURUS" {
				// Ytsaurus does not follow tag version notation ¯\_(ツ)_/¯.
				tag = "stable-" + ver
			}
			image := YtsaurusRegistry + "/" + strings.ToLower(strings.ReplaceAll(name, "_", "-"))
			image = GetenvOr(name+"_IMAGE", image) + ":" + tag
			image = GetenvOr(name+"_IMAGE_"+strings.ReplaceAll(ver, ".", "_"), image)

			entry := ImageEntry{
				Epoch:   epoch,
				Version: ver,
				Image:   image,
			}
			images[name][epoch] = entry
			images[name][ver] = entry
			if v, err := version.ParseVersion(ver); err == nil {
				majmin := fmt.Sprintf("%v.%v", v.Major(), v.Minor())
				images[name][majmin] = entry
			}
		}
	}

	// Add directly set ytsaurus version.
	for _, ver := range []string{YtsaurusPrevVersion, YtsaurusCurrVersion, YtsaurusNextVersion, YtsaurusLateVersion} {
		if _, ok := images["YTSAURUS"][ver]; !ok && ver != "" {
			images["YTSAURUS"][ver] = ImageEntry{
				Epoch:   "CURRENT",
				Version: ver,
				Image:   GetenvOr("YTSAURUS_IMAGE", YtsaurusRegistry+"/ytsaurus") + ":stable-" + ver,
			}
		}
	}

	for key, yt := range images["YTSAURUS"] {
		choose := func(name string) ImageEntry {
			if e, ok := images[name][yt.Epoch]; ok {
				return e
			}
			return images[name][YtsaurusCurrVersion]
		}
		ytsaurusVersion, err := version.ParseYtsaurusVersion(yt.Version)
		if err != nil {
			panic(fmt.Sprintf("key %q version %q - %v", key, yt.Version, err))
		}
		queryTracker := choose("QUERY_TRACKER")
		Images[key] = YtsaurusImages{
			YtsaurusVersion:     *ytsaurusVersion,
			QueryTrackerVersion: *version.MustParse(queryTracker.Version),

			Job:          YtsaurusJobImage,
			Core:         yt.Image,
			Strawberry:   choose("STRAWBERRY").Image,
			Chyt:         choose("CHYT").Image,
			QueryTracker: queryTracker.Image,
		}
	}

	if index := slices.Index(epochs, images["YTSAURUS"][YtsaurusCurrVersion].Epoch); index != -1 {
		if YtsaurusPrevVersion == "" && index > 0 {
			YtsaurusPrevVersion = epochs[index-1]
		}
		if YtsaurusNextVersion == "" && index < len(epochs)-1 {
			YtsaurusNextVersion = epochs[index+1]
		}
		if YtsaurusLateVersion == "" && index < len(epochs)-2 {
			YtsaurusLateVersion = epochs[index+2]
		}
	}
	PrevImages = Images[YtsaurusPrevVersion]
	TestImages = Images[YtsaurusCurrVersion]
	NextImages = Images[YtsaurusNextVersion]
}

var (
	masterVolumeSize   = resource.MustParse("5Gi")
	dataNodeVolumeSize = resource.MustParse("10Gi")
	execNodeVolumeSize = resource.MustParse("5Gi")

	defaultNodeResources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("0"),
			corev1.ResourceMemory: resource.MustParse("0"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}
	dataNodeResources   = defaultNodeResources
	tabletNodeResources = defaultNodeResources
	execNodeResources   = defaultNodeResources

	execNodeJobResources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("0"),
			corev1.ResourceMemory: resource.MustParse("0"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("4"),
			corev1.ResourceMemory: resource.MustParse("8Gi"),
		},
	}
)

type YtsaurusBuilder struct {
	Images       YtsaurusImages
	Namespace    string
	SandboxImage *string
	CRIService   *ytv1.CRIServiceType
	Ytsaurus     *ytv1.Ytsaurus
	Overrides    *corev1.ConfigMap
	Chyt         *ytv1.Chyt
	Spyt         *ytv1.Spyt

	// Set MinReadyInstanceCount for all components
	MinReadyInstanceCount *int

	WithHTTPSProxy     bool
	WithHTTPSOnlyProxy bool
	WithRPCProxy       bool
	WithRPCProxyTLS    bool
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
				CoreImage:        b.Images.Core,
				JobImage:         ptr.To(b.Images.Job),
			},
			Discovery: ytv1.DiscoverySpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount:         1,
					MinReadyInstanceCount: b.MinReadyInstanceCount,
					Resources:             *defaultNodeResources.DeepCopy(),
					Loggers:               b.CreateLoggersSpec(),
				},
			},
			PrimaryMasters: ytv1.MastersSpec{
				MasterConnectionSpec: ytv1.MasterConnectionSpec{
					CellTag: CellTag,
				},
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount:         1,
					MinReadyInstanceCount: b.MinReadyInstanceCount,
					Resources:             *defaultNodeResources.DeepCopy(),
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

func (b *YtsaurusBuilder) WithNativeTransportTLS(serverCert, clientCert string) {
	b.Ytsaurus.Spec.CABundle = &ytv1.FileObjectReference{
		Name: TestCABundleName,
	}

	transport := ytv1.RPCTransportSpec{
		TLSSecret: &corev1.LocalObjectReference{
			Name: serverCert,
		},
		TLSRequired:                true,
		TLSInsecure:                true,
		TLSPeerAlternativeHostName: b.Ytsaurus.Name,
	}

	if clientCert != "" {
		transport.TLSClientSecret = &corev1.LocalObjectReference{
			Name: clientCert,
		}
		transport.TLSInsecure = false
	}

	b.Ytsaurus.Spec.NativeTransport = &transport
}

func (b *YtsaurusBuilder) WithHTTPSProxies(httpsCert string, httpsOnly bool) {
	b.WithHTTPSProxy = true
	b.WithHTTPSOnlyProxy = httpsOnly

	b.Ytsaurus.Spec.ClusterFeatures.HTTPProxyHaveHTTPSAddress = true

	b.Ytsaurus.Spec.CARootBundle = &ytv1.FileObjectReference{
		Name: TestCARootBundleName,
		Items: []corev1.KeyToPath{
			{
				Key:  "ca-certificates.crt",
				Path: "ca-certificates.crt",
			},
			{
				Key:  "ca-certificates.jks",
				Path: "java/cacerts",
			},
		},
	}

	for i := range b.Ytsaurus.Spec.HTTPProxies {
		b.Ytsaurus.Spec.HTTPProxies[i].Transport = ytv1.HTTPTransportSpec{
			HTTPSSecret: &corev1.LocalObjectReference{
				Name: httpsCert,
			},
			DisableHTTP: httpsOnly,
		}
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
			Resources:             *defaultNodeResources.DeepCopy(),
			Loggers:               b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithAllClusterFeatures() {
	b.Ytsaurus.Spec.ClusterFeatures = &ytv1.ClusterFeatures{
		RPCProxyHavePublicAddress: true,
		HTTPProxyHaveChytAddress:  true,
		HTTPProxyHaveHTTPSAddress: true,
		SecureClusterTransports:   false, // Turned off to increase coverage.
	}
}

func (b *YtsaurusBuilder) WithAllGlobalPodOptions() {
	b.Ytsaurus.Spec.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "global-image-pull-secrets"}}
	// FIXME: set on everything not just pods
	b.Ytsaurus.Spec.ExtraPodAnnotations = map[string]string{"extra-pod-annotation": "true"}
	b.Ytsaurus.Spec.PodAnnotations = map[string]string{"global-pod-annotation": "true"}
	b.Ytsaurus.Spec.PodLabels = map[string]string{"global-pod-label": "true"}
	b.Ytsaurus.Spec.DNSPolicy = ptr.To(corev1.DNSClusterFirst)
	b.Ytsaurus.Spec.DNSConfig = &corev1.PodDNSConfig{Options: []corev1.PodDNSConfigOption{{Name: "global-dns-option"}}}
	b.Ytsaurus.Spec.HostNetwork = ptr.To(true)
	b.Ytsaurus.Spec.NodeSelector = map[string]string{"global-node-selector": "true"}
	b.Ytsaurus.Spec.RuntimeClassName = ptr.To("global-runtime-class")
	b.Ytsaurus.Spec.SetHostnameAsFQDN = ptr.To(false)
	b.Ytsaurus.Spec.Tolerations = []corev1.Toleration{{Key: "global-toleration"}}
}

func (b *YtsaurusBuilder) WithAllInstancePodOptions(spec *ytv1.InstanceSpec) {
	spec.Affinity = &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      "instance-affinity",
						Operator: corev1.NodeSelectorOpExists,
					}},
				}},
			},
		},
	}
	spec.DNSConfig = &corev1.PodDNSConfig{Options: []corev1.PodDNSConfigOption{{Name: "instance-dns-option"}}}
	spec.DNSPolicy = ptr.To(corev1.DNSNone)
	spec.HostNetwork = ptr.To(false)
	spec.NodeSelector = map[string]string{"instance-node-selector": "true"}
	spec.PodAnnotations = map[string]string{"instance-pod-annotation": "true"}
	spec.PodLabels = map[string]string{"instance-pod-label": "true"}
	spec.RuntimeClassName = ptr.To("instance-runtime-class")
	spec.SetHostnameAsFQDN = ptr.To(true)
	spec.Tolerations = []corev1.Toleration{{Key: "instance-toleration"}}
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
			Resources:             *defaultNodeResources.DeepCopy(),
			Loggers:               b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithControllerAgents() {
	b.Ytsaurus.Spec.ControllerAgents = &ytv1.ControllerAgentsSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Resources:             *defaultNodeResources.DeepCopy(),
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
			Image:                 ptr.To(b.Images.QueryTracker),
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Resources:             *defaultNodeResources.DeepCopy(),
			Loggers:               b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithYqlAgent() {
	b.Ytsaurus.Spec.YQLAgents = &ytv1.YQLAgentSpec{
		InstanceSpec: ytv1.InstanceSpec{
			Image:                 ptr.To(b.Images.QueryTracker),
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Resources:             *defaultNodeResources.DeepCopy(),
			Loggers:               b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithQueueAgent() {
	b.Ytsaurus.Spec.QueueAgents = &ytv1.QueueAgentSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:         1,
			MinReadyInstanceCount: b.MinReadyInstanceCount,
			Resources:             *defaultNodeResources.DeepCopy(),
			Loggers:               b.CreateLoggersSpec(),
		},
	}
}

func (b *YtsaurusBuilder) WithRPCProxies() {
	b.WithRPCProxy = true

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
			Resources:             *defaultNodeResources.DeepCopy(),
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
			Resources:             *defaultNodeResources.DeepCopy(),
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
			Resources:             *execNodeResources.DeepCopy(),
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
		JobResources: execNodeJobResources.DeepCopy(),
	}
}

func (b *YtsaurusBuilder) SetupCRIJobEnvironment(node *ytv1.ExecNodesSpec) {
	node.Locations = append(node.Locations, ytv1.LocationSpec{
		LocationType: ytv1.LocationTypeImageCache,
		Path:         "/yt/node-data/image-cache",
	})
	node.JobEnvironment = &ytv1.JobEnvironmentSpec{
		CRI: &ytv1.CRIJobEnvironmentSpec{
			CRIService:             b.CRIService,
			SandboxImage:           b.SandboxImage,
			APIRetryTimeoutSeconds: ptr.To(int32(120)),
		},
	}
}

func (b *YtsaurusBuilder) WithCRIJobEnvironment() {
	for i := range b.Ytsaurus.Spec.ExecNodes {
		b.SetupCRIJobEnvironment(&b.Ytsaurus.Spec.ExecNodes[i])
	}
}

func (b *YtsaurusBuilder) WithNvidiaContainerRuntime() {
	for i := range b.Ytsaurus.Spec.ExecNodes {
		b.Ytsaurus.Spec.ExecNodes[i].GPUManager = &ytv1.GPUManagerSpec{
			GPUInfoProvider: ptr.To(ytv1.GPUInfoProviderNvidiaSMI),
		}
		b.Ytsaurus.Spec.ExecNodes[i].JobEnvironment.Runtime = &ytv1.JobRuntimeSpec{
			Nvidia: &ytv1.NvidiaRuntimeSpec{},
		}
	}
}

func (b *YtsaurusBuilder) CreateDataNodeInstanceSpec(instanceCount int32) ytv1.InstanceSpec {
	return ytv1.InstanceSpec{
		InstanceCount:         instanceCount,
		MinReadyInstanceCount: b.MinReadyInstanceCount,
		Resources:             *dataNodeResources.DeepCopy(),
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
		Resources:             *tabletNodeResources.DeepCopy(),
		Loggers:               b.CreateLoggersSpec(),
	}
}

func (b *YtsaurusBuilder) CreateOffshoreInstanceSpec(instanceCount int32) ytv1.InstanceSpec {
	return ytv1.InstanceSpec{
		InstanceCount:         instanceCount,
		MinReadyInstanceCount: b.MinReadyInstanceCount,
		Resources:             *defaultNodeResources.DeepCopy(),
		Loggers:               b.CreateLoggersSpec(),
	}
}

func (b *YtsaurusBuilder) WithStrawberryController() {
	b.Ytsaurus.Spec.StrawberryController = &ytv1.StrawberryControllerSpec{
		Image: ptr.To(b.Images.Strawberry),
	}
}

func (b *YtsaurusBuilder) CreateChyt() *ytv1.Chyt {
	b.Chyt = &ytv1.Chyt{
		ObjectMeta: metav1.ObjectMeta{
			Name:      YtsaurusName,
			Namespace: b.Namespace,
		},
		Spec: ytv1.ChytSpec{
			Ytsaurus: &corev1.LocalObjectReference{
				Name: YtsaurusName,
			},
			Image:              b.Images.Chyt,
			MakeDefault:        true,
			CreatePublicClique: ptr.To(true),
		},
	}
	return b.Chyt
}

func (b *YtsaurusBuilder) CreateSpyt() *ytv1.Spyt {
	b.Spyt = &ytv1.Spyt{
		ObjectMeta: metav1.ObjectMeta{
			Name:      YtsaurusName,
			Namespace: b.Namespace,
		},
		Spec: ytv1.SpytSpec{
			Ytsaurus: &corev1.LocalObjectReference{
				Name: YtsaurusName,
			},
		},
	}
	return b.Spyt
}

func (b *YtsaurusBuilder) CreateRemoteYtsaurus() *ytv1.RemoteYtsaurus {
	return &ytv1.RemoteYtsaurus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RemoteResourceName,
			Namespace: b.Namespace,
		},
		Spec: ytv1.RemoteYtsaurusSpec{
			MasterConnectionSpec: ytv1.MasterConnectionSpec{
				CellTag: CellTag,
				HostAddresses: []string{
					fmt.Sprintf("ms-0.masters.%s.svc.cluster.local", b.Namespace),
				},
			},
		},
	}
}

func (b *YtsaurusBuilder) CreateRemoteDataNodes() *ytv1.RemoteDataNodes {
	return &ytv1.RemoteDataNodes{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RemoteResourceName,
			Namespace: b.Namespace,
		},
		Spec: ytv1.RemoteDataNodesSpec{
			RemoteClusterSpec: &corev1.LocalObjectReference{
				Name: RemoteResourceName,
			},
			CommonSpec: ytv1.CommonSpec{
				CoreImage: b.Images.Core,
			},
			DataNodesSpec: ytv1.DataNodesSpec{
				InstanceSpec: b.CreateDataNodeInstanceSpec(3),
			},
		},
	}
}

func (b *YtsaurusBuilder) CreateRemoteExecNodes() *ytv1.RemoteExecNodes {
	return &ytv1.RemoteExecNodes{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RemoteResourceName,
			Namespace: b.Namespace,
		},
		Spec: ytv1.RemoteExecNodesSpec{
			RemoteClusterSpec: &corev1.LocalObjectReference{
				Name: RemoteResourceName,
			},
			CommonSpec: ytv1.CommonSpec{
				CoreImage: b.Images.Core,
				JobImage:  ptr.To(b.Images.Job),
			},
			ExecNodesSpec: b.CreateExecNodeSpec(),
		},
	}
}

func (b *YtsaurusBuilder) CreateRemoteTabletNodes() *ytv1.RemoteTabletNodes {
	return &ytv1.RemoteTabletNodes{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RemoteResourceName,
			Namespace: b.Namespace,
		},
		Spec: ytv1.RemoteTabletNodesSpec{
			RemoteClusterSpec: &corev1.LocalObjectReference{
				Name: RemoteResourceName,
			},
			CommonSpec: ytv1.CommonSpec{
				CoreImage: b.Images.Core,
			},
			TabletNodesSpec: ytv1.TabletNodesSpec{
				InstanceSpec: b.CreateTabletNodeSpec(3),
			},
		},
	}
}

func (b *YtsaurusBuilder) CreateOffshoreDataGateways() *ytv1.OffshoreDataGateways {
	return &ytv1.OffshoreDataGateways{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RemoteResourceName,
			Namespace: b.Namespace,
		},
		Spec: ytv1.OffshoreDataGatewaysSpec{
			RemoteClusterSpec: &corev1.LocalObjectReference{
				Name: RemoteResourceName,
			},
			CommonSpec: ytv1.CommonSpec{
				CoreImage: b.Images.Core,
			},
			OffshoreDataGatewaySpec: ytv1.OffshoreDataGatewaySpec{
				InstanceSpec: b.CreateOffshoreInstanceSpec(3),
			},
		},
	}
}
