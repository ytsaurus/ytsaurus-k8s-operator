package ytconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/canonize"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
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
	testBasicInstanceSpec = ytv1.InstanceSpec{InstanceCount: 3}
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
				Resources: corev1.VolumeResourceRequirements{
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
	ytsaurus := getYtsaurusWithoutNodes()
	canonize.AssertStruct(t, "ytsaurus", ytsaurus)
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetStrawberryInitClusterConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetControllerAgentsConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithoutNodes()
	canonize.AssertStruct(t, "ytsaurus", ytsaurus)
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
			ytsaurus := getYtsaurusWithoutNodes()
			canonize.AssertStruct(t, "ytsaurus", ytsaurus)
			g := NewLocalNodeGenerator(ytsaurus, ytsaurus.Name, testClusterDomain)
			spec := getDataNodeSpec(test.Location)
			canonize.AssertStruct(t, "data-node-"+name, spec)
			cfg, err := g.GetDataNodeConfig(spec)
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
	spec := getDataNodeSpec(testLocationChunkStore)
	canonize.AssertStruct(t, "data-node", spec)
	cfg, err := g.GetDataNodeConfig(spec)
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
			ytsaurus := getYtsaurusWithoutNodes()
			canonize.AssertStruct(t, "ytsaurus", ytsaurus)
			g := NewLocalNodeGenerator(ytsaurus, ytsaurus.Name, testClusterDomain)
			spec := getExecNodeSpec(test.JobResources)
			canonize.AssertStruct(t, "exec-node-"+name, spec)
			cfg, err := g.GetExecNodeConfig(spec)
			require.NoError(t, err)
			canonize.Assert(t, cfg)
		})
	}
}

func TestGetExecNodeConfigWithCri(t *testing.T) {
	ytsaurus := getYtsaurusWithoutNodes()
	canonize.AssertStruct(t, "ytsaurus", ytsaurus)
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
			canonize.AssertStruct(t, "exec-node-"+name, spec)
			cfg, err := g.GetExecNodeConfig(spec)
			require.NoError(t, err)
			canonize.Assert(t, cfg)
		})
	}
}

