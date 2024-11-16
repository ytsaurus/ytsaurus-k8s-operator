package ytconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"

	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/canonize"
)

var (
	testClusterDomain = "fake.zone"
	testNamespace     = "fake"
	testYtsaurusName  = "test"
	testObjectMeta    = metav1.ObjectMeta{
		Namespace: testNamespace,
		Name:      testYtsaurusName,
	}
	testLogRotationPeriod   int64 = 900000
	testTotalLogSize              = resource.MustParse("10Gi")
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
	testBasicInstanceSpec = ytv1.InstanceSpec{InstanceCount: 3, MonitoringPort: ptr.To(int32(12345))}
	testStorageClassname  = "yc-network-hdd"
	testResourceReqs      = corev1.ResourceRequirements{
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
	testMaxTrashMilliseconds       int64 = 60000
	testLocationChunkStoreMaxTrash       = ytv1.LocationSpec{
		LocationType:         "ChunkStore",
		Path:                 "/yt/hdd1/chunk-store",
		Medium:               "nvme",
		MaxTrashMilliseconds: &testMaxTrashMilliseconds,
	}
	testLocationChunkStoreQuota        = resource.MustParse("1Ti")
	testLocationChunkStoreLowWatermark = resource.MustParse("50Gi")
	testLocationChunkStoreWatermark    = ytv1.LocationSpec{
		LocationType: "ChunkStore",
		Path:         "/yt/hdd1/chunk-store",
		Medium:       "nvme",
		Quota:        &testLocationChunkStoreQuota,
		LowWatermark: &testLocationChunkStoreLowWatermark,
	}
	testLocationChunkCache = ytv1.LocationSpec{
		LocationType: "ChunkCache",
		Path:         "/yt/hdd1/chunk-cache",
	}
	testLocationSlotsQuota = resource.MustParse("5Gi")
	testLocationSlots      = ytv1.LocationSpec{
		LocationType: "Slots",
		Path:         "/yt/hdd2/slots",
		Quota:        &testLocationSlotsQuota,
	}
	testLocationImageCacheQuota = resource.MustParse("4Gi")
	testLocationImageCache      = ytv1.LocationSpec{
		LocationType: "ImageCache",
		Path:         "/yt/hdd1/images",
		Quota:        &testLocationImageCacheQuota,
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
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("10Gi"),
					},
				},
			},
		},
	}
	testClusterNodeSpec = ytv1.ClusterNodesSpec{
		Tags: []string{"rack:xn-a"},
		Rack: "fake",
	}
)

func TestGetStrawberryInitClusterConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetStrawberryInitClusterConfig()
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
	cases := map[string]struct {
		Location ytv1.LocationSpec
	}{
		"without-trash-ttl": {
			Location: testLocationChunkStore,
		},
		"with-trash-ttl": {
			Location: testLocationChunkStoreMaxTrash,
		},
		"with-watermark": {
			Location: testLocationChunkStoreWatermark,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			ytsaurus := getYtsaurusWithEverything()
			g := NewLocalNodeGenerator(ytsaurus, ytsaurus.Name, testClusterDomain)
			cfg, err := g.GetDataNodeConfig(getDataNodeSpec(test.Location))
			require.NoError(t, err)
			canonize.Assert(t, cfg)
		})
	}
}

func TestGetDataNodeWithoutYtsaurusConfig(t *testing.T) {
	g := NewRemoteNodeGenerator(
		getRemoteYtsaurus(),
		testYtsaurusName,
		testClusterDomain,
		getCommonSpec(),
	)
	cfg, err := g.GetDataNodeConfig(getDataNodeSpec(testLocationChunkStore))
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
	cases := map[string]struct {
		JobResources *corev1.ResourceRequirements
	}{
		"without-job-resources": {
			JobResources: nil,
		},
		"with-job-resources": {
			JobResources: &testJobResourceReqs,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			ytsaurus := getYtsaurusWithEverything()
			g := NewLocalNodeGenerator(ytsaurus, ytsaurus.Name, testClusterDomain)
			cfg, err := g.GetExecNodeConfig(getExecNodeSpec(test.JobResources))
			require.NoError(t, err)
			canonize.Assert(t, cfg)
		})
	}
}

