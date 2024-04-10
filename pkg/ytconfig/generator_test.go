package ytconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"k8s.io/apimachinery/pkg/types"

	"go.ytsaurus.tech/library/go/ptr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/canonize"
)

var (
	testClusterDomain  = "fake.zone"
	testNamespace      = "fake"
	testYtsaurusName   = "test"
	testNamespacedName = types.NamespacedName{
		Namespace: testNamespace,
		Name:      testYtsaurusName,
	}
	testLogRotationPeriod   int64 = 900000
	testTotalLogSize              = 10 * int64(1<<30)
	testMasterExternalHosts       = []string{
		"host1.external.address",
		"host2.external.address",
		"host3.external.address",
	}
	testMasterCachesExternalHosts = []string{
		"host1.external.address",
		"host2.external.address",
		"host3.external.address",
	}
	testBasicInstanceSpec = v1.InstanceSpec{InstanceCount: 3, MonitoringPort: ptr.Int32(12345)}
	testStorageClassname  = "yc-network-hdd"
	testResourceReqs      = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewQuantity(20, resource.BinarySI),
			corev1.ResourceMemory: *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI),
		},
	}
	testLocationChunkStore = v1.LocationSpec{
		LocationType: "ChunkStore",
		Path:         "/yt/hdd1/chunk-store",
		Medium:       "nvme",
	}
	testLocationChunkCache = v1.LocationSpec{
		LocationType: "ChunkCache",
		Path:         "/yt/hdd1/chunk-cache",
	}
	testLocationSlots = v1.LocationSpec{
		LocationType: "Slots",
		Path:         "/yt/hdd2/slots",
	}
	testLocationImageCache = v1.LocationSpec{
		LocationType: "ImageCache",
		Path:         "/yt/hdd1/images",
	}
	testVolumeMounts = []corev1.VolumeMount{
		{
			Name:      "hdd1",
			MountPath: "/yt/hdd1",
		},
	}
	testVolumeClaimTemplates = []v1.EmbeddedPersistentVolumeClaim{
		{
			EmbeddedObjectMetadata: v1.EmbeddedObjectMetadata{
				Name: "hdd1",
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageClassName: &testStorageClassname,
			},
		},
	}
	testClusterNodeSpec = v1.ClusterNodesSpec{
		Tags: []string{"rack:xn-a"},
		Rack: "fake",
	}
)

func TestGetChytInitClusterConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetChytInitClusterConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetControllerAgentsConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetControllerAgentConfig(ytsaurus.Spec.ControllerAgents)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetDataNodeConfig(t *testing.T) {
	g := NewLocalNodeGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetDataNodeConfig(getDataNodeSpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetDataNodeWithoutYtsaurusConfig(t *testing.T) {
	g := NewRemoteNodeGenerator(
		testNamespacedName,
		testClusterDomain,
		getCommonSpec(),
		getMasterConnectionSpecWithFixedMasterHosts(),
		getMasterCachesSpecWithFixedHosts(),
	)
	cfg, err := g.GetDataNodeConfig(getDataNodeSpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetDiscoveryConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetDiscoveryConfig(&ytsaurus.Spec.Discovery)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetExecNodeConfig(t *testing.T) {
	g := NewLocalNodeGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetExecNodeConfig(getExecNodeSpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetExecNodeConfigWithCri(t *testing.T) {
	g := NewLocalNodeGenerator(getYtsaurusWithEverything(), testClusterDomain)
	spec := withCri(getExecNodeSpec())
	cfg, err := g.GetExecNodeConfig(spec)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetContainerdConfig(t *testing.T) {
	g := NewLocalNodeGenerator(getYtsaurusWithEverything(), testClusterDomain)
	spec := withCri(getExecNodeSpec())
	cfg, err := g.GetContainerdConfig(&spec)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetExecNodeWithoutYtsaurusConfig(t *testing.T) {
	g := NewRemoteNodeGenerator(
		testNamespacedName,
		testClusterDomain,
		getCommonSpec(),
		getMasterConnectionSpecWithFixedMasterHosts(),
		getMasterCachesSpecWithFixedHosts(),
	)
	cfg, err := g.GetExecNodeConfig(getExecNodeSpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetHTTPProxyConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetHTTPProxyConfig(getHTTPProxySpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetMasterConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetMasterConfig(&ytsaurus.Spec.PrimaryMasters)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetMasterWithFixedHostsConfig(t *testing.T) {
	ytsaurus := withFixedMasterHosts(getYtsaurus())
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetMasterConfig(&ytsaurus.Spec.PrimaryMasters)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetNativeClientConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetNativeClientConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetQueryTrackerConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetQueryTrackerConfig(ytsaurus.Spec.QueryTrackers)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetQueueAgentConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetQueueAgentConfig(ytsaurus.Spec.QueueAgents)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetRPCProxyConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetRPCProxyConfig(getRPCProxySpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetRPCProxyWithoutOauthConfig(t *testing.T) {
	g := NewGenerator(getYtsaurus(), testClusterDomain)
	cfg, err := g.GetRPCProxyConfig(getRPCProxySpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetSchedulerConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetSchedulerConfig(ytsaurus.Spec.Schedulers)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetSchedulerWithFixedMasterHostsConfig(t *testing.T) {
	ytsaurus := withFixedMasterHosts(withScheduler(getYtsaurus()))
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetSchedulerConfig(ytsaurus.Spec.Schedulers)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetStrawberryControllerConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetStrawberryControllerConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetTabletNodeConfig(t *testing.T) {
	g := NewLocalNodeGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetTabletNodeConfig(getTabletNodeSpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetTabletNodeWithoutYtsaurusConfig(t *testing.T) {
	g := NewRemoteNodeGenerator(
		testNamespacedName,
		testClusterDomain,
		getCommonSpec(),
		getMasterConnectionSpecWithFixedMasterHosts(),
		getMasterCachesSpecWithFixedHosts(),
	)
	cfg, err := g.GetTabletNodeConfig(getTabletNodeSpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetTCPProxyConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetTCPProxyConfig(getTCPProxySpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetUIClustersConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetUIClustersConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetUICustomConfig(t *testing.T) {
	g := NewGenerator(withUICustom(getYtsaurus()), testClusterDomain)
	cfg, err := g.GetUICustomConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetYQLAgentConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetYQLAgentConfig(ytsaurus.Spec.YQLAgents)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetMasterCachesWithFixedHostsConfig(t *testing.T) {
	ytsaurus := withFixedMasterCachesHosts(getYtsaurusWithEverything())
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetMasterCachesConfig(ytsaurus.Spec.MasterCaches)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetMasterCachesConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetMasterCachesConfig(ytsaurus.Spec.MasterCaches)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func getYtsaurus() *v1.Ytsaurus {
	return &v1.Ytsaurus{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testYtsaurusName,
		},
		Spec: v1.YtsaurusSpec{
			CommonSpec: getCommonSpec(),

			PrimaryMasters: v1.MastersSpec{
				MasterConnectionSpec:   getMasterConnectionSpec(),
				MaxSnapshotCountToKeep: ptr.Int(1543),
				InstanceSpec: v1.InstanceSpec{
					InstanceCount:  1,
					MonitoringPort: ptr.Int32(consts.MasterMonitoringPort),

					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "master-data",
							MountPath: "/yt/master-data",
						},
					},

					Locations: []v1.LocationSpec{
						{
							LocationType: v1.LocationTypeMasterChangelogs,
							Path:         "/yt/master-data/master-changelogs",
						},
						{
							LocationType: v1.LocationTypeMasterSnapshots,
							Path:         "/yt/master-data/master-snapshots",
						},
					},

					VolumeClaimTemplates: []v1.EmbeddedPersistentVolumeClaim{
						{
							EmbeddedObjectMetadata: v1.EmbeddedObjectMetadata{
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

					Loggers: []v1.TextLoggerSpec{
						{
							BaseLoggerSpec: v1.BaseLoggerSpec{
								Name:        "info",
								MinLogLevel: v1.LogLevelInfo,
								Compression: v1.LogCompressionNone,
								Format:      v1.LogFormatPlainText,
							},
							WriterType: v1.LogWriterTypeFile,
						},
						{
							BaseLoggerSpec: v1.BaseLoggerSpec{
								Name:        "error",
								MinLogLevel: v1.LogLevelError,
								Compression: v1.LogCompressionNone,
								Format:      v1.LogFormatPlainText,
							},
							WriterType: v1.LogWriterTypeFile,
						},
						{
							BaseLoggerSpec: v1.BaseLoggerSpec{
								Name:        "debug",
								MinLogLevel: v1.LogLevelDebug,
								Compression: v1.LogCompressionZstd,
								Format:      v1.LogFormatPlainText,

								RotationPolicy: &v1.LogRotationPolicy{
									RotationPeriodMilliseconds: &testLogRotationPeriod,
									MaxTotalSizeToKeep:         &testTotalLogSize,
								},
							},
							WriterType: v1.LogWriterTypeFile,
							CategoriesFilter: &v1.CategoriesFilter{
								Type:   v1.CategoriesFilterTypeExclude,
								Values: []string{"Bus"},
							},
						},
					},
				},
			},
		},
	}
}

func getYtsaurusWithEverything() *v1.Ytsaurus {
	ytsaurus := getYtsaurus()
	ytsaurus = withControllerAgents(ytsaurus)
	ytsaurus = withOauthSpec(ytsaurus)
	ytsaurus = withResolverConfigured(ytsaurus)
	ytsaurus = withDiscovery(ytsaurus)
	ytsaurus = withQueryTracker(ytsaurus)
	ytsaurus = withQueueAgent(ytsaurus)
	ytsaurus = withStrawberry(ytsaurus)
	ytsaurus = withScheduler(ytsaurus)
	ytsaurus = withTCPProxies(ytsaurus)
	ytsaurus = withUI(ytsaurus)
	ytsaurus = withYQLAgent(ytsaurus)
	ytsaurus = withMasterCaches(ytsaurus)
	return ytsaurus
}

func withControllerAgents(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.ControllerAgents = &v1.ControllerAgentsSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.UsePorto = true
	ytsaurus.Spec.ControllerAgents.InstanceSpec.MonitoringPort = ptr.Int32(consts.ControllerAgentMonitoringPort)
	return ytsaurus
}

func withOauthSpec(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.OauthService = &v1.OauthServiceSpec{
		Host:   "oauth-host",
		Port:   433,
		Secure: true,
		UserInfo: v1.OauthUserInfoHandlerSpec{
			Endpoint:   "user-info-endpoint",
			LoginField: "login",
		},
	}
	return ytsaurus
}

func withResolverConfigured(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.UseIPv4 = true
	ytsaurus.Spec.UseIPv6 = false
	return ytsaurus
}

func withDiscovery(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.Discovery = v1.DiscoverySpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.Discovery.InstanceSpec.MonitoringPort = ptr.Int32(consts.DiscoveryMonitoringPort)
	return ytsaurus
}

func withQueryTracker(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.QueryTrackers = &v1.QueryTrackerSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.QueryTrackers.InstanceSpec.MonitoringPort = ptr.Int32(consts.QueryTrackerMonitoringPort)
	return ytsaurus
}

func withQueueAgent(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.QueueAgents = &v1.QueueAgentSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.QueueAgents.InstanceSpec.MonitoringPort = ptr.Int32(consts.QueueAgentMonitoringPort)
	return ytsaurus
}

func withStrawberry(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	image := "dummy-strawberry-image"
	ytsaurus.Spec.StrawberryController = &v1.StrawberryControllerSpec{
		Resources: testResourceReqs,
		Image:     &image,
	}
	return ytsaurus
}

func withScheduler(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.Schedulers = &v1.SchedulersSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.Schedulers.InstanceSpec.MonitoringPort = ptr.Int32(consts.SchedulerMonitoringPort)
	return ytsaurus
}

func withFixedMasterHosts(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.PrimaryMasters.HostAddresses = testMasterExternalHosts
	return ytsaurus
}

func withTCPProxies(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.TCPProxies = []v1.TCPProxiesSpec{
		{
			InstanceSpec: testBasicInstanceSpec,
			MinPort:      10000,
			PortCount:    20000,
		},
	}
	ytsaurus.Spec.TCPProxies[0].InstanceSpec.MonitoringPort = ptr.Int32(consts.TCPProxyMonitoringPort)
	return ytsaurus
}

func withUI(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	image := "dummy-ui-image"
	odinUrl := "http://odin-webservice.odin.svc.cluster.local"

	ytsaurus.Spec.UI = &v1.UISpec{
		Image:       &image,
		ServiceType: corev1.ServiceTypeNodePort,
		OdinBaseUrl: &odinUrl,
	}
	return ytsaurus
}

func withUICustom(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	odinUrl := "http://odin-webservice.odin.svc.cluster.local"
	ytsaurus.Spec.UI = &v1.UISpec{
		OdinBaseUrl: &odinUrl,
	}
	return ytsaurus
}

func withMasterCaches(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.MasterCaches = &v1.MasterCachesSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.MasterCaches.InstanceSpec.MonitoringPort = ptr.Int32(consts.MasterCachesMonitoringPort)
	return ytsaurus
}

func withYQLAgent(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.YQLAgents = &v1.YQLAgentSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.YQLAgents.InstanceSpec.MonitoringPort = ptr.Int32(consts.YQLAgentMonitoringPort)
	return ytsaurus
}

func withFixedMasterCachesHosts(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.MasterCaches.MasterCachesConnectionSpec.HostAddresses = testMasterCachesExternalHosts
	ytsaurus.Spec.MasterCaches.InstanceSpec.MonitoringPort = ptr.Int32(consts.MasterCachesMonitoringPort)
	return ytsaurus
}

func getDataNodeSpec() v1.DataNodesSpec {
	return v1.DataNodesSpec{
		InstanceSpec: v1.InstanceSpec{
			InstanceCount:        20,
			MonitoringPort:       ptr.Int32(consts.DataNodeMonitoringPort),
			Resources:            testResourceReqs,
			Locations:            []v1.LocationSpec{testLocationChunkStore},
			VolumeMounts:         testVolumeMounts,
			VolumeClaimTemplates: testVolumeClaimTemplates,
		},
		ClusterNodesSpec: testClusterNodeSpec,
		Name:             "dn-a",
	}
}

func getExecNodeSpec() v1.ExecNodesSpec {
	rotationPolicyMS := int64(900000)
	rotationPolicyMaxTotalSize := int64(3145728)
	return v1.ExecNodesSpec{
		InstanceSpec: v1.InstanceSpec{
			InstanceCount:  50,
			MonitoringPort: ptr.Int32(consts.ExecNodeMonitoringPort),
			Resources:      testResourceReqs,
			Locations: []v1.LocationSpec{
				testLocationChunkCache,
				testLocationSlots,
			},
			VolumeMounts:         testVolumeMounts,
			VolumeClaimTemplates: testVolumeClaimTemplates,
		},
		ClusterNodesSpec: testClusterNodeSpec,
		JobProxyLoggers: []v1.TextLoggerSpec{
			{
				BaseLoggerSpec: v1.BaseLoggerSpec{
					Name:        "debug",
					Format:      v1.LogFormatPlainText,
					MinLogLevel: v1.LogLevelDebug,
					Compression: v1.LogCompressionZstd,
					RotationPolicy: &v1.LogRotationPolicy{
						RotationPeriodMilliseconds: &rotationPolicyMS,
						MaxTotalSizeToKeep:         &rotationPolicyMaxTotalSize,
					},
				},
				WriterType: v1.LogWriterTypeFile,
				CategoriesFilter: &v1.CategoriesFilter{
					Type:   v1.CategoriesFilterTypeExclude,
					Values: []string{"Bus", "Concurrency"},
				},
			},
		},
		Name: "end-a",
	}
}

func withCri(spec v1.ExecNodesSpec) v1.ExecNodesSpec {
	spec.Locations = append(spec.Locations, testLocationImageCache)
	spec.JobResources = &testResourceReqs
	spec.JobEnvironment = &v1.JobEnvironmentSpec{
		UserSlots: ptr.Int(42),
		CRI: &v1.CRIJobEnvironmentSpec{
			SandboxImage:           ptr.String("registry.k8s.io/pause:3.8"),
			APIRetryTimeoutSeconds: ptr.Int32(120),
			CRINamespace:           ptr.String("yt"),
			BaseCgroup:             ptr.String("/yt"),
		},
		UseArtifactBinds: ptr.Bool(true),
		DoNotSetUserId:   ptr.Bool(true),
	}
	return spec
}

func getTabletNodeSpec() v1.TabletNodesSpec {
	return v1.TabletNodesSpec{
		InstanceSpec: v1.InstanceSpec{
			InstanceCount:  100,
			MonitoringPort: ptr.Int32(consts.TabletNodeMonitoringPort),
			Resources:      testResourceReqs,
			Locations: []v1.LocationSpec{
				testLocationChunkCache,
				testLocationSlots,
			},
			VolumeMounts:         testVolumeMounts,
			VolumeClaimTemplates: testVolumeClaimTemplates,
		},
		ClusterNodesSpec: testClusterNodeSpec,
	}
}

func getHTTPProxySpec() v1.HTTPProxiesSpec {
	httpPort := int32(10000)
	httpsPort := int32(10001)
	return v1.HTTPProxiesSpec{
		InstanceSpec: v1.InstanceSpec{
			InstanceCount:  3,
			MonitoringPort: ptr.Int32(consts.HTTPProxyMonitoringPort),
		},
		ServiceType: corev1.ServiceTypeNodePort,
		Role:        "control",
		Transport: v1.HTTPTransportSpec{
			HTTPSSecret: &corev1.LocalObjectReference{Name: "yt-test-infra-wildcard"},
		},
		HttpNodePort:  &httpPort,
		HttpsNodePort: &httpsPort,
	}
}

func getRPCProxySpec() v1.RPCProxiesSpec {
	return v1.RPCProxiesSpec{
		InstanceSpec: v1.InstanceSpec{
			InstanceCount:  3,
			MonitoringPort: ptr.Int32(consts.RPCProxyMonitoringPort),
		},
		Role: "default",
	}
}

func getTCPProxySpec() v1.TCPProxiesSpec {
	return v1.TCPProxiesSpec{
		InstanceSpec: v1.InstanceSpec{
			InstanceCount:  3,
			MonitoringPort: ptr.Int32(consts.TCPProxyMonitoringPort),
		},
		Role: "default",
	}
}

func getCommonSpec() v1.CommonSpec {
	return v1.CommonSpec{
		UseIPv6: true,
	}
}

func getMasterConnectionSpec() v1.MasterConnectionSpec {
	return v1.MasterConnectionSpec{
		CellTag: 0,
	}
}

func getMasterConnectionSpecWithFixedMasterHosts() v1.MasterConnectionSpec {
	spec := getMasterConnectionSpec()
	spec.HostAddresses = testMasterExternalHosts
	spec.CellTag = 1000
	return spec
}

func getMasterCachesSpec() v1.MasterCachesSpec {
	return v1.MasterCachesSpec{
		InstanceSpec: v1.InstanceSpec{
			InstanceCount:  3,
			MonitoringPort: ptr.Int32(consts.MasterCachesMonitoringPort),
		},
		HostAddressLabel: "",
	}
}

func getMasterCachesSpecWithFixedHosts() *v1.MasterCachesSpec {
	spec := getMasterCachesSpec()
	spec.MasterCachesConnectionSpec.HostAddresses = testMasterCachesExternalHosts
	return &spec
}