func TestGetContainerdConfig(t *testing.T) {
	ytsaurus := getYtsaurusWithoutNodes()
	canonize.AssertStruct(t, "ytsaurus", ytsaurus)
	g := NewLocalNodeGenerator(ytsaurus, ytsaurus.Name, testClusterDomain)
	spec := withCri(getExecNodeSpec(nil), nil, false)
	canonize.AssertStruct(t, "exec-node", spec)
	cfg, err := g.GetContainerdConfig(&spec)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetExecNodeWithoutYtsaurusConfig(t *testing.T) {
	remoteYtsaurus := getRemoteYtsaurus()
	commonSpec := getCommonSpec()
	nodeSpec := getExecNodeSpec(nil)
	canonize.AssertStruct(t, "remote-ytsaurus", remoteYtsaurus)
	canonize.AssertStruct(t, "common", commonSpec)
	canonize.AssertStruct(t, "exec-node", nodeSpec)
	g := NewRemoteNodeGenerator(
		remoteYtsaurus,
		testYtsaurusName,
		testClusterDomain,
		commonSpec,
	)
	cfg, err := g.GetExecNodeConfig(nodeSpec)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetHTTPProxyConfigDisableCreateOauthUser(t *testing.T) {
	spec := getYtsaurusWithoutNodes()
	spec.Spec.OauthService.DisableUserCreation = ptr.To(true)
	g := NewGenerator(spec, testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	proxySpec := getHTTPProxySpec()
	canonize.AssertStruct(t, "http-proxy", proxySpec)
	cfg, err := g.GetHTTPProxyConfig(proxySpec, g.GetComponentLabeller(consts.HttpProxyType, "control"))
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetHTTPProxyConfigEnableCreateOauthUser(t *testing.T) {
	spec := getYtsaurusWithoutNodes()
	spec.Spec.OauthService.DisableUserCreation = ptr.To(false)
	g := NewGenerator(spec, testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	proxySpec := getHTTPProxySpec()
	canonize.AssertStruct(t, "http-proxy", proxySpec)
	cfg, err := g.GetHTTPProxyConfig(proxySpec, g.GetComponentLabeller(consts.HttpProxyType, "control"))
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetHTTPProxyConfigWithAddresses(t *testing.T) {
	spec := getYtsaurusWithoutNodes()
	g := NewGenerator(spec, testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	proxySpec := getHTTPProxySpec()
	proxySpec.ExternalNetworkDomain = ptr.To("example.com")
	proxySpec.HttpPort = proxySpec.HttpNodePort
	proxySpec.HttpsPort = proxySpec.HttpsNodePort
	canonize.AssertStruct(t, "http-proxy", proxySpec)
	cfg, err := g.GetHTTPProxyConfig(proxySpec, g.GetComponentLabeller(consts.HttpProxyType, "control"))
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetHttpProxyConfigErrors(t *testing.T) {
	spec := getYtsaurusWithoutNodes()
	g := NewGenerator(spec, testClusterDomain)

	cases := []struct {
		name string
		mod  func(*ytv1.HTTPProxiesSpec) *ytv1.HTTPProxiesSpec
		err  string
	}{
		{
			name: "not NodePort",
			mod: func(spec *ytv1.HTTPProxiesSpec) *ytv1.HTTPProxiesSpec {
				spec.ServiceType = corev1.ServiceTypeClusterIP
				return spec
			},
			err: "invalid http-proxy spec, externalNetworkDomain is set but serviceType is not NodePort",
		},
		{
			name: "HttpPort != HttpNodePort (1)",
			mod: func(spec *ytv1.HTTPProxiesSpec) *ytv1.HTTPProxiesSpec {
				spec.HttpPort = ptr.To(int32(123))
				return spec
			},
			err: "invalid http-proxy spec, httpPort and httpNodePort must be equal or empty",
		},
		{
			name: "HttpPort != HttpNodePort (2)",
			mod: func(spec *ytv1.HTTPProxiesSpec) *ytv1.HTTPProxiesSpec {
				spec.HttpNodePort = ptr.To(int32(123))
				return spec
			},
			err: "invalid http-proxy spec, httpPort and httpNodePort must be equal or empty",
		},
		{
			name: "HttpPort != HttpNodePort (3)",
			mod: func(spec *ytv1.HTTPProxiesSpec) *ytv1.HTTPProxiesSpec {
				spec.HttpPort = nil
				return spec
			},
			err: "invalid http-proxy spec, httpPort and httpNodePort must be equal or empty",
		},
		{
			name: "HttpPort != HttpNodePort (4)",
			mod: func(spec *ytv1.HTTPProxiesSpec) *ytv1.HTTPProxiesSpec {
				spec.HttpNodePort = nil
				return spec
			},
			err: "invalid http-proxy spec, httpPort and httpNodePort must be equal or empty",
		},
		{
			name: "HttpsPort != HttpsNodePort (1)",
			mod: func(spec *ytv1.HTTPProxiesSpec) *ytv1.HTTPProxiesSpec {
				spec.HttpsPort = ptr.To(int32(123))
				return spec
			},
			err: "invalid http-proxy spec, httpsPort and httpsNodePort must be equal or empty",
		},
		{
			name: "HttpsPort != HttpsNodePort (2)",
			mod: func(spec *ytv1.HTTPProxiesSpec) *ytv1.HTTPProxiesSpec {
				spec.HttpsNodePort = ptr.To(int32(123))
				return spec
			},
			err: "invalid http-proxy spec, httpsPort and httpsNodePort must be equal or empty",
		},
		{
			name: "HttpsPort != HttpsNodePort (3)",
			mod: func(spec *ytv1.HTTPProxiesSpec) *ytv1.HTTPProxiesSpec {
				spec.HttpsPort = nil
				return spec
			},
			err: "invalid http-proxy spec, httpsPort and httpsNodePort must be equal or empty",
		},
		{
			name: "HttpsPort != HttpsNodePort (4)",
			mod: func(spec *ytv1.HTTPProxiesSpec) *ytv1.HTTPProxiesSpec {
				spec.HttpsNodePort = nil
				return spec
			},
			err: "invalid http-proxy spec, httpsPort and httpsNodePort must be equal or empty",
		},
	}

	l := g.GetComponentLabeller(consts.HttpProxyType, "control")

	for _, c := range cases {
		proxySpec := getHTTPProxySpec()
		proxySpec.ExternalNetworkDomain = ptr.To("example.com")
		proxySpec.HttpPort = ptr.To(*proxySpec.HttpNodePort)
		proxySpec.HttpsPort = ptr.To(*proxySpec.HttpsNodePort)

		t.Run(c.name, func(t *testing.T) {
			_, err := g.GetHTTPProxyConfig(*c.mod(&proxySpec), l)
			require.Error(t, err)
			require.Equal(t, c.err, err.Error())
		})
	}
}

func TestGetMasterWithFixedHostsConfig(t *testing.T) {
	ytsaurus := withFixedMasterHosts(getYtsaurus())
	canonize.AssertStruct(t, "ytsaurus", ytsaurus)
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetMasterConfig(&ytsaurus.Spec.PrimaryMasters)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetMasterWithMonitoringPortConfig(t *testing.T) {
	ytsaurus := withMasterMonitoringPort(getYtsaurus())
	canonize.AssertStruct(t, "ytsaurus", ytsaurus)
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetMasterConfig(&ytsaurus.Spec.PrimaryMasters)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetRPCProxyWithoutOauthConfig(t *testing.T) {
	g := NewGenerator(getYtsaurus(), testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	proxySpec := getRPCProxySpec()
	canonize.AssertStruct(t, "rpc-proxy", proxySpec)
	cfg, err := g.GetRPCProxyConfig(proxySpec)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetSchedulerWithoutTabletNodes(t *testing.T) {
	ytsaurus := getYtsaurusWithoutNodes()
	canonize.AssertStruct(t, "ytsaurus", ytsaurus)
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetSchedulerConfig(ytsaurus.Spec.Schedulers)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetSchedulerWithFixedMasterHostsConfig(t *testing.T) {
	ytsaurus := withFixedMasterHosts(withScheduler(getYtsaurus()))
	canonize.AssertStruct(t, "ytsaurus", ytsaurus)
	g := NewGenerator(ytsaurus, testClusterDomain)
	cfg, err := g.GetSchedulerConfig(ytsaurus.Spec.Schedulers)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetStrawberryControllerConfigWithExtendedHTTPMapping(t *testing.T) {
	g := NewGenerator(getYtsaurusWithoutNodes(), testClusterDomain)
	externalProxy := "some.domain"
	g.ytsaurus.Spec.StrawberryController.ExternalProxy = &externalProxy
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	cfg, err := g.GetStrawberryControllerConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetStrawberryControllerConfigWithCustomFamilies(t *testing.T) {
	g := NewGenerator(getYtsaurusWithoutNodes(), testClusterDomain)
	externalProxy := "some.domain"
	g.ytsaurus.Spec.StrawberryController.ExternalProxy = &externalProxy
	g.ytsaurus.Spec.StrawberryController.ControllerFamilies = append(
		g.ytsaurus.Spec.StrawberryController.ControllerFamilies,
		"superservice1", "superservice2", "superservice3",
	)
	defaultRouteFamily := "superservice2"
	g.ytsaurus.Spec.StrawberryController.DefaultRouteFamily = &defaultRouteFamily
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	cfg, err := g.GetStrawberryControllerConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetTabletNodeWithoutYtsaurusConfig(t *testing.T) {
	remoteYtsaurus := getRemoteYtsaurus()
	commonSpec := getCommonSpec()
	tabletNodeSpec := getTabletNodeSpec()
	canonize.AssertStruct(t, "remote-ytsaurus", remoteYtsaurus)
	canonize.AssertStruct(t, "common", commonSpec)
	canonize.AssertStruct(t, "tablet-node", tabletNodeSpec)
	g := NewRemoteNodeGenerator(
		remoteYtsaurus,
		testYtsaurusName,
		testClusterDomain,
		commonSpec,
	)
	cfg, err := g.GetTabletNodeConfig(tabletNodeSpec)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetTCPProxyConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithoutNodes(), testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	spec := getTCPProxySpec()
	canonize.AssertStruct(t, "tcp-proxy", spec)
	cfg, err := g.GetTCPProxyConfig(spec)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetUIClustersConfig(t *testing.T) {
	g := NewGenerator(getYtsaurusWithoutNodes(), testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	cfg, err := g.GetUIClustersConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetUIClustersConfigWithSettings(t *testing.T) {
	g := NewGenerator(withUICustom(getYtsaurus()), testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	cfg, err := g.GetUIClustersConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetUICustomConfig(t *testing.T) {
	g := NewGenerator(withUICustom(getYtsaurus()), testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	cfg, err := g.GetUICustomConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetUICustomConfigWithSettings(t *testing.T) {
	g := NewGenerator(withUICustomSettings(getYtsaurus()), testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	cfg, err := g.GetUICustomConfig()
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestResolverOptionsKeepSocketAndForceTCP(t *testing.T) {
	ytsaurus := getYtsaurusWithoutNodes()
	ytsaurus.Spec.CommonSpec.ForceTCP = ptr.To(true)
	ytsaurus.Spec.CommonSpec.KeepSocket = ptr.To(true)
	g := NewGenerator(ytsaurus, testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	cfg, err := g.GetMasterConfig(&ytsaurus.Spec.PrimaryMasters)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetMasterCachesWithFixedHostsConfig(t *testing.T) {
	ytsaurus := withFixedMasterCachesHosts(getYtsaurusWithoutNodes())
	g := NewGenerator(ytsaurus, testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)
	cfg, err := g.GetMasterCachesConfig(ytsaurus.Spec.MasterCaches)
	require.NoError(t, err)
	canonize.Assert(t, cfg)
}

func TestGetYtsaurusComponents(t *testing.T) {
	ytsaurus := getYtsaurusWithAllComponents()

	g := NewGenerator(ytsaurus, testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)

	for _, component := range consts.LocalComponentTypes {
		t.Run(string(component), func(t *testing.T) {
			names, err := g.GetComponentNames(component)
			require.NoError(t, err)
			require.Len(t, names, 1)
			cfg, err := g.GetComponentConfig(component, names[0])
			require.NoError(t, err)
			canonize.Assert(t, cfg)
		})
	}
}

func TestGetYtsaurusWithTlsInterconnect(t *testing.T) {
	ytsaurus := getYtsaurusWithAllComponents()

	ytsaurus.Spec.CABundle = &corev1.LocalObjectReference{
		Name: "ytsaurus-ca-bundle",
	}
	ytsaurus.Spec.NativeTransport = &ytv1.RPCTransportSpec{
		TLSSecret: &corev1.LocalObjectReference{
			Name: "ytsaurus-native-cert",
		},
		TLSRequired:                true,
		TLSInsecure:                true,                                 // not mTLS
		TLSPeerAlternativeHostName: testNamespace + ".svc.cluster.local", // or testNamespace.testClusterDomain ?
	}

	g := NewGenerator(ytsaurus, testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)

	for _, component := range consts.LocalComponentTypes {
		t.Run(string(component), func(t *testing.T) {
			names, err := g.GetComponentNames(component)
			require.NoError(t, err)
			require.Len(t, names, 1)
			cfg, err := g.GetComponentConfig(component, names[0])
			require.NoError(t, err)
			canonize.Assert(t, cfg)
		})
	}
}

func TestGetYtsaurusWithMutualTLSInterconnect(t *testing.T) {
	ytsaurus := getYtsaurusWithAllComponents()

	ytsaurus.Spec.CABundle = &corev1.LocalObjectReference{
		Name: "ytsaurus-ca-bundle",
	}
	ytsaurus.Spec.NativeTransport = &ytv1.RPCTransportSpec{
		TLSSecret: &corev1.LocalObjectReference{
			Name: "ytsaurus-native-cert",
		},
		TLSClientSecret: &corev1.LocalObjectReference{
			Name: "ytsaurus-native-client-cert",
		},
		TLSRequired:                true,
		TLSPeerAlternativeHostName: testNamespace + ".svc.cluster.local",
	}

	g := NewGenerator(ytsaurus, testClusterDomain)
	canonize.AssertStruct(t, "ytsaurus", g.ytsaurus)

	for _, component := range consts.LocalComponentTypes {
		t.Run(string(component), func(t *testing.T) {
			names, err := g.GetComponentNames(component)
			require.NoError(t, err)
			require.Len(t, names, 1)
			cfg, err := g.GetComponentConfig(component, names[0])
			require.NoError(t, err)
			canonize.Assert(t, cfg)
		})
	}
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
					InstanceCount: 1,

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
								Resources: corev1.VolumeResourceRequirements{
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

// TODO(khlebnikov): Get rid of this yet another spec generator.
func getYtsaurusWithoutNodes() *ytv1.Ytsaurus {
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
	ytsaurus = withKafkaProxies(ytsaurus)
	ytsaurus = withUI(ytsaurus)
	ytsaurus = withYQLAgent(ytsaurus)
	ytsaurus = withMasterCaches(ytsaurus)
	return ytsaurus
}

func getYtsaurusWithAllComponents() *ytv1.Ytsaurus {
	ytsaurus := getYtsaurusWithoutNodes()
	ytsaurus.Spec.HTTPProxies = append(ytsaurus.Spec.HTTPProxies, getHTTPProxySpec())
	ytsaurus.Spec.RPCProxies = append(ytsaurus.Spec.RPCProxies, getRPCProxySpec())
	ytsaurus.Spec.DataNodes = append(ytsaurus.Spec.DataNodes, getDataNodeSpec(testLocationChunkStore))
	ytsaurus.Spec.ExecNodes = append(ytsaurus.Spec.ExecNodes, getExecNodeSpec(nil))
	ytsaurus.Spec.TabletNodes = append(ytsaurus.Spec.TabletNodes, getTabletNodeSpec())
	return ytsaurus
}

func withControllerAgents(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.ControllerAgents = &ytv1.ControllerAgentsSpec{InstanceSpec: testBasicInstanceSpec}
	ytsaurus.Spec.UsePorto = true
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
			LoginTransformations: []ytv1.OauthUserLoginTransformation{
				{
					MatchPattern: "(.*)@ytsaurus.team",
					Replacement:  `\1`,
				},
			},
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
	return ytsaurus
}

func withQueryTracker(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{InstanceSpec: testBasicInstanceSpec}
	return ytsaurus
}

func withQueueAgent(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.QueueAgents = &ytv1.QueueAgentSpec{InstanceSpec: testBasicInstanceSpec}
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
	return ytsaurus
}

func withFixedMasterHosts(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.PrimaryMasters.HostAddresses = testMasterExternalHosts
	return ytsaurus
}

func withMasterMonitoringPort(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.PrimaryMasters.MonitoringPort = ptr.To(int32(20010))
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
	return ytsaurus
}

func withKafkaProxies(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.KafkaProxies = []ytv1.KafkaProxiesSpec{
		{
			InstanceSpec: testBasicInstanceSpec,
		},
	}
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
	return ytsaurus
}

func withYQLAgent(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.YQLAgents = &ytv1.YQLAgentSpec{InstanceSpec: testBasicInstanceSpec}
	return ytsaurus
}

func withFixedMasterCachesHosts(ytsaurus *ytv1.Ytsaurus) *ytv1.Ytsaurus {
	ytsaurus.Spec.MasterCaches.MasterCachesConnectionSpec.HostAddresses = testMasterCachesExternalHosts
	return ytsaurus
}

func getDataNodeSpec(locations ...ytv1.LocationSpec) ytv1.DataNodesSpec {
	return ytv1.DataNodesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount:        20,
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
			InstanceCount: 50,
			Resources:     testResourceReqs,
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
			InstanceCount: 100,
			Resources:     testResourceReqs,
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
			InstanceCount: 3,
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
			InstanceCount: 3,
		},
		Role: "default",
	}
}

func getTCPProxySpec() ytv1.TCPProxiesSpec {
	return ytv1.TCPProxiesSpec{
		InstanceSpec: ytv1.InstanceSpec{
			InstanceCount: 3,
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
			InstanceCount: 3,
		},
		HostAddressLabel: "",
	}
}

func getMasterCachesSpecWithFixedHosts() ytv1.MasterCachesSpec {
	spec := getMasterCachesSpec()
	spec.MasterCachesConnectionSpec.HostAddresses = testMasterCachesExternalHosts
	return spec
}
