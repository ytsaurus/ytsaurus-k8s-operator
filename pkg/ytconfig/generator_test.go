package ytconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.ytsaurus.tech/library/go/ptr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/canonize"
)

var (
	testClusterDomain             = "fake.zone"
	testLogRotationPeriod   int64 = 900000
	testTotalLogSize              = 10 * int64(1<<30)
	testMasterExternalHosts       = []string{
		"host1.external.address",
		"host2.external.address",
		"host3.external.address",
	}
	testBasicInstanceSpec = v1.InstanceSpec{InstanceCount: 3}
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
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetControllerAgentConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetDataNodeConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetDataNodeConfig(getDataNodeSpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetDiscoveryConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetDiscoveryConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetExecNodeConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
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
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetMasterConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetMasterWithFixedHostsConfig(t *testing.T) {
	g := NewGenerator(withFixedMasterHosts(getYtsaurus()), testClusterDomain)
	cfg, err := g.GetMasterConfig()
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
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetQueryTrackerConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetQueueAgentConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetQueueAgentConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetRPCProxyConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetRPCProxyConfig(getRPCProxySpec())
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetSchedulerConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetSchedulerConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetSchedulerWithFixedMasterHostsConfig(t *testing.T) {
	g := NewGenerator(withFixedMasterHosts(withScheduler(getYtsaurus())), testClusterDomain)
	cfg, err := g.GetSchedulerConfig()
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
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
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
	g := NewGenerator(getYtsaurusWithEverything(), testClusterDomain)
	cfg, err := g.GetYQLAgentConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func getYtsaurus() *v1.Ytsaurus {
	return &v1.Ytsaurus{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake",
			Name:      "test",
		},
		Spec: v1.YtsaurusSpec{
			UseIPv6: true,

			PrimaryMasters: v1.MastersSpec{
				CellTag:                0,
				MaxSnapshotCountToKeep: ptr.Int(1543),
				InstanceSpec: v1.InstanceSpec{
					InstanceCount: 1,

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
	ytsaurus = withTCPPRoxies(ytsaurus)
	ytsaurus = withUI(ytsaurus)
	ytsaurus = withYQLAgent(ytsaurus)
	return ytsaurus
}

func withControllerAgents(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.ControllerAgents = &v1.ControllerAgentsSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.UsePorto = true
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
	return ytsaurus
}

func withQueryTracker(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.QueryTrackers = &v1.QueryTrackerSpec{InstanceSpec: testBasicInstanceSpec}
	return ytsaurus
}

func withQueueAgent(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.QueueAgents = &v1.QueueAgentSpec{InstanceSpec: testBasicInstanceSpec}
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
	return ytsaurus
}

func withFixedMasterHosts(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.PrimaryMasters.HostAddresses = testMasterExternalHosts
	return ytsaurus
}

func withTCPPRoxies(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.TCPProxies = []v1.TCPProxiesSpec{
		{
			InstanceSpec: testBasicInstanceSpec,
			MinPort:      10000,
			PortCount:    20000,
		},
	}
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

func withYQLAgent(ytsaurus *v1.Ytsaurus) *v1.Ytsaurus {
	ytsaurus.Spec.YQLAgents = &v1.YQLAgentSpec{InstanceSpec: testBasicInstanceSpec}
	return ytsaurus
}

func getDataNodeSpec() v1.DataNodesSpec {
	return v1.DataNodesSpec{
		InstanceSpec: v1.InstanceSpec{
			InstanceCount:        20,
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
			InstanceCount: 50,
			Resources:     testResourceReqs,
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

func getTabletNodeSpec() v1.TabletNodesSpec {
	return v1.TabletNodesSpec{
		InstanceSpec: v1.InstanceSpec{
			InstanceCount: 100,
			Resources:     testResourceReqs,
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
			InstanceCount: 3,
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
			InstanceCount: 3,
		},
		Role: "default",
	}
}

func getTCPProxySpec() v1.TCPProxiesSpec {
	return v1.TCPProxiesSpec{
		InstanceSpec: v1.InstanceSpec{
			InstanceCount: 3,
		},
		Role: "default",
	}
}