func TestGetExecNodeConfigWithCri(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	g := NewLocalNodeGenerator(ytsaurus, ytsaurus.Name, testClusterDomain)

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
			spec := withCri(getExecNodeSpec(nil), test.JobResources, test.Isolated)
			cfg, err := g.GetExecNodeConfig(spec)
			require.NoError(t, err)
			canonize.Assert(t, cfg)
		})
	}
}

func TestGetContainerdConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	g := NewLocalNodeGenerator(ytsaurus, ytsaurus.Name, testClusterDomain)

	spec := withCri(getExecNodeSpec(nil), nil, false)
	cfg, err := g.GetContainerdConfig(&spec)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetExecNodeWithoutYtsaurusConfig(t *testing.T) {
	g := NewRemoteNodeGenerator(
		getRemoteYtsaurus(),
		testYtsaurusName,
		testClusterDomain,
		getCommonSpec(),
	)
	cfg, err := g.GetExecNodeConfig(getExecNodeSpec(nil))
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

func TestGetStrawberryControllerConfigWithExtendedHTTPMapping(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	externalProxy := "some.domain"
	g.ytsaurus.Spec.StrawberryController.ExternalProxy = &externalProxy
	cfg, err := g.GetStrawberryControllerConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetStrawberryControllerConfigWithCustomFamilies(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	externalProxy := "some.domain"
	g.ytsaurus.Spec.StrawberryController.ExternalProxy = &externalProxy
	g.ytsaurus.Spec.StrawberryController.ControllerFamilies = append(
		g.ytsaurus.Spec.StrawberryController.ControllerFamilies,
		"superservice1", "superservice2", "superservice3",
	)
	defaultRouteFamily := "superservice2"
	g.ytsaurus.Spec.StrawberryController.DefaultRouteFamily = &defaultRouteFamily
	cfg, err := g.GetStrawberryControllerConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetTabletNodeConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	g := NewLocalNodeGenerator(ytsaurus, ytsaurus.Name, testClusterDomain)
	cfg, err := g.GetTabletNodeConfig(getTabletNodeSpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetTabletNodeWithoutYtsaurusConfig(t *testing.T) {
	g := NewRemoteNodeGenerator(
		getRemoteYtsaurus(),
		testYtsaurusName,
		testClusterDomain,
		getCommonSpec(),
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

func TestGetUIClustersConfigWithSettings(t *testing.T) {
	g := NewGenerator(withUICustom(getYtsaurus()), testClusterDomain)
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

func TestGetUICustomConfigWithSettings(t *testing.T) {
	g := NewGenerator(withUICustomSettings(getYtsaurus()), testClusterDomain)
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

func TestResolverOptionsKeepSocketAndForceTCP(t *testing.T) {
	ytsaurus := getYtsaurusWithEverything()
	ytsaurus.Spec.CommonSpec.ForceTCP = ptr.To(true)
	ytsaurus.Spec.CommonSpec.KeepSocket = ptr.To(true)
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetMasterConfig(&ytsaurus.Spec.PrimaryMasters)
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

func getYtsaurus() *ytv1.Ytsaurus {
	return &ytv1.Ytsaurus{
		ObjectMeta: testObjectMeta,
		Spec: ytv1.YtsaurusSpec{
			CommonSpec: *getCommonSpec(),

			PrimaryMasters: ytv1.MastersSpec{
				MasterConnectionSpec:   getMasterConnectionSpec(),
				MaxSnapshotCountToKeep: ptr.To(1543),
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount:  1,
					MonitoringPort: ptr.To(int32(consts.MasterMonitoringPort)),

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

func getRemoteYtsaurus() *ytv1.RemoteYtsaurus {
	return &ytv1.RemoteYtsaurus{
		ObjectMeta: testObjectMeta,
		Spec: ytv1.RemoteYtsaurusSpec{
			MasterConnectionSpec: getMasterConnectionSpecWithFixedMasterHosts(),
			MasterCachesSpec:     getMasterCachesSpecWithFixedHosts(),
		},
	}
}

func getYtsaurusWithEverything() *ytv1.Ytsaurus {
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

func withControllerAgents(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.ControllerAgents = &ytv1.ControllerAgentsSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.UsePorto = true
	ytsaurus.Spec.ControllerAgents.InstanceSpec.MonitoringPort = ptr.To(int32(consts.ControllerAgentMonitoringPort))
	return ytsaurus
}

func withOauthSpec(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.OauthService = &ytv1.OauthServiceSpec{
		Host:   "oauth-host",
		Port:   433,
		Secure: true,
		UserInfo: ytv1.OauthUserInfoHandlerSpec{
			Endpoint:   "user-info-endpoint",
			LoginField: "login",
		},
	}
	return ytsaurus
}

func withResolverConfigured(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.UseIPv4 = true
	ytsaurus.Spec.UseIPv6 = false
	return ytsaurus
}

func withDiscovery(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.Discovery = ytv1.DiscoverySpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.Discovery.InstanceSpec.MonitoringPort = ptr.To(int32(consts.DiscoveryMonitoringPort))
	return ytsaurus
}

func withQueryTracker(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.QueryTrackers.InstanceSpec.MonitoringPort = ptr.To(int32(consts.QueryTrackerMonitoringPort))
	return ytsaurus
}

func withQueueAgent(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.QueueAgents = &ytv1.QueueAgentSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.QueueAgents.InstanceSpec.MonitoringPort = ptr.To(int32(consts.QueueAgentMonitoringPort))
	return ytsaurus
}

func withStrawberry(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	image := "dummy-strawberry-image"
	ytsaurus.Spec.StrawberryController = &ytv1.StrawberryControllerSpec{
		Resources: testResourceReqs,
		Image:     &image,
	}
	return ytsaurus
}

func withScheduler(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.Schedulers = &ytv1.SchedulersSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.Schedulers.InstanceSpec.MonitoringPort = ptr.To(int32(consts.SchedulerMonitoringPort))
	return ytsaurus
}

func withFixedMasterHosts(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.PrimaryMasters.HostAddresses = testMasterExternalHosts
	return ytsaurus
}

func withTCPProxies(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.TCPProxies = []ytv1.TCPProxiesSpec{
		{
			InstanceSpec: testBasicInstanceSpec,
			MinPort:      10000,
			PortCount:    20000,
		},
	}
	ytsaurus.Spec.TCPProxies[0].InstanceSpec.MonitoringPort = ptr.To(int32(consts.TCPProxyMonitoringPort))
	return ytsaurus
}

func withUI(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	image := "dummy-ui-image"
	odinUrl := "http://odin-webservice.odin.svc.cluster.local"

	ytsaurus.Spec.UI = &ytv1.UISpec{
		Image:       &image,
		ServiceType: corev1.ServiceTypeNodePort,
		OdinBaseUrl: &odinUrl,
	}
	return ytsaurus
}

func withUICustom(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	odinUrl := "http://odin-webservice.odin.svc.cluster.local"
	externalProxy := "https://my-external-proxy.example.com"
	ytsaurus.Spec.UI = &ytv1.UISpec{
		ExternalProxy: &externalProxy,
		OdinBaseUrl:   &odinUrl,
		Secure:        true,
	}
	return ytsaurus
}

func withUICustomSettings(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	odinUrl := "http://odin-webservice.odin.svc.cluster.local"
	ytsaurus.Spec.UI = &ytv1.UISpec{
		OdinBaseUrl:    &odinUrl,
		DirectDownload: nil,
	}
	return ytsaurus
}

func withMasterCaches(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.MasterCaches = &ytv1.MasterCachesSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.MasterCaches.InstanceSpec.MonitoringPort = ptr.To(int32(consts.MasterCachesMonitoringPort))
	return ytsaurus
}

func withYQLAgent(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.YQLAgents = &ytv1.YQLAgentSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.YQLAgents.InstanceSpec.MonitoringPort = ptr.To(int32(consts.YQLAgentMonitoringPort))
	return ytsaurus
}

func withFixedMasterCachesHosts(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.MasterCaches.MasterCachesConnectionSpec.HostAddresses = testMasterCachesExternalHosts
	ytsaurus.Spec.MasterCaches.InstanceSpec.MonitoringPort = ptr.To(int32(consts.MasterCachesMonitoringPort))
	return ytsaurus
}

func getDataNodeSpec(locations ...ytv1.LocationSpec) ytv1.DataNodesSpec {
	return ytv1.DataNodesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:        20,
			MonitoringPort:       ptr.To(int32(consts.DataNodeMonitoringPort)),
			Resources:            testResourceReqs,
			Locations:            locations,
			VolumeMounts:         testVolumeMounts,
			VolumeClaimTemplates: testVolumeClaimTemplates,
		},
		ClusterNodesSpec: testClusterNodeSpec,
		Name:             "dn-a",
	}
}

func getExecNodeSpec(jobResources *corev1.ResourceRequirements) ytv1.ExecNodesSpec {
	rotationPolicyMS := int64(900000)
	rotationPolicyMaxTotalSize := resource.MustParse("3145728")
	return ytv1.ExecNodesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:  50,
			MonitoringPort: ptr.To(int32(consts.ExecNodeMonitoringPort)),
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
		UserSlots: ptr.To(int(42)),
		CRI: &ytv1.CRIJobEnvironmentSpec{
			SandboxImage:           ptr.To("registry.k8s.io/pause:3.8"),
			APIRetryTimeoutSeconds: ptr.To(int32(120)),
			CRINamespace:           ptr.To("yt"),
			BaseCgroup:             ptr.To("/yt"),
		},
		UseArtifactBinds: ptr.To(true),
		DoNotSetUserId:   ptr.To(true),
		Isolated:         ptr.To(isolated),
	}
	return spec
}

func getTabletNodeSpec() ytv1.TabletNodesSpec {
	return ytv1.TabletNodesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:  100,
			MonitoringPort: ptr.To(int32(consts.TabletNodeMonitoringPort)),
			Resources:      testResourceReqs,
			Locations: []ytv1.LocationSpec{
				testLocationChunkCache,
				testLocationSlots,
			},
			VolumeMounts:         testVolumeMounts,
			VolumeClaimTemplates: testVolumeClaimTemplates,
		},
		ClusterNodesSpec: testClusterNodeSpec,
	}
}

func getHTTPProxySpec() ytv1.HTTPProxiesSpec {
	httpPort := int32(10000)
	httpsPort := int32(10001)
	return ytv1.HTTPProxiesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:  3,
			MonitoringPort: ptr.To(int32(consts.HTTPProxyMonitoringPort)),
		},
		ServiceType: corev1.ServiceTypeNodePort,
		Role:        "control",
		Transport: ytv1.HTTPTransportSpec{
			HTTPSSecret: &corev1.LocalObjectReference{Name: "yt-test-infra-wildcard"},
		},
		HttpNodePort:  &httpPort,
		HttpsNodePort: &httpsPort,
	}
}

func getRPCProxySpec() ytv1.RPCProxiesSpec {
	return ytv1.RPCProxiesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:  3,
			MonitoringPort: ptr.To(int32(consts.RPCProxyMonitoringPort)),
		},
		Role: "default",
	}
}

func getTCPProxySpec() ytv1.TCPProxiesSpec {
	return ytv1.TCPProxiesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:  3,
			MonitoringPort: ptr.To(int32(consts.TCPProxyMonitoringPort)),
		},
		Role: "default",
	}
}

func getCommonSpec() *ytv1.CommonSpec {
	return &ytv1.CommonSpec{
		UseIPv6: true,
	}
}

func getMasterConnectionSpec() ytv1.MasterConnectionSpec {
	return ytv1.MasterConnectionSpec{
		CellTag: 0,
	}
}

func getMasterConnectionSpecWithFixedMasterHosts() ytv1.MasterConnectionSpec {
	spec := getMasterConnectionSpec()
	spec.HostAddresses = testMasterExternalHosts
	spec.CellTag = 1000
	return spec
}

func getMasterCachesSpec() ytv1.MasterCachesSpec {
	return ytv1.MasterCachesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:  3,
			MonitoringPort: ptr.To(int32(consts.MasterCachesMonitoringPort)),
		},
		HostAddressLabel: "",
	}
}

func getMasterCachesSpecWithFixedHosts() ytv1.MasterCachesSpec {
	spec := getMasterCachesSpec()
	spec.MasterCachesConnectionSpec.HostAddresses = testMasterCachesExternalHosts
	return spec
}
