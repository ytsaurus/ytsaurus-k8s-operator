package controllers_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/version"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"go.ytsaurus.tech/yt/go/mapreduce"
	ytspec "go.ytsaurus.tech/yt/go/mapreduce/spec"
	"go.ytsaurus.tech/yt/go/schema"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	"go.ytsaurus.tech/yt/go/yt/ytrpc"
	"go.ytsaurus.tech/yt/go/yterrors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ypatch"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

const (
	pollInterval     = time.Millisecond * 250
	reactionTimeout  = time.Second * 150
	bootstrapTimeout = time.Minute * 5
	upgradeTimeout   = time.Minute * 10
	imagePullTimeout = time.Minute * 10

	consistencyTimeout = time.Second * 10

	chytBootstrapTimeout = time.Minute * 5

	operationCPULimit    = 1
	operationMemoryLimit = 256 << 20

	operationPollInterval = time.Millisecond * 250
	operationTimeout      = time.Second * 120
	lightRequestTimeout   = time.Minute * 2
	httpRequestTimeout    = time.Minute * 5
)

func getYtClient(httpClient *http.Client, proxyAddress string) yt.Client {
	ytClient, err := ythttp.NewClient(&yt.Config{
		Proxy:                 proxyAddress,
		Token:                 consts.DefaultAdminPassword,
		LightRequestTimeout:   ptr.To(lightRequestTimeout),
		CompressionCodec:      yt.ClientCodecNone,
		DisableProxyDiscovery: true,
		Logger:                ytLogger,
		HTTPClient:            httpClient,
	})
	Expect(err).Should(Succeed())
	return ytClient
}

func getYtRPCClient(httpClient *http.Client, proxyAddress, rpcProxyAddress string) yt.Client {
	ytClient, err := ytrpc.NewClient(&yt.Config{
		Proxy:                 proxyAddress,
		RPCProxy:              rpcProxyAddress,
		Token:                 consts.DefaultAdminPassword,
		DisableProxyDiscovery: true,
		Logger:                ytLogger,
		HTTPClient:            httpClient,
	})
	Expect(err).Should(Succeed())
	return ytClient
}

func getHTTPProxyAddress(ctx context.Context, g *ytconfig.Generator, namespace, portName string) (string, error) {
	proxy := os.Getenv("E2E_YT_HTTP_PROXY")
	if proxy != "" {
		return proxy, nil
	}
	// This one is used in real code in YtsaurusClient.
	proxy = os.Getenv("YTOP_PROXY")
	if proxy != "" {
		return proxy, nil
	}

	if os.Getenv("E2E_YT_PROXY") != "" {
		panic("E2E_YT_PROXY is deprecated, use E2E_YT_HTTP_PROXY")
	}

	serviceName := g.GetHTTPProxiesServiceName(consts.DefaultHTTPProxyRole)

	return getServiceAddress(ctx, namespace, serviceName, portName)
}

func getRPCProxyAddress(ctx context.Context, g *ytconfig.Generator, namespace string) (string, error) {
	proxy := os.Getenv("E2E_YT_RPC_PROXY")
	if proxy != "" {
		return proxy, nil
	}

	serviceName := g.GetRPCProxiesServiceName(consts.DefaultName)

	portName := consts.YTRPCPortName
	if g.GetClusterFeatures().RPCProxyHavePublicAddress {
		portName = consts.YTPublicRPCPortName
	}

	return getServiceAddress(ctx, namespace, serviceName, portName)
}

func getMasterPod(name, namespace string) corev1.Pod {
	msPod := corev1.Pod{}
	err := k8sClient.Get(specCtx, client.ObjectKey{Name: name, Namespace: namespace}, &msPod)
	Expect(err).Should(Succeed())
	return msPod
}

func runImpossibleUpdateAndRollback(ytsaurus *ytv1.Ytsaurus, ytClient yt.Client) {
	By("Run cluster impossible update")
	Expect(ytsaurus.Spec.CoreImage).To(Equal(testutil.CurrentImages.Core))
	ytsaurus.Spec.CoreImage = testutil.FutureImages.Core
	UpdateObject(specCtx, ytsaurus)

	EventuallyYtsaurus(specCtx, ytsaurus, reactionTimeout).Should(HaveClusterUpdateState(ytv1.UpdateStateImpossibleToStart))

	By("Set previous core image")
	ytsaurus.Spec.CoreImage = testutil.CurrentImages.Core
	UpdateObject(specCtx, ytsaurus)

	By("Wait for running")
	EventuallyYtsaurus(specCtx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())

	By("Check that cluster alive after update")
	res := make([]string, 0)
	Expect(ytClient.ListNode(specCtx, ypath.Path("/"), &res, nil)).Should(Succeed())
}

type testRow struct {
	A string `yson:"a"`
}

var _ = Describe("Basic e2e test for Ytsaurus controller", Label("e2e"), func() {
	// NOTE: All context variables must be initialized BeforeEach, to prevent crosstalk.
	var namespace string
	var stopEventsLogger func()
	var requiredImages []string
	var objects []client.Object
	var namespaceWatcher *NamespaceWatcher
	var ytBuilder *testutil.YtsaurusBuilder
	var ytsaurus *ytv1.Ytsaurus
	var generator *ytconfig.Generator
	var certBuilder *testutil.CertBuilder
	var caBundleCertificates []byte
	var httpClient *http.Client
	var ytProxyAddress string
	var ytClient yt.Client
	var ytProxyAddressHTTPS string
	var ytClientHTTPS yt.Client
	var ytRPCProxyAddress string
	var ytRPCClient yt.Client
	var ytRPCTLSClient yt.Client
	var remoteComponentNames map[consts.ComponentType][]string

	// NOTE: execution order for each test spec:
	// - BeforeEach               (init, configuration)
	// - JustBeforeEach           (creation, validation)
	// - It                       (test itself)
	// - JustAfterEach            (diagnosis, validation)
	// - AfterEach, DeferCleanup  (cleanup)
	//
	// See:
	// https://onsi.github.io/ginkgo/#separating-creation-and-configuration-justbeforeeach
	// https://onsi.github.io/ginkgo/#spec-cleanup-aftereach-and-defercleanup
	// https://onsi.github.io/ginkgo/#separating-diagnostics-collection-and-teardown-justaftereach
	// NOTE: cross-node operations must use specCtx, in-node operations should use node ctx.

	withHTTPSProxy := func(httpsOnly bool) {
		By("Adding HTTPS proxy TLS certificates")
		httpProxyCert := certBuilder.BuildCertificate(ytsaurus.Name+"-http-proxy", []string{
			generator.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole),
			generator.GetHTTPProxiesServiceAddress(consts.DefaultHTTPProxyRole),
			generator.GetComponentLabeller(consts.HttpProxyType, consts.DefaultHTTPProxyRole).GetInstanceAddressWildcard(),
		})
		objects = append(objects, httpProxyCert)

		if httpsOnly {
			By("Adding HTTPS-only proxies")
		} else {
			By("Adding HTTP/HTTPS proxies")
		}
		ytBuilder.WithHTTPSProxies(httpProxyCert.Name, httpsOnly)
	}

	withRPCTLSProxy := func() {
		By("Adding RPC proxy TLS certificates")
		rpcProxyCert := certBuilder.BuildCertificate(ytsaurus.Name+"-rpc-proxy", []string{
			generator.GetComponentLabeller(consts.RpcProxyType, "").GetInstanceAddressWildcard(),
		})
		objects = append(objects, rpcProxyCert)

		ytBuilder.WithRPCProxyTLS = true
		ytsaurus.Spec.RPCProxies[0].Transport = ytv1.RPCTransportSpec{
			TLSSecret: &corev1.LocalObjectReference{
				Name: rpcProxyCert.Name,
			},
		}
	}

	BeforeEach(func(ctx context.Context) {
		By("Logging nodes state", func() {
			logNodesState(ctx)
		})

		By("Creating namespace")
		currentSpec := CurrentSpecReport()
		namespaceObject := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-e2e-",
				Labels: map[string]string{
					"app.kubernetes.io/component": "test",
					"app.kubernetes.io/name":      "test-" + strings.Join(currentSpec.Labels(), "-"),
					"app.kubernetes.io/part-of":   "ytsaurus-dev",
				},
				Annotations: map[string]string{
					"kubernetes.io/description": currentSpec.LeafNodeText,
				},
			},
		}
		Expect(k8sClient.Create(ctx, &namespaceObject)).Should(Succeed())

		namespace = namespaceObject.Name // Fetch unique namespace name
		log.Info("Namespace created", "namespace", namespace)
		DeferCleanup(AttachProgressReporter(func() string {
			return fmt.Sprintf("namespace: %s", namespace)
		}))

		By("Logging all events in namespace")
		stopEventsLogger = LogObjectEvents(specCtx, namespace)

		By("Logging some other objects in namespace")
		namespaceWatcher = NewNamespaceWatcher(specCtx, namespace)
		namespaceWatcher.Start()
		DeferCleanup(namespaceWatcher.Stop)

		By("Creating minimal Ytsaurus spec")
		ytBuilder = &testutil.YtsaurusBuilder{
			Images:    testutil.CurrentImages,
			Namespace: namespace,
		}
		ytBuilder.CreateMinimal()
		ytsaurus = ytBuilder.Ytsaurus
		ytsaurus.Spec.ClusterFeatures = &ytv1.ClusterFeatures{}

		requiredImages = nil
		objects = []client.Object{ytsaurus}

		By("Tracking Ytsaurus updates")
		DeferCleanup(TrackObjectUpdates(specCtx, ytsaurus, &ytv1.YtsaurusList{}, NewYtsaurusStatusTracker()))

		generator = ytconfig.NewGenerator(ytsaurus, "cluster.local")
		remoteComponentNames = make(map[consts.ComponentType][]string)

		certBuilder = &testutil.CertBuilder{
			Namespace:   namespace,
			IPAddresses: getNodesAddresses(ctx),
		}
	})

	// NOTE: AfterEach are executed in reverse order.
	AfterEach(func(ctx context.Context) {
		if stopEventsLogger != nil {
			By("Stopping namespace events logger")
			stopEventsLogger()
		}
	})

	AfterEach(func(ctx context.Context) {
		if ShouldPreserveArtifacts() {
			log.Info("Preserving artifacts", "namespace", namespace)
			return
		}

		By("Deleting resource objects")
		for _, object := range objects {
			By(fmt.Sprintf("Deleting %v %v", GetObjectGVK(object), object.GetName()))
			if err := k8sClient.Delete(ctx, object); err != nil {
				log.Error(err, "Cannot delete", "object", object.GetName())
			}
		}

		By("Deleting namespace", func() {
			var ns corev1.Namespace
			ns.SetName(namespace)
			Expect(k8sClient.Delete(ctx, &ns)).Should(Succeed())
			stop := AttachProgressReporter(func() string {
				return fmt.Sprintf("namespace: %+v", &ns)
			})
			defer stop()
			Eventually(ctx, func(ctx context.Context) error {
				return k8sClient.Get(ctx, client.ObjectKeyFromObject(&ns), &ns)
			}, reactionTimeout, pollInterval).To(MatchError(apierrors.IsNotFound, "IsNotFound"))
		})
	})

	JustBeforeEach(func(ctx context.Context) {
		By("Creating HTTP(s) client")
		httpTransport := &http.Transport{
			TLSHandshakeTimeout: 15 * time.Second,
			DisableCompression:  true,
			MaxConnsPerHost:     10,
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 1,
			IdleConnTimeout:     60 * time.Second,
		}
		httpClient = &http.Client{
			Transport: httpTransport,
			Timeout:   httpRequestTimeout,
		}

		if ytBuilder.WithHTTPSProxy {
			By("Fetching CA bundle certificates")
			// NOTE: CA bundle instead of full CA root bundle for more strict TLS verification.
			Eventually(func() (err error) {
				caBundleCertificates, err = readFileObject(ctx, namespace, ytv1.FileObjectReference{
					Name: testutil.TestCABundleName,
					Key:  consts.CABundleFileName,
				})
				return err
			}, bootstrapTimeout, pollInterval).Should(Succeed())
			Expect(caBundleCertificates).ToNot(BeEmpty())
			rootCAs := x509.NewCertPool()
			Expect(rootCAs.AppendCertsFromPEM(caBundleCertificates)).To(BeTrue())
			httpTransport.TLSClientConfig = &tls.Config{
				RootCAs: rootCAs,
			}
		}
	})

	JustBeforeEach(func(ctx context.Context) {
		// NOTE: Testcase should skip optional cases at "BeforeEach" stage.
		Expect(ytsaurus.Spec.CoreImage).ToNot(BeEmpty(), "ytsaurus core image is not specified")

		By("Pulling required images")
		requiredImages = append(requiredImages, ytsaurus.Spec.CoreImage)
		if spec := ytsaurus.Spec.QueryTrackers; spec != nil && spec.Image != nil {
			requiredImages = append(requiredImages, *spec.Image)
		}
		if spec := ytsaurus.Spec.YQLAgents; spec != nil && spec.Image != nil {
			requiredImages = append(requiredImages, *spec.Image)
		}
		pullImages(ctx, namespace, requiredImages, imagePullTimeout)

		By("Creating resource objects")
		for _, object := range objects {
			By(fmt.Sprintf("Creating %v %v", GetObjectGVK(object), object.GetName()))
			object.SetNamespace(namespace)
			Expect(k8sClient.Create(ctx, object)).Should(Succeed())
		}

		DeferCleanup(AttachProgressReporter(func() string {
			falseConditions := make(map[string]string)
			for _, condition := range ytsaurus.Status.Conditions {
				if condition.Status == metav1.ConditionFalse {
					falseConditions[condition.Type] = condition.Reason
				}
			}
			return fmt.Sprintf("ytsaurus false conditions: %v", falseConditions)
		}))

		By("Starting cluster health reporter")
		DeferCleanup(AttachProgressReporterWithContext(specCtx, func(ctx context.Context) string {
			proxyPort := consts.HTTPPortName
			if ytBuilder.WithHTTPSOnlyProxy {
				proxyPort = consts.HTTPSPortName
			}
			proxyAddress, err := getHTTPProxyAddress(ctx, generator, namespace, proxyPort)
			if err != nil {
				return fmt.Sprintf("cannot find proxy address: %v", err)
			}
			ytClient := getYtClient(httpClient, proxyPort+"://"+proxyAddress)
			// NOTE: WhoAmI right now does not retry.
			if _, err := ytClient.WhoAmI(ctx, nil); err != nil {
				return fmt.Sprintf("ytsaurus api error: %v", err)
			}
			var clusterHealth ClusterHealthReport
			clusterHealth.Collect(ctx, ytClient)
			return clusterHealth.String()
		}))

		DeferCleanup(AttachProgressReporterWithContext(specCtx, func(ctx context.Context) string {
			pending, failed := fetchStuckPods(specCtx, namespace)
			if len(pending) != 0 {
				logNodesState(specCtx)
			}
			if len(pending)+len(failed) != 0 {
				return fmt.Sprintf("Pods pending: %v, failed: %v", pending, failed)
			}
			return ""
		}))

		log.Info("Ytsaurus",
			"namespace", ytsaurus.Namespace,
			"name", ytsaurus.Name,
			"hostNetwork", ytsaurus.Spec.HostNetwork,
			"cellTag", ytsaurus.Spec.PrimaryMasters.CellTag,
			"coreImage", ytsaurus.Spec.CoreImage)

		By("Checking that Ytsaurus state is equal to `Running`", func() {
			EventuallyYtsaurus(ctx, ytsaurus, bootstrapTimeout).Should(HaveClusterStateRunning())
		})
	})

	JustBeforeEach(func(ctx context.Context) {
		By("Logging nodes state", func() {
			logNodesState(ctx)
		})

		var err error

		if !ytBuilder.WithHTTPSOnlyProxy {
			By("Creating YTsaurus HTTP client")
			ytProxyAddress, err = getHTTPProxyAddress(ctx, generator, namespace, consts.HTTPPortName)
			Expect(err).ToNot(HaveOccurred())
			ytProxyAddress = "http://" + ytProxyAddress

			ytClient = getYtClient(httpClient, ytProxyAddress)

			By("Checking YTsaurus HTTP client", func() {
				// NOTE: NodeExists retries - see ReadRetryParams.
				Expect(ytClient.NodeExists(ctx, ypath.Path("/"), nil)).To(BeTrue())
				// NOTE: WhoAmI right now does not retry.
				Eventually(ctx, func(ctx context.Context) (*yt.WhoAmIResult, error) {
					return ytClient.WhoAmI(ctx, nil)
				}).To(HaveField("Login", consts.DefaultAdminLogin))
			})
		}

		if ytBuilder.WithHTTPSProxy {
			By("Creating YTsaurus HTTPS client")
			ytProxyAddressHTTPS, err = getHTTPProxyAddress(ctx, generator, namespace, consts.HTTPSPortName)
			Expect(err).ToNot(HaveOccurred())
			ytProxyAddressHTTPS = "https://" + ytProxyAddressHTTPS

			ytClientHTTPS = getYtClient(httpClient, ytProxyAddressHTTPS)

			By("Checking YTsaurus HTTPS client", func() {
				// NOTE: NodeExists retries - see ReadRetryParams.
				Expect(ytClientHTTPS.NodeExists(ctx, ypath.Path("/"), nil)).To(BeTrue())
				// NOTE: WhoAmI right now does not retry.
				Eventually(ctx, func(ctx context.Context) (*yt.WhoAmIResult, error) {
					return ytClientHTTPS.WhoAmI(ctx, nil)
				}).To(HaveField("Login", consts.DefaultAdminLogin))
			})

			if ytBuilder.WithHTTPSOnlyProxy {
				ytProxyAddress = ytProxyAddressHTTPS
				ytClient = ytClientHTTPS
			}
		}

		DeferCleanup(AttachProgressReporter(func() string {
			return fmt.Sprintf("YT_PROXY=%s YT_TOKEN=%s", ytProxyAddress, consts.DefaultAdminPassword)
		}))

		log.Info("Ytsaurus access",
			"YT_PROXY", ytProxyAddress,
			"YT_PROXY HTTPS", ytProxyAddressHTTPS,
			"YT_TOKEN", consts.DefaultAdminPassword,
			"login", consts.DefaultAdminLogin,
			"password", consts.DefaultAdminPassword,
		)

		checkClusterHealth(ctx, ytClient)

		createTestUser(ytClient)
	})

	JustBeforeEach(func(ctx context.Context) {
		if !ytBuilder.WithRPCProxy {
			return
		}

		var err error

		// TODO(khlebnikov): Generalize for all components in cluster health report.
		By("Checking RPC proxies are registered")
		var rpcProxies []string
		Expect(ytClient.ListNode(ctx, ypath.Path("//sys/rpc_proxies"), &rpcProxies, nil)).Should(Succeed())
		Expect(rpcProxies).Should(HaveLen(int(ytsaurus.Spec.RPCProxies[0].InstanceCount)))

		By("Checking YT RPC Proxy discovery")
		proxies := discoverProxies(ctx, httpClient, ytProxyAddress, nil)
		Expect(proxies).ToNot(BeEmpty())
		Expect(proxies).To(HaveEach(HaveSuffix(fmt.Sprintf(":%v", consts.RPCProxyPublicRPCPort))))

		By("Creating ytsaurus RPC client")
		ytRPCProxyAddress, err = getRPCProxyAddress(ctx, generator, namespace)
		Expect(err).ToNot(HaveOccurred())
		ytRPCClient = getYtRPCClient(httpClient, ytProxyAddress, ytRPCProxyAddress)

		By("Checking RPC proxy is working")
		Expect(ytRPCClient.NodeExists(ctx, ypath.Path("/"), nil)).To(BeTrue())
	})

	JustBeforeEach(func(ctx context.Context) {
		if !ytBuilder.WithRPCProxyTLS {
			return
		}

		var err error

		By("Checking YT RPC TLS Client")
		ytRPCTLSClient, err = ytrpc.NewClient(&yt.Config{
			Proxy:                    ytProxyAddressHTTPS,
			RPCProxy:                 ytRPCProxyAddress,
			CertificateAuthorityData: caBundleCertificates,
			Token:                    consts.DefaultAdminPassword,
			DisableProxyDiscovery:    true,
			Logger:                   ytLogger,
			HTTPClient:               httpClient,
		})
		Expect(err).Should(Succeed())
		// Expect(ytTLSRPCClient.WhoAmI(ctx, nil)).To(HaveField("Login", consts.DefaultAdminLogin))

		// TODO(khlebnikov): Add API to verify TLS connectivity.

		_, err = ytRPCTLSClient.NodeExists(ctx, ypath.Path("/"), nil)
		Expect(err).Should(Succeed())
	})

	JustBeforeEach(func(ctx context.Context) {
		if len(ytsaurus.Spec.ExecNodes) == 0 {
			return
		}

		// NOTE: There is no reliable readiness signal for compute stack except active checks.
		op := NewVanillaOperation(ytClient)

		By("Waiting scheduler is ready to start operation", func() {
			Eventually(ctx, op.Start, bootstrapTimeout, pollInterval).Should(Succeed())
		})

		By("Waiting scheduler could provide operation status", func() {
			Eventually(ctx, op.Status, bootstrapTimeout, pollInterval).ShouldNot(BeNil())
		})

		By("Waiting operation completion", func() {
			op.Wait()
		})
	})

	JustBeforeEach(func(ctx context.Context) {
		if len(ytsaurus.Spec.ExecNodes) == 0 || len(ytsaurus.Spec.DataNodes) == 0 {
			log.Info("Skipping testing map operations without exec or data nodes")
			return
		}

		By("Checking map operation")
		op := NewMapTestOperation(ytClient)
		Expect(op.Start()).Should(Succeed())
		By("Waiting operation completion", func() {
			op.Wait()
		})
	})

	JustBeforeEach(func(ctx context.Context) {
		By("Checking cluster components")
		clusterComponents, err := ListClusterComponents(ctx, ytClient)
		Expect(err).To(Succeed())

		componentGroups := func(componentType consts.ComponentType) []string {
			names, err := generator.GetComponentNames(componentType)
			Expect(err).To(Succeed())
			return append(names, remoteComponentNames[componentType]...)
		}

		instances := map[consts.ComponentType]int{}
		for _, component := range clusterComponents {
			var busService BusService
			Eventually(func(ctx context.Context) error {
				var err error
				busService, err = component.GetBusService(ctx)
				return err
			}, bootstrapTimeout, pollInterval, ctx).Should(Succeed())

			log.Info("Cluster component instance",
				"type", component.Type,
				"address", component.Address,
				"name", busService.Name,
				"version", busService.Version,
				"start_time", busService.StartTime,
				"build_time", busService.BuildTime,
			)
			Expect(componentGroups(component.Type)).ToNot(BeEmpty(), "component %q", component.Type)
			instances[component.Type] += 1
		}

		for _, componentType := range consts.LocalComponentTypes {
			if consts.ComponentCypressPath(componentType) == "" {
				continue
			}
			groups := componentGroups(componentType)
			log.Info("Cluster component", "type", componentType, "groups", len(groups), "instances", instances[componentType])
			if len(groups) == 0 {
				Expect(instances[componentType]).To(BeZero(), "component %q", componentType)
			} else {
				Expect(instances[componentType]).To(BeNumerically(">=", len(groups)), "component %q", componentType)
			}
		}
	})

	JustBeforeEach(func(ctx context.Context) {
		chyt := ytBuilder.Chyt
		if chyt == nil {
			return
		}

		By("Checking CHYT status")
		Eventually(ctx, func(ctx context.Context) (*ytv1.Chyt, error) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(chyt), chyt)
			return chyt, err
		}, bootstrapTimeout, pollInterval).Should(
			HaveField("Status.ReleaseStatus", ytv1.ChytReleaseStatusFinished),
			"CHYT status: %+v", &chyt.Status,
		)

		By("Checking CHYT readiness")
		// FIXME(khlebnikov): There is no reliable readiness signal.
		Eventually(ctx, queryClickHouse, chytBootstrapTimeout, pollInterval).WithArguments(
			httpClient, ytProxyAddress, "SELECT 1",
		).MustPassRepeatedly(3).Should(Equal("1\n"))
	})

	JustBeforeEach(func(ctx context.Context) {
		if ytBuilder.Chyt == nil || ytsaurus.Spec.QueryTrackers == nil {
			return
		}

		By("Checking CHYT via query tracker")
		Expect(makeQuery(ctx, ytClient, yt.QueryEngineCHYT, "SELECT 1")).To(HaveLen(1))

		By("Creating table")
		Expect(makeQuery(ctx, ytClient, yt.QueryEngineCHYT,
			"CREATE TABLE `//tmp/chqt_test` ENGINE = YtTable() AS SELECT 1",
		)).To(BeNil())
	})

	JustBeforeEach(func(ctx context.Context) {
		if ytsaurus.Spec.YQLAgents == nil || ytsaurus.Spec.QueryTrackers == nil {
			return
		}

		By("Checking YQL via query tracker")
		Expect(makeQuery(ctx, ytClient, yt.QueryEngineYQL, "SELECT 1")).To(HaveLen(1))

		// FIXME(khlebnikov): CA Root Bundle handing is broken inside YQL DQ: https://github.com/ydb-platform/ydb/pull/30703
		if !ytBuilder.WithHTTPSProxy {
			By("Creating table")
			Expect(makeQuery(ctx, ytClient, yt.QueryEngineYQL,
				"INSERT INTO `//tmp/yql_test` SELECT 1",
			)).To(BeNil())
		}
	})

	JustAfterEach(func(ctx context.Context) {
		if CurrentSpecReport().Failed() {
			return
		}

		if len(ytsaurus.Spec.ExecNodes) != 0 && ytClient != nil {
			By("Running vanilla operation")
			op := NewVanillaOperation(ytClient)

			By("Waiting scheduler is ready to start operation")
			Eventually(ctx, op.Start, bootstrapTimeout, pollInterval).Should(Succeed())

			// FIXME(khlebnikov): In some cases cluster is broken and cannot run operations.
			By("Aborting operation")
			Expect(op.Abort()).To(Succeed())
		}
	})

	Context("Update scenarios", Label("update"), func() {
		var podsBeforeUpdate map[string]corev1.Pod

		BeforeEach(func() {
			By("Adding base components")
			ytBuilder.WithBaseComponents()
		})

		JustBeforeEach(func(ctx context.Context) {
			By("Getting pods before actions")
			podsBeforeUpdate = getComponentPods(ctx, namespace)
		})

		DescribeTableSubtree("Updating Ytsaurus image", Label("basic"),
			func(oldImage, newImage string) {
				BeforeEach(func() {
					if oldImage == "" || newImage == "" {
						Skip("Ytsaurus old or new image is not specified")
					}

					requiredImages = append(requiredImages, newImage)

					ytBuilder.WithNamedDataNodes(ptr.To("dn-t"))

					By("Setting old core image for testing upgrade")
					ytsaurus.Spec.CoreImage = oldImage

					if oldImage == testutil.YtsaurusImage23_2 {
						By("Disabling master caches in 23.2")
						ytsaurus.Spec.MasterCaches = nil
					}
				})

				It("Triggers cluster update", func(ctx context.Context) {
					By("Checking jobs order")
					completedJobs := namespaceWatcher.GetCompletedJobNames()
					Expect(completedJobs).Should(Equal(getInitializingStageJobNames()))

					checkPodLabels(ctx, namespace)

					log.Info("Update core image", "before", ytsaurus.Spec.CoreImage, "after", newImage)
					ytsaurus.Spec.CoreImage = newImage
					UpdateObject(ctx, ytsaurus)

					EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())

					By("Waiting cluster update completes")
					EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
					checkClusterHealth(ctx, ytClient)
					checkChunkLocations(ytClient)

					podsAfterFullUpdate := getComponentPods(ctx, namespace)
					pods := getChangedPods(podsBeforeUpdate, podsAfterFullUpdate)
					Expect(pods.Created).To(BeEmpty(), "created")
					Expect(pods.Deleted).To(BeEmpty(), "deleted")
					Expect(pods.Updated).To(ConsistOf(maps.Keys(podsBeforeUpdate)), "updated")

					CurrentlyObject(ctx, ytsaurus).Should(HaveObservedGeneration())
				})
			},
			Entry("When update Ytsaurus 23.2 -> 24.1", Label("24.1"), testutil.YtsaurusImage23_2, testutil.YtsaurusImage24_1),
			Entry("When update Ytsaurus 24.1 -> 24.2", Label("24.2"), testutil.YtsaurusImage24_1, testutil.YtsaurusImage24_2),
			Entry("When update Ytsaurus 24.2 -> 25.1", Label("25.1"), testutil.YtsaurusImage24_2, testutil.YtsaurusImage25_1),
			Entry("When update Ytsaurus 25.1 -> 25.2", Label("25.2"), testutil.YtsaurusImage25_1, testutil.YtsaurusImage25_2),
		)

		Context("Test update plan selector", Label("plan", "selector"), func() {

			BeforeEach(func() {
				By("Setting 24.2+ image for update selector tests")
				// This image is used for update selector tests because for earlier images
				// there will be migration of imaginary chunks locations which restarts datanodes
				// and makes it hard to test updateSelector.
				// For 24.2+ image no migration is needed.
				ytsaurus.Spec.CoreImage = testutil.YtsaurusImage24_2
			})

			It("Should be updated according to UpdateSelector=Everything", func(ctx context.Context) {

				By("Run cluster update with selector: class=Nothing")
				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassNothing}}
				updateSpecToTriggerAllComponentUpdate(ytsaurus)
				UpdateObject(ctx, ytsaurus)

				By("Ensure cluster doesn't start updating")
				ConsistentlyYtsaurus(ctx, ytsaurus, consistencyTimeout).Should(HaveClusterStateRunning())

				By("Ensure cluster generation is observed")
				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())
				Expect(ytsaurus).Should(HaveClusterStateRunning())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponents).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).ToNot(BeEmpty())

				By("Verifying that pods were not recreated")
				podsAfterBlockedUpdate := getComponentPods(ctx, namespace)
				Expect(podsBeforeUpdate).To(
					Equal(podsAfterBlockedUpdate),
					"pods shouldn't be recreated when update is blocked",
				)

				By("Update cluster update with selector: class=Everything")
				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassEverything}}
				UpdateObject(ctx, ytsaurus)

				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents(
					consts.ControllerAgentType,
					consts.DataNodeType,
					consts.DiscoveryType,
					consts.ExecNodeType,
					consts.HttpProxyType,
					consts.MasterType,
					consts.MasterCacheType,
					consts.SchedulerType,
					consts.TabletNodeType,
				))
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).ToNot(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).To(BeEmpty())

				By("Wait cluster update with full update complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				Expect(ytsaurus).Should(HaveObservedGeneration())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponents).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).To(BeEmpty())

				podsAfterFullUpdate := getComponentPods(ctx, namespace)
				pods := getChangedPods(podsBeforeUpdate, podsAfterFullUpdate)
				Expect(pods.Created).To(BeEmpty(), "created")
				Expect(pods.Deleted).To(BeEmpty(), "deleted")
				Expect(pods.Updated).To(ConsistOf(maps.Keys(podsBeforeUpdate)), "updated")
			})

			It("Should be updated according to UpdateSelector=TabletNodesOnly,ExecNodesOnly", func(ctx context.Context) {

				By("Run cluster update with selector:ExecNodesOnly")
				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{
					Component: ytv1.Component{
						Type: consts.ExecNodeType,
					},
				}}
				updateSpecToTriggerAllComponentUpdate(ytsaurus)
				UpdateObject(ctx, ytsaurus)

				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents(consts.ExecNodeType))
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).ToNot(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).ToNot(BeEmpty())

				By("Wait cluster update with selector:ExecNodesOnly complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				Expect(ytsaurus).Should(HaveObservedGeneration())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponents).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).ToNot(BeEmpty())

				checkClusterHealth(ctx, ytClient)

				podsAfterEndUpdate := getComponentPods(ctx, namespace)
				pods := getChangedPods(podsBeforeUpdate, podsAfterEndUpdate)
				Expect(pods.Created).To(BeEmpty(), "created")
				Expect(pods.Deleted).To(BeEmpty(), "deleted")
				Expect(pods.Updated).To(ConsistOf("end-0"), "updated")

				By("Run cluster update with selector:TabletNodesOnly")
				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{
					Component: ytv1.Component{
						Type: consts.TabletNodeType,
					},
				}}
				UpdateObject(ctx, ytsaurus)

				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents(consts.TabletNodeType))
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).ToNot(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).ToNot(BeEmpty())

				By("Wait cluster update with selector:TabletNodesOnly complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				Expect(ytsaurus).Should(HaveObservedGeneration())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponents).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).ToNot(BeEmpty())

				checkClusterHealth(ctx, ytClient)

				podsAfterTndUpdate := getComponentPods(ctx, namespace)
				pods = getChangedPods(podsAfterEndUpdate, podsAfterTndUpdate)
				Expect(pods.Created).To(BeEmpty(), "created")
				Expect(pods.Deleted).To(BeEmpty(), "deleted")
				Expect(pods.Updated).To(ConsistOf("tnd-0", "tnd-1", "tnd-2"), "updated")
			})

			It("Should be updated according to UpdateSelector=MasterOnly,StatelessOnly", func(ctx context.Context) {

				By("Run cluster update with selector:MasterOnly")
				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{
					Component: ytv1.Component{
						Type: consts.MasterType,
					},
				}}
				updateSpecToTriggerAllComponentUpdate(ytsaurus)
				UpdateObject(ctx, ytsaurus)

				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents(consts.MasterType))
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).ToNot(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).ToNot(BeEmpty())

				By("Wait cluster update with selector:MasterOnly complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				Expect(ytsaurus).Should(HaveObservedGeneration())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponents).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).ToNot(BeEmpty())

				checkClusterHealth(ctx, ytClient)

				podsAfterMasterUpdate := getComponentPods(ctx, namespace)
				pods := getChangedPods(podsBeforeUpdate, podsAfterMasterUpdate)
				Expect(pods.Created).To(BeEmpty(), "created")
				Expect(pods.Deleted).To(BeEmpty(), "deleted")
				Expect(pods.Updated).To(ConsistOf("ms-0"), "updated")

				By("Run cluster update with selector:StatelessOnly")
				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{
					Class: consts.ComponentClassStateless,
				}}
				UpdateObject(ctx, ytsaurus)

				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents(
					consts.ControllerAgentType,
					consts.DiscoveryType,
					consts.ExecNodeType,
					consts.HttpProxyType,
					consts.MasterCacheType,
					consts.SchedulerType,
				))
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).ToNot(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).ToNot(BeEmpty())

				By("Wait cluster update with selector:StatelessOnly complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				Expect(ytsaurus).Should(HaveObservedGeneration())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponents).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).ToNot(BeEmpty())

				checkClusterHealth(ctx, ytClient)
				podsAfterStatelessUpdate := getComponentPods(ctx, namespace)
				pods = getChangedPods(podsAfterMasterUpdate, podsAfterStatelessUpdate)
				Expect(pods.Deleted).To(BeEmpty(), "deleted")
				Expect(pods.Created).To(BeEmpty(), "created")
				Expect(pods.Updated).To(ConsistOf("ca-0", "ds-0", "end-0", "hp-0", "msc-0", "sch-0"), "updated")
			})

			It("Should update only specified data node group", func(ctx context.Context) {
				By("Adding second data node group")
				ytsaurus.Spec.DataNodes = append(ytsaurus.Spec.DataNodes, ytv1.DataNodesSpec{
					InstanceSpec: ytBuilder.CreateDataNodeInstanceSpec(1),
					Name:         "dn-2",
				})
				ytsaurus.Status.UpdateStatus.Conditions = append(ytsaurus.Status.UpdateStatus.Conditions, metav1.Condition{
					Type:    consts.ConditionDataNodesWithImaginaryChunksAbsent,
					Status:  metav1.ConditionTrue,
					Message: "Emulate that all data nodes have real chunks, since it is not relevant to this test",
				})
				UpdateObject(ctx, ytsaurus)

				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveClusterState(ytv1.ClusterStateReconfiguration))
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				podsBeforeUpdate = getComponentPods(ctx, namespace)

				By("Run cluster update with selector targeting only second data node group")
				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{
					Component: ytv1.Component{
						Type: consts.DataNodeType,
						Name: "dn-2",
					},
				}}
				updateSpecToTriggerAllComponentUpdate(ytsaurus)
				UpdateObject(ctx, ytsaurus)

				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())
				Expect(ytsaurus).Should(HaveClusterUpdatingComponentsNames("dn-2"))
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).ToNot(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).ToNot(BeEmpty())

				By("Wait cluster update with selector:DataNodesOnly complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				Expect(ytsaurus).Should(HaveObservedGeneration())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponents).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.UpdatingComponentsSummary).To(BeEmpty())
				Expect(ytsaurus.Status.UpdateStatus.BlockedComponentsSummary).ToNot(BeEmpty())

				checkClusterHealth(ctx, ytClient)

				podsAfterUpdate := getComponentPods(ctx, namespace)
				pods := getChangedPods(podsBeforeUpdate, podsAfterUpdate)
				Expect(pods.Created).To(BeEmpty(), "created")
				Expect(pods.Deleted).To(BeEmpty(), "deleted")
				// Only the second data node group should be updated
				Expect(pods.Updated).To(ConsistOf("dnd-dn-2-0"), "updated")
			})

		}) // update selector

		Context("With config overrides", Label("overrides"), func() {
			var overrides *corev1.ConfigMap

			BeforeEach(func() {
				ytBuilder.WithOverrides()
				overrides = ytBuilder.Overrides
				objects = append(objects, overrides)

				// configure exec nodes to use CRI environment
				ytsaurus.Spec.ExecNodes[0].JobEnvironment = &ytv1.JobEnvironmentSpec{
					CRI: &ytv1.CRIJobEnvironmentSpec{
						SandboxImage: ptr.To("registry.k8s.io/pause:3.8"),
					},
				}
				ytsaurus.Spec.ExecNodes[0].Locations = append(ytsaurus.Spec.ExecNodes[0].Locations, ytv1.LocationSpec{
					LocationType: ytv1.LocationTypeImageCache,
					Path:         "/yt/node-data/image-cache",
				})
				ytsaurus.Spec.JobImage = &ytsaurus.Spec.CoreImage
			})

			It("ConfigOverrides update should trigger reconciliation", func(ctx context.Context) {
				By("Updating config overrides")
				overrides.Data["ytserver-discovery.yson"] = `{resource_limits = {total_memory = 123456789;};}`
				overrides.Data["containerd.toml"] = `{"plugins" = {"io.containerd.grpc.v1.cri" = {registry = {configs = {"cr.test" = {auth = {username = user; password = password;}}}}}}}`
				UpdateObject(ctx, overrides)

				By("Waiting cluster update starts")
				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveClusterStateUpdating())
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents(
					consts.DiscoveryType,
					consts.ExecNodeType,
				)) // change in containerd.toml must trigger exec node update

				By("Waiting cluster update completes")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())

				podsAfterUpdate := getComponentPods(ctx, namespace)
				pods := getChangedPods(podsBeforeUpdate, podsAfterUpdate)
				Expect(pods.Created).To(BeEmpty(), "created")
				Expect(pods.Deleted).To(BeEmpty(), "deleted")
				Expect(pods.Updated).To(ConsistOf("ds-0", "end-0"), "updated")
			})

			It("Applies cypress patch by config overrides", func(ctx context.Context) {
				By("Updating config overrides")
				overrides.Data["cypress-patch.yson"] = YsonPretty(
					ypatch.PatchSet{
						"//sys/scheduler/config": {
							ypatch.Replace("/spec_template/issue_temporary_token", true),
						},
					},
				)
				UpdateObject(ctx, overrides)

				Eventually(func() (any, error) {
					var value any
					err := ytClient.GetNode(ctx, ypath.Path("//sys/scheduler/config/spec_template/issue_temporary_token"), &value, nil)
					return value, err
				}, upgradeTimeout, pollInterval).Should(BeTrue())
			})

		}) // update overrides

		Context("Misc update test cases", Label("misc"), func() {

			// This is a test for specific regression bug when master pods are recreated during PossibilityCheck stage.
			It("Master shouldn't be recreated before WaitingForPodsCreation state if config changes", Label("master"), func(ctx context.Context) {

				By("Store master pod creation time")
				msPod := getMasterPod(testutil.MasterPodName, namespace)
				msPodCreationFirstTimestamp := msPod.CreationTimestamp

				By("Setting maintenance to stop in PossibilityCheck", func() {
					var masters []string
					Expect(ytClient.ListNode(ctx, ypath.Path("//sys/primary_masters"), &masters, nil)).Should(Succeed())
					Expect(masters).Should(HaveLen(1))
					Expect(ytClient.SetNode(ctx, ypath.Path("//sys/primary_masters").Child(masters[0]).Attr("maintenance"), true, nil)).To(Succeed())
				})

				By("Run cluster update")
				ytsaurus.Spec.HostNetwork = ptr.To(true)
				ytsaurus.Spec.PrimaryMasters.HostAddresses = []string{
					getKindControlPlaneNode(ctx).Name,
				}
				UpdateObject(ctx, ytsaurus)

				By("Waiting PossibilityCheck")
				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveClusterUpdateState(ytv1.UpdateStatePossibilityCheck))

				By("Check that master pod was NOT recreated at the PossibilityCheck stage")
				// FIXME: rewrite this.
				time.Sleep(1 * time.Second)
				msPod = getMasterPod(testutil.MasterPodName, namespace)
				msPodCreationSecondTimestamp := msPod.CreationTimestamp
				log.Info("ms pods ts", "first", msPodCreationFirstTimestamp, "second", msPodCreationSecondTimestamp)
				Expect(msPodCreationFirstTimestamp.Equal(&msPodCreationSecondTimestamp)).Should(BeTrue())
			})

		}) // update misc

		Context("With tablet nodes", Label("tablet"), func() {

			BeforeEach(func() {
				ytBuilder.WithTabletNodes()
			})

			It("Should run and try to update Ytsaurus with tablet cell bundle which is not in `good` health", func(ctx context.Context) {

				By("Check that tablet cell bundles are in `good` health")
				Eventually(func() bool {
					notGoodBundles, err := components.GetNotGoodTabletCellBundles(ctx, ytClient)
					if err != nil {
						return false
					}
					return len(notGoodBundles) == 0
				}, upgradeTimeout, pollInterval).Should(BeTrue())

				By("Ban all tablet nodes")
				for i := range ytsaurus.Spec.TabletNodes[0].InstanceCount {
					Expect(ytClient.SetNode(ctx, ypath.Path(fmt.Sprintf(
						"//sys/cluster_nodes/tnd-%v.tablet-nodes.%v.svc.cluster.local:9022/@banned", i, namespace)), true, nil)).Should(Succeed())
				}

				By("Waiting tablet cell bundles are not in `good` health")
				Eventually(func() bool {
					notGoodBundles, err := components.GetNotGoodTabletCellBundles(ctx, ytClient)
					if err != nil {
						return false
					}
					return len(notGoodBundles) > 0
				}, reactionTimeout, pollInterval).Should(BeTrue())

				runImpossibleUpdateAndRollback(ytsaurus, ytClient)
			})

		}) // update tablet

		Context("With data nodes", Label("data"), func() {

			BeforeEach(func() {
				Expect(ytsaurus.Spec.DataNodes).ToNot(BeEmpty())

				// FIXME: Why?
				By("Removing tablet nodes")
				ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{}
			})

			It("Should run and try to update Ytsaurus with lvc", func(ctx context.Context) {

				By("Create a chunk")
				_, err := ytClient.CreateNode(ctx, ypath.Path("//tmp/a"), yt.NodeTable, nil)
				Expect(err).Should(Succeed())

				Eventually(func(g Gomega) {
					writer, err := ytClient.WriteTable(ctx, ypath.Path("//tmp/a"), nil)
					g.Expect(err).ShouldNot(HaveOccurred())
					g.Expect(writer.Write(testRow{A: "123"})).Should(Succeed())
					g.Expect(writer.Commit()).Should(Succeed())
				}, reactionTimeout, pollInterval).Should(Succeed())

				By("Ban all data nodes")
				for i := range ytsaurus.Spec.DataNodes[0].InstanceCount {
					node_path := ypath.Path(fmt.Sprintf("//sys/cluster_nodes/dnd-%v.data-nodes.%v.svc.cluster.local:9012/@banned", i, namespace))
					Expect(ytClient.SetNode(ctx, node_path, true, nil)).Should(Succeed())
				}

				By("Waiting for lvc > 0")
				Eventually(func() bool {
					lvcCount := 0
					err := ytClient.GetNode(ctx, ypath.Path("//sys/lost_vital_chunks/@count"), &lvcCount, nil)
					if err != nil {
						return false
					}
					return lvcCount > 0
				}, reactionTimeout, pollInterval).Should(BeTrue())

				runImpossibleUpdateAndRollback(ytsaurus, ytClient)
			})

		}) // update data

		Context("With query tracker", Label("query-tracker"), func() {

			BeforeEach(func() {
				ytBuilder.WithQueryTracker()
			})

			It("Should run with query tracker and check that query tracker rpc address set up correctly", func(ctx context.Context) {
				By("Check that query tracker channel exists in cluster_connection")
				Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/@cluster_connection/query_tracker/stages/production/channel"), nil)).Should(BeTrue())
			})

		}) // update query-tracker

		Context("With yql agent", Label("yql-agent"), func() {
			BeforeEach(func() {
				ytBuilder.WithQueryTracker()
				ytBuilder.WithYqlAgent()
			})

			It("Should run with yql agent and check that yql agent channel options set up correctly", func(ctx context.Context) {

				By("Check that yql agent channel exists in cluster_connection")
				Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/@cluster_connection/yql_agent/stages/production/channel"), nil)).Should(BeTrue())
				result := true
				Expect(ytClient.GetNode(ctx, ypath.Path("//sys/@cluster_connection/yql_agent/stages/production/channel/disable_balancing_on_single_address"), &result, nil)).Should(Succeed())
				Expect(result).Should(BeFalse())

				By("Check that access control object namespace 'queries' exists")
				Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/access_control_object_namespaces/queries"), nil)).Should(BeTrue())

				By("Check that access control object 'nobody' in namespace 'queries' exists")
				Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/access_control_object_namespaces/queries/nobody"), nil)).Should(BeTrue())

				By("Check that access control object 'everyone' in namespace 'queries' exists")
				Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/access_control_object_namespaces/queries/everyone"), nil)).Should(BeTrue())

				By("Check that access control object 'everyone-use' in namespace 'queries' exists")
				Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/access_control_object_namespaces/queries/everyone-use"), nil)).Should(BeTrue())

				By("Check that access control object 'everyone-share' in namespace 'queries' exists")
				Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/access_control_object_namespaces/queries/everyone-share"), nil)).Should(BeTrue())

				By("Check that access control object namespace 'queries' allows users to create children and owners to do everything")
				queriesAcl := []map[string]interface{}{}
				Expect(ytClient.GetNode(ctx, ypath.Path("//sys/access_control_object_namespaces/queries/@acl"), &queriesAcl, nil)).Should(Succeed())
				Expect(queriesAcl).Should(ConsistOf(
					SatisfyAll(
						HaveKeyWithValue("action", "allow"),
						HaveKeyWithValue("subjects", ConsistOf("users")),
						HaveKeyWithValue("permissions", ConsistOf("modify_children")),
						HaveKeyWithValue("inheritance_mode", "object_only"),
					),
					SatisfyAll(
						HaveKeyWithValue("action", "allow"),
						HaveKeyWithValue("subjects", ConsistOf("owner")),
						HaveKeyWithValue("permissions", ConsistOf("read", "write", "administer", "remove")),
						HaveKeyWithValue("inheritance_mode", "immediate_descendants_only"),
					),
				))

				By("Check that access control object 'everyone' in namespace 'queries' allows everyone to read and use")
				everyonePrincipalAcl := []map[string]interface{}{}
				Expect(ytClient.GetNode(ctx, ypath.Path("//sys/access_control_object_namespaces/queries/everyone/@principal_acl"), &everyonePrincipalAcl, nil)).Should(Succeed())
				Expect(everyonePrincipalAcl).Should(ConsistOf(
					SatisfyAll(
						HaveKeyWithValue("action", "allow"),
						HaveKeyWithValue("subjects", ConsistOf("everyone")),
						HaveKeyWithValue("permissions", ConsistOf("read", "use")),
					),
				))

				By("Check that access control object 'everyone-share' in namespace 'queries' allows everyone to read and use")
				everyoneSharePrincipalAcl := []map[string]interface{}{}
				Expect(ytClient.GetNode(ctx, ypath.Path("//sys/access_control_object_namespaces/queries/everyone-share/@principal_acl"), &everyoneSharePrincipalAcl, nil)).Should(Succeed())
				Expect(everyoneSharePrincipalAcl).Should(ConsistOf(
					SatisfyAll(
						HaveKeyWithValue("action", "allow"),
						HaveKeyWithValue("subjects", ConsistOf("everyone")),
						HaveKeyWithValue("permissions", ConsistOf("read", "use")),
					),
				))

				By("Check that access control object 'everyone-use' in namespace 'queries' allows everyone to use")
				everyoneUsePrincipalAcl := []map[string]interface{}{}
				Expect(ytClient.GetNode(ctx, ypath.Path("//sys/access_control_object_namespaces/queries/everyone-use/@principal_acl"), &everyoneUsePrincipalAcl, nil)).Should(Succeed())
				Expect(everyoneUsePrincipalAcl).Should(ConsistOf(
					SatisfyAll(
						HaveKeyWithValue("action", "allow"),
						HaveKeyWithValue("subjects", ConsistOf("everyone")),
						HaveKeyWithValue("permissions", ConsistOf("use")),
					),
				))
			})

		}) // update yql-agent

		Context("With queue agent", Label("queue-agent"), func() {
			BeforeEach(func() {
				ytBuilder.WithQueueAgent()
			})

			It("Should run with queue agent", func(ctx context.Context) {
				Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/queue_agents/queues"), nil)).Should(BeTrue())
				Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/queue_agents/consumers"), nil)).Should(BeTrue())
			})

		}) // update queue-agent

	}) // update

	Context("Remote ytsaurus tests", Label("remote"), func() {
		var remoteYtsaurus *ytv1.RemoteYtsaurus
		var remoteNodes client.Object

		BeforeEach(func() {
			remoteYtsaurus = &ytv1.RemoteYtsaurus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testutil.RemoteResourceName,
					Namespace: namespace,
				},
				Spec: ytv1.RemoteYtsaurusSpec{
					MasterConnectionSpec: ytv1.MasterConnectionSpec{
						CellTag: ytsaurus.Spec.PrimaryMasters.CellTag,
						HostAddresses: []string{
							fmt.Sprintf("ms-0.masters.%s.svc.cluster.local", namespace),
						},
					},
				},
			}
			objects = append(objects, remoteYtsaurus)
			remoteNodes = nil
		})

		JustBeforeEach(func(ctx context.Context) {
			By("Waiting remote nodes are Running")
			Expect(remoteNodes).ToNot(BeNil())
			// FIXME(khlebnikov): Why timeout is so large?
			EventuallyObject(ctx, remoteNodes, reactionTimeout*2).Should(HaveRemoteNodeReleaseStatusRunning())
			Expect(remoteNodes).To(HaveObservedGeneration())
		})

		Context("With remote exec nodes", Label("exec"), func() {
			BeforeEach(func() {
				ytBuilder.WithBaseComponents()

				By("Adding remote exec nodes")

				// Ensure that no local exec nodes exist, only remote ones (which will be created later).
				ytsaurus.Spec.ExecNodes = nil

				remoteComponentNames[consts.ExecNodeType] = []string{testutil.RemoteResourceName}
				remoteNodes = &ytv1.RemoteExecNodes{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testutil.RemoteResourceName,
						Namespace: namespace,
					},
					Spec: ytv1.RemoteExecNodesSpec{
						RemoteClusterSpec: &corev1.LocalObjectReference{
							Name: testutil.RemoteResourceName,
						},
						CommonSpec: ytv1.CommonSpec{
							CoreImage: testutil.CurrentImages.Core,
							JobImage:  ptr.To(testutil.CurrentImages.Job),
						},
						ExecNodesSpec: ytBuilder.CreateExecNodeSpec(),
					},
				}
				objects = append(objects, remoteNodes)
			})

			It("Should create ytsaurus with remote exec nodes and execute a job", func(ctx context.Context) {

				By("Running running vanilla operation")
				NewVanillaOperation(ytClient).Run()

				By("Running sort operation to ensure exec node works")
				op := runAndCheckSortOperation(ytClient)

				result, err := ytClient.ListJobs(ctx, op.ID(), &yt.ListJobsOptions{
					JobState: &yt.JobCompleted,
				})
				Expect(err).ShouldNot(HaveOccurred())
				for _, job := range result.Jobs {
					fmt.Println(job)
				}

				statuses, err := yt.ListAllJobs(ctx, ytClient, op.ID(), &yt.ListJobsOptions{
					JobState: &yt.JobCompleted,
				})
				Expect(err).ShouldNot(HaveOccurred())
				// This is flaking for some reason. Sometimes list of jobs is empty.
				// Disabling it for now.
				// Expect(statuses).Should(Not(BeEmpty()))
				for _, status := range statuses {
					Expect(status.Address).Should(
						ContainSubstring("end-"+testutil.RemoteResourceName),
						"actual status: %s", status,
					)
				}
			})

		}) // remote exec

		Context("With remote data nodes", Label("data"), func() {
			BeforeEach(func() {
				By("Adding remote data nodes")
				remoteComponentNames[consts.DataNodeType] = []string{testutil.RemoteResourceName}
				remoteNodes = &ytv1.RemoteDataNodes{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testutil.RemoteResourceName,
						Namespace: namespace,
					},
					Spec: ytv1.RemoteDataNodesSpec{
						RemoteClusterSpec: &corev1.LocalObjectReference{
							Name: testutil.RemoteResourceName,
						},
						CommonSpec: ytv1.CommonSpec{
							CoreImage: testutil.CurrentImages.Core,
						},
						DataNodesSpec: ytv1.DataNodesSpec{
							InstanceSpec: ytBuilder.CreateDataNodeInstanceSpec(3),
						},
					},
				}
				objects = append(objects, remoteNodes)
			})

			It("Should create ytsaurus with remote data nodes and write to a table", func(ctx context.Context) {

				By("Create a chunk on a remote data node")
				_, errCreateNode := ytClient.CreateNode(ctx, ypath.Path("//tmp/a"), yt.NodeTable, &yt.CreateNodeOptions{})
				Expect(errCreateNode).Should(Succeed())

				Eventually(func(g Gomega) {
					writer, err := ytClient.WriteTable(ctx, ypath.Path("//tmp/a"), nil)
					g.Expect(err).ShouldNot(HaveOccurred())
					g.Expect(writer.Write(testRow{A: "123"})).Should(Succeed())
					g.Expect(writer.Commit()).Should(Succeed())
				}, reactionTimeout, pollInterval).Should(Succeed())
			})

		}) // remote data

		Context("With remote tablet nodes", Label("tablet"), func() {
			BeforeEach(func() {
				// We have to create the minimal YT - with master, discovery servers and data-nodes
				ytBuilder.WithDataNodes()

				By("Adding remote tablet nodes")
				remoteComponentNames[consts.TabletNodeType] = []string{testutil.RemoteResourceName}
				remoteNodes = &ytv1.RemoteTabletNodes{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testutil.RemoteResourceName,
						Namespace: namespace,
					},
					Spec: ytv1.RemoteTabletNodesSpec{
						RemoteClusterSpec: &corev1.LocalObjectReference{
							Name: testutil.RemoteResourceName,
						},
						CommonSpec: ytv1.CommonSpec{
							CoreImage: testutil.CurrentImages.Core,
						},
						TabletNodesSpec: ytv1.TabletNodesSpec{
							InstanceSpec: ytBuilder.CreateTabletNodeSpec(3),
						},
					},
				}
				objects = append(objects, remoteNodes)
			})

			It("Should create ytsaurus with remote tablet nodes and write to a table", func(ctx context.Context) {

				dynamicTableTest := "//sys/test-tnd-remote"

				type newOperation struct {
					ID       string `yson:"id,key"`
					Status   string `yson:"status"`
					Restarts uint64 `yson:"restarts"`
				}

				newSchema, err := schema.Infer(newOperation{})
				Expect(err).Should(Succeed())

				By("Creating a tablet cell in the default bundle client")
				tabletCellID, errCreateTabletCell := ytClient.CreateObject(ctx, "tablet_cell", &yt.CreateObjectOptions{
					Attributes: map[string]interface{}{
						"tablet_cell_bundle": components.DefaultBundle,
					},
				})
				Expect(errCreateTabletCell).Should(Succeed())
				Eventually(func() bool {
					isTableCellHealthGood, err := components.WaitTabletCellHealth(ctx, ytClient, tabletCellID)
					if err != nil {
						return false
					}
					return isTableCellHealthGood
				}, upgradeTimeout, pollInterval).Should(BeTrue())

				By("Create a dynamic table on a remote tablet node")

				_, errCreateNode := ytClient.CreateNode(ctx, ypath.Path(dynamicTableTest), yt.NodeTable, &yt.CreateNodeOptions{
					Attributes: map[string]interface{}{
						"dynamic": true,
						"schema":  newSchema,
					},
				})
				Expect(errCreateNode).Should(Succeed())

				By("Check that tablet cell bundles are in `good` health")

				Eventually(func() bool {
					notGoodBundles, err := components.GetNotGoodTabletCellBundles(ctx, ytClient)
					if err != nil {
						return false
					}
					return len(notGoodBundles) == 0
				}, upgradeTimeout, pollInterval).Should(BeTrue())

				By("Mounting the dynamic table")
				errMount := ytClient.MountTable(ctx, ypath.Path(dynamicTableTest), &yt.MountTableOptions{})
				Expect(errMount).Should(Succeed())

				Eventually(func() bool {
					isTableMounted, err := components.WaitTabletStateMounted(ctx, ytClient, ypath.Path(dynamicTableTest))
					if err != nil {
						return false
					}
					return isTableMounted
				}, upgradeTimeout, pollInterval).Should(BeTrue())

				By("Inserting a row into dynamic table")
				Eventually(func(g Gomega) {
					var testRow newOperation
					testRow.ID = "1"
					testRow.Status = "Completed"
					testRow.Restarts = 2
					errInsertRows := ytClient.InsertRows(ctx, ypath.Path(dynamicTableTest), []any{&testRow}, nil)
					g.Expect(errInsertRows).ShouldNot(HaveOccurred())
				}, reactionTimeout, pollInterval).Should(Succeed())
			})

		}) // remote tablet

	}) // remote

	Context("Integration tests", Label("integration"), func() {

		Context("With CRI job environment", Label("cri", "containerd"), func() {

			BeforeEach(func() {
				By("Adding exec nodes")
				ytBuilder.WithScheduler()
				ytBuilder.WithControllerAgents()
				ytBuilder.WithExecNodes()
				ytBuilder.WithDataNodes()

				By("Adding CRI job environment")
				ytBuilder.WithCRIJobEnvironment()
			})

			It("Verify CRI job environment", func(ctx context.Context) {
				// TODO(khlebnikov): Check docker image and resource limits.
			})

		}) // integration cri

		Context("With CRI-O", Label("cri", "crio"), func() {

			BeforeEach(func() {
				ytsaurus.Spec.CoreImage = testutil.YtsaurusImage25_2
				requiredImages = append(requiredImages, ytsaurus.Spec.CoreImage)

				By("Adding exec nodes")
				ytBuilder.WithScheduler()
				ytBuilder.WithControllerAgents()
				ytBuilder.WithExecNodes()
				ytBuilder.WithDataNodes()

				By("Adding CRI-O job environment")
				ytBuilder.CRIService = ptr.To(ytv1.CRIServiceCRIO)
				ytBuilder.WithCRIJobEnvironment()

				if os.Getenv("YTSAURUS_CRIO_READY") == "" {
					By("Adding CRI-O install script")

					installCRIOScript := `#!/bin/bash
# Install CRI-O and CRI tools, see https://cri-o.io/
set -eux -o pipefail
: ${K8S_VERSION=v1.34}
: ${CRIO_VERSION=v1.34}
mkdir -p /etc/apt/keyrings
curl -fsSL "https://pkgs.k8s.io/core:/stable:/${K8S_VERSION}/deb/Release.key" -o /etc/apt/keyrings/kubernetes-apt-keyring.asc
curl -fsSL "https://download.opensuse.org/repositories/isv:/cri-o:/stable:/${CRIO_VERSION}/deb/Release.key" -o /etc/apt/keyrings/cri-o-apt-keyring.asc
tee /etc/apt/sources.list.d/kubernetes.list <<EOF
deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.asc] https://pkgs.k8s.io/core:/stable:/${K8S_VERSION}/deb/ /
EOF
tee /etc/apt/sources.list.d/cri-o.list <<EOF
deb [signed-by=/etc/apt/keyrings/cri-o-apt-keyring.asc] https://download.opensuse.org/repositories/isv:/cri-o:/stable:/${CRIO_VERSION}/deb/ /
EOF
export DEBIAN_FRONTEND=noninteractive
apt update -q --error-on=any
apt install -y --no-install-recommends cri-tools cri-o
exec "$@"`

					scripts := corev1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name: "scripts",
						},
						Data: map[string]string{
							"install-crio.sh": installCRIOScript,
						},
					}

					objects = append(objects, &scripts)
					execNode := &ytsaurus.Spec.ExecNodes[0]
					execNode.Volumes = append(execNode.Volumes, ytv1.Volume{
						Name: "scripts",
						VolumeSource: ytv1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "scripts",
								},
								DefaultMode: ptr.To(int32(0o555)),
							},
						},
					})
					execNode.VolumeMounts = append(execNode.VolumeMounts, corev1.VolumeMount{
						Name:      "scripts",
						MountPath: "/yt/scripts",
						ReadOnly:  true,
					})
					execNode.JobEnvironment.CRI.EntrypointWrapper = []string{
						"tini",
						"--",
						"/yt/scripts/install-crio.sh",
					}
				}
			})

			It("Verify CRI-O job environment", Label("crun"), func(ctx context.Context) {
				// TODO(khlebnikov): Check docker image and resource limits.
			})

			Context("With runc runtime", Label("runc"), func() {
				BeforeEach(func() {
					ytBuilder.WithOverrides()
					overrides := ytBuilder.Overrides
					objects = append(objects, overrides)
					overrides.Data["crio.conf"] = `{"crio" = { "runtime" = { "default_runtime" = "runc" }}}`
				})
				It("Verify CRI-O job environment", func(ctx context.Context) {
				})
			})

			Context("With nvidia runtime", Label("nvidia"), func() {
				BeforeEach(func() {
					ytBuilder.WithNvidiaContainerRuntime()
				})
				It("Verify CRI-O job environment", func(ctx context.Context) {
				})
			})

		}) // integration crio

		Context("With RPC proxy", Label("rpc-proxy"), func() {
			BeforeEach(func() {
				ytBuilder.WithRPCProxies()
			})

			It("Rpc proxies should require authentication", func(ctx context.Context) {
				// Just in case, so dirty dev environment wouldn't interfere.
				Expect(os.Unsetenv("YT_TOKEN")).Should(Succeed())

				By("Checking RPC proxy without token does not work")
				cli, err := ytrpc.NewClient(&yt.Config{
					Proxy:                 ytProxyAddress,
					RPCProxy:              ytRPCProxyAddress,
					DisableProxyDiscovery: true,
					Logger:                ytLogger,
				})
				Expect(err).Should(Succeed())

				_, err = cli.NodeExists(ctx, ypath.Path("/"), nil)
				Expect(yterrors.ContainsErrorCode(err, yterrors.CodeRPCAuthenticationError)).Should(BeTrue())

				By("Checking RPC proxy works with token")
				cli = getYtRPCClient(httpClient, ytProxyAddress, ytRPCProxyAddress)
				// Expect(cli.WhoAmI(ctx, nil)).To(HaveField("Login", consts.DefaultAdminLogin))

				_, err = cli.NodeExists(ctx, ypath.Path("/"), nil)
				Expect(err).Should(Succeed())
			})

		}) // integration rpc-proxy

		Context("With host network", Label("host-network"), func() {

			BeforeEach(func() {
				ytsaurus.Spec.HostNetwork = ptr.To(true)
			})

			It("Sensors should be annotated with host", func(ctx context.Context) {
				var primaryMasters []string
				Expect(ytClient.ListNode(ctx, ypath.Path("//sys/primary_masters"), &primaryMasters, nil)).Should(Succeed())
				Expect(primaryMasters).Should(Not(BeEmpty()))

				masterAddress := primaryMasters[0]

				monitoringPort := 0
				Expect(ytClient.GetNode(ctx, ypath.Path(fmt.Sprintf("//sys/primary_masters/%v/orchid/config/monitoring_port", masterAddress)), &monitoringPort, nil)).Should(Succeed())

				msPod := getMasterPod(testutil.MasterPodName, namespace)
				log.Info("Pod", "ip", msPod.Status.PodIP)

				rsp, err := http.Get(fmt.Sprintf(
					"http://%v:%v/solomon/all",
					msPod.Status.PodIP,
					monitoringPort))
				Expect(err).Should(Succeed())
				Expect(rsp.StatusCode).Should(Equal(http.StatusOK))

				body, err := io.ReadAll(rsp.Body)
				Expect(err).Should(Succeed())

				Expect(json.Valid(body)).Should(BeTrue())

				var parsedBody map[string]any
				Expect(json.Unmarshal(body, &parsedBody)).Should(Succeed())

				sensors, ok := parsedBody["sensors"].([]any)
				Expect(ok).Should(BeTrue())
				Expect(sensors).ShouldNot(BeEmpty())

				sensor, ok := sensors[0].(map[string]any)
				Expect(ok).Should(BeTrue())
				Expect("labels").Should(BeKeyOf(sensor))

				labels, ok := sensor["labels"].(map[string]any)
				Expect(ok).Should(BeTrue())
				Expect("host").Should(BeKeyOf(labels))
				Expect("pod").Should(BeKeyOf(labels))
				// We don not check the actual value, since it is something weird in our testing setup.
				Expect(labels["host"]).ShouldNot(BeEmpty())
				Expect(labels["pod"]).Should(Equal("ms-0"))
				log.Info("Result", "host", labels["host"], "pod", labels["pod"])
			})

		}) // integration host-network

		Context("With CHYT", Label("chyt"), func() {

			BeforeEach(func() {
				By("Adding strawberry")
				ytBuilder.WithBaseComponents()
				ytBuilder.WithStrawberryController()

				By("Adding CHYT instance")
				chyt := ytBuilder.CreateChyt()
				objects = append(objects, chyt)
			})

			It("Checks ClickHouse", func(ctx context.Context) {
				By("Creating table")
				Expect(queryClickHouse(ctx, httpClient, ytProxyAddress,
					"CREATE TABLE `//tmp/chyt_test` ENGINE = YtTable() AS SELECT 1",
				)).To(Equal(""))
			})

		}) // integration chyt

		DescribeTableSubtree("With Bus RPC TLS", Label("tls"), func(images testutil.YtsaurusImages) {
			var nativeServerCert, nativeClientCert *certv1.Certificate

			BeforeEach(func() {
				log.Info("YTsaurus images",
					"coreImage", images.Core,
					"chytImage", images.Chyt,
					"strawberry", images.Strawberry,
					"queryTracker", images.QueryTracker,
				)
				if images.Core == "" || images.Chyt == "" || images.Strawberry == "" || images.QueryTracker == "" {
					Skip("One of required images is not specified")
				}

				requiredImages = append(requiredImages, images.Core, images.Chyt, images.Strawberry, images.QueryTracker)

				By("Adding native transport TLS certificates")
				nativeServerCert = certBuilder.BuildCertificate(ytsaurus.Name+"-server", []string{
					ytsaurus.Name,
				})
				nativeClientCert = certBuilder.BuildCertificate(ytsaurus.Name+"-client", []string{
					ytsaurus.Name,
				})
				objects = append(objects,
					nativeServerCert,
					nativeClientCert,
				)

				if images.MutualTLSReady {
					By("Enabling RPC proxy public address")
					ytsaurus.Spec.ClusterFeatures.RPCProxyHavePublicAddress = true

					By("Activating mutual TLS interconnect")
					ytBuilder.WithNativeTransportTLS(nativeServerCert.Name, nativeClientCert.Name)
				} else {
					By("Activating TLS-only interconnect")
					ytBuilder.WithNativeTransportTLS(nativeServerCert.Name, "")
				}

				By("Adding all components")
				ytsaurus.Spec.CoreImage = images.Core
				ytBuilder.Images = images
				ytBuilder.WithBaseComponents()
				ytBuilder.WithRPCProxies()
				ytBuilder.WithQueryTracker()
				ytBuilder.WithYqlAgent()
				ytBuilder.WithStrawberryController()

				// FIXME(khlebnikov): Workaround for bug in strawberry controller logging.
				ytsaurus.Spec.StrawberryController.LogToStderr = true

				withHTTPSProxy(true)

				withRPCTLSProxy()

				By("Adding CHYT instance")
				chyt := ytBuilder.CreateChyt()
				chyt.Spec.Image = images.Chyt
				objects = append(objects, chyt)
			})

			JustAfterEach(func(ctx context.Context) {
				// TODO(khlebnikov): Ping component pairs.
				// TODO(khlebnikov): Poke CHYT servers orchid.
				By("Checking connections between cluster components")
				clusterComponents, err := ListClusterComponents(ctx, ytClient)
				Expect(err).To(Succeed())
				for _, component := range clusterComponents {
					// NOTE: This triggers connection between master and instance to fetch orchid.
					conns, err := component.GetBusConnections(ctx)
					Expect(err).To(Succeed())
					log.Info("Bus connections", "type", component.Type, "node", component.Address, "count", len(conns))
					for connID, conn := range conns {
						if !conn.Encrypted && component.Type == consts.RpcProxyType {
							continue
						}
						Expect(conn.Encrypted).To(BeTrue(), "connection %q between %q and %q band %q", connID, component.Address, conn.Address, conn.MultiplexingBand)
					}
				}
			})

			It("Verify that mTLS is active", func(ctx context.Context) {
				By("Getting CHYT operation id")
				clickHouseID, err := queryClickHouseID(ctx, httpClient, ytProxyAddress)
				Expect(err).To(Succeed())

				By("Creating table //tmp/chyt_test")
				Expect(queryClickHouse(ctx, httpClient, ytProxyAddress,
					"CREATE TABLE `//tmp/chyt_test` ENGINE = YtTable() AS SELECT 1",
				)).To(Equal(""))

				By("Reissuing RPC TLS certificates", func() {
					reissueCertificate(ctx, nativeServerCert)
					reissueCertificate(ctx, nativeClientCert)
				})

				// Restart http proxy to drop bus connection cache
				// TODO(khlebnikov): Add better management to orchid.
				By("Restarting http proxy", func() {
					podName := fmt.Sprintf("%s-%d", generator.GetComponentLabeller(consts.HttpProxyType, "").GetServerStatefulSetName(), 0)
					restartPod(ctx, namespace, podName)
				})

				By("Waiting http proxy", func() {
					Eventually(func(ctx context.Context) (bool, error) {
						return ytClient.NodeExists(ctx, ypath.Path("/"), nil)
					}, bootstrapTimeout, pollInterval, ctx).MustPassRepeatedly(5).Should(BeTrue())
				})

				// FIXME(khlebnikov): Workaround for DNS issues inside cluster.
				By("Waiting cluster components", func() {
					components, err := ListClusterComponents(ctx, ytClient)
					Expect(err).To(Succeed())
					for _, component := range components {
						Eventually(component.GetBusService, bootstrapTimeout, pollInterval, ctx).ShouldNot(HaveField("Name", ""))
					}
				})

				if images.StrawberryHandlesRestarts {
					By("Waiting for chyt operation restart by strawberry")
					Eventually(ctx, queryClickHouseID, chytBootstrapTimeout).WithArguments(httpClient, ytProxyAddress).ToNot(Equal(clickHouseID))
				} else {
					By("Aborting chyt operation")
					Expect(ytClient.AbortOperation(ctx, yt.OperationID(clickHouseID), nil)).To(Succeed())
				}

				By("Waiting CHYT readiness")
				Eventually(ctx, queryClickHouse, chytBootstrapTimeout, pollInterval).WithArguments(
					httpClient, ytProxyAddress, "SELECT 1",
				).MustPassRepeatedly(3).Should(Equal("1\n"))

				By("Checking table //tmp/chyt_test")
				Expect(queryClickHouse(ctx, httpClient, ytProxyAddress,
					"SELECT * FROM `//tmp/chyt_test`;",
				)).To(Equal("1\n"))
			})
		},
			Entry("YTsaurus 24.2", Label("24.2"), testutil.YtsaurusImages24_2),
			Entry("YTsaurus 25.1", Label("25.1"), testutil.YtsaurusImages25_1),
			Entry("YTsaurus 25.2", Label("25.2"), testutil.YtsaurusImages25_2),
			Entry("YTsaurus twilight", Label("twilight"), testutil.TwilightImages),
			Entry("YTsaurus nightly", Label("nightly"), testutil.NightlyImages),
		) // integration tls

	}) // integration

	Context("update plan strategy testing", Label("update", "plan", "strategy"), func() {
		var podsBeforeUpdate map[string]corev1.Pod

		BeforeEach(func() {
			By("Adding base components")
			ytBuilder.WithBaseComponents()
		})

		JustBeforeEach(func(ctx context.Context) {
			By("Getting pods before actions")
			podsBeforeUpdate = getComponentPods(ctx, namespace)
		})

		DescribeTableSubtree("bulk strategy", Label("bulk"),
			func(componentType consts.ComponentType, stsName string) {
				BeforeEach(func() {

					switch componentType {
					case consts.QueryTrackerType:
						ytBuilder.WithQueryTracker()
						ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{
							InstanceSpec: ytv1.InstanceSpec{
								Image:         ptr.To(testutil.QueryTrackerImagePrevious),
								InstanceCount: 3,
							},
						}
					case consts.MasterType:
						ytsaurus.Spec.PrimaryMasters.InstanceCount = 3
					case consts.ControllerAgentType:
						ytsaurus.Spec.ControllerAgents.InstanceCount = 3
					case consts.DiscoveryType:
						ytsaurus.Spec.Discovery.InstanceCount = 3
					case consts.HttpProxyType:
						ytsaurus.Spec.HTTPProxies[0].InstanceCount = 3
					case consts.ExecNodeType:
						ytsaurus.Spec.ExecNodes[0].InstanceCount = 3
					}
					ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{
						{
							Component: ytv1.Component{Type: componentType},
							Strategy: &ytv1.ComponentUpdateStrategy{
								RunPreChecks: ptr.To(true),
							},
						},
					}
				})
				It("Should update "+stsName+" in bulkUpdate mode and have Running state", func(ctx context.Context) {

					By("Trigger " + stsName + " update")
					switch componentType {
					case consts.QueryTrackerType:
						ytsaurus.Spec.QueryTrackers.Image = ptr.To(testutil.QueryTrackerImageCurrent)
					default:
						updateSpecToTriggerAllComponentUpdate(ytsaurus)
					}

					UpdateObject(ctx, ytsaurus)
					EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())

					By("Waiting cluster update completes")
					EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())

					By("Fetching updated StatefulSet revision")
					sts := appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: stsName}}
					EventuallyObject(ctx, &sts, reactionTimeout).Should(WithTransform(
						func(current *appsv1.StatefulSet) string {
							return current.Status.UpdateRevision
						},
						Not(BeEmpty()),
					))
					updateRevision := sts.Status.UpdateRevision

					By("Waiting for pods to move to the new revision")
					Eventually(func() bool {
						podsNow := getComponentPods(ctx, namespace)
						for name, pod := range podsNow {
							if strings.HasPrefix(name, stsName+"-") {
								if pod.Labels[appsv1.StatefulSetRevisionLabel] != updateRevision {
									return false
								}
							}
						}
						return true
					}, upgradeTimeout, pollInterval).Should(BeTrue())

					checkClusterHealth(ctx, ytClient)
					checkChunkLocations(ytClient)
				})
			},
			Entry("update query tracker", Label(consts.GetStatefulSetPrefix(consts.QueryTrackerType)), consts.QueryTrackerType, consts.GetStatefulSetPrefix(consts.QueryTrackerType)),
			Entry("update master", Label(consts.GetStatefulSetPrefix(consts.MasterType)), consts.MasterType, consts.GetStatefulSetPrefix(consts.MasterType)),
			Entry("update controller-agent", Label(consts.GetStatefulSetPrefix(consts.ControllerAgentType)), consts.ControllerAgentType, consts.GetStatefulSetPrefix(consts.ControllerAgentType)),
			Entry("update discovery", Label(consts.GetStatefulSetPrefix(consts.DiscoveryType)), consts.DiscoveryType, consts.GetStatefulSetPrefix(consts.DiscoveryType)),
			Entry("update http-proxy", Label(consts.GetStatefulSetPrefix(consts.HttpProxyType)), consts.HttpProxyType, consts.GetStatefulSetPrefix(consts.HttpProxyType)),
			Entry("update end-node", Label(consts.GetStatefulSetPrefix(consts.ExecNodeType)), consts.ExecNodeType, consts.GetStatefulSetPrefix(consts.ExecNodeType)),
			Entry("update tnd-node", Label(consts.GetStatefulSetPrefix(consts.TabletNodeType)), consts.TabletNodeType, consts.GetStatefulSetPrefix(consts.TabletNodeType)),
			Entry("update dnd-node", Label(consts.GetStatefulSetPrefix(consts.DataNodeType)), consts.DataNodeType, consts.GetStatefulSetPrefix(consts.DataNodeType)),
		)

		DescribeTableSubtree("on-delete strategy", Label("ondelete"),
			func(componentType consts.ComponentType, stsName string) {
				BeforeEach(func() {
					switch componentType {
					case consts.SchedulerType:
						ytsaurus.Spec.Schedulers = &ytv1.SchedulersSpec{
							InstanceSpec: ytv1.InstanceSpec{
								Image:         ptr.To(testutil.YtsaurusImagePrevious),
								InstanceCount: 3,
							},
						}
					case consts.MasterType:
						ytsaurus.Spec.PrimaryMasters.InstanceCount = 3
					case consts.MasterCacheType:
						ytsaurus.Spec.MasterCaches.InstanceCount = 3
					case consts.RpcProxyType:
						ytBuilder.WithRPCProxies()
						ytsaurus.Spec.RPCProxies[0].InstanceCount = 3
					}
					ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{
						{
							Component: ytv1.Component{Type: componentType},
							Strategy: &ytv1.ComponentUpdateStrategy{
								OnDelete:     &ytv1.ComponentOnDeleteUpdateMode{},
								RunPreChecks: ptr.To(true),
							},
						},
					}
				})
				It("should update "+stsName+" with OnDelete strategy and have cluster Running state", func(ctx context.Context) {

					By("Trigger " + stsName + " update")
					switch componentType {
					case consts.SchedulerType:
						ytsaurus.Spec.Schedulers.Image = ptr.To(testutil.YtsaurusImageCurrent)
					default:
						updateSpecToTriggerAllComponentUpdate(ytsaurus)
					}

					UpdateObject(ctx, ytsaurus)
					EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())

					By("Verify OnDelete mode is activated")
					EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(
						HaveClusterUpdateState(ytv1.UpdateStateWaitingForPodsRemoval),
					)

					By("Verify StatefulSet has OnDelete update strategy")
					sts := appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: stsName}}
					EventuallyObject(ctx, &sts, reactionTimeout).Should(HaveField("Spec.UpdateStrategy.Type", appsv1.OnDeleteStatefulSetStrategyType))

					By("Verify StatefulSet has a new revision but pods stay on the old one")
					EventuallyObject(ctx, &sts, reactionTimeout).Should(WithTransform(
						func(current *appsv1.StatefulSet) bool {
							return current.Status.CurrentRevision != "" &&
								current.Status.UpdateRevision != "" &&
								current.Status.UpdateRevision != current.Status.CurrentRevision
						},
						BeTrue(),
					))

					oldRev := sts.Status.CurrentRevision
					Consistently(func() bool {
						podsNow := getComponentPods(ctx, namespace)
						for name, pod := range podsNow {
							if strings.HasPrefix(name, stsName+"-") {
								if pod.Labels[appsv1.StatefulSetRevisionLabel] != oldRev {
									return false
								}
							}
						}
						return true
					}, 10*time.Second).Should(BeTrue())

					By("Manually delete component pods")
					for name := range podsBeforeUpdate {
						if strings.HasPrefix(name, stsName+"-") {
							var pod corev1.Pod
							err := k8sClient.Get(ctx, types.NamespacedName{
								Namespace: namespace,
								Name:      name,
							}, &pod)
							if err == nil {
								Expect(k8sClient.Delete(ctx, &pod)).Should(Succeed())
							}
						}
					}

					By("Waiting cluster update completes")
					EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())

					By("Fetching updated StatefulSet revision")
					EventuallyObject(ctx, &sts, reactionTimeout).Should(WithTransform(
						func(current *appsv1.StatefulSet) string {
							return current.Status.UpdateRevision
						},
						Not(BeEmpty()),
					))
					updateRevision := sts.Status.UpdateRevision

					By("Waiting for pods to move to the new revision")
					Eventually(func() bool {
						podsNow := getComponentPods(ctx, namespace)
						for name, pod := range podsNow {
							if strings.HasPrefix(name, stsName+"-") {
								if pod.Labels[appsv1.StatefulSetRevisionLabel] != updateRevision {
									return false
								}
							}
						}
						return true
					}, upgradeTimeout, pollInterval).Should(BeTrue())
				})
			},
			Entry("update scheduler", Label(consts.GetStatefulSetPrefix(consts.SchedulerType)), consts.SchedulerType, consts.GetStatefulSetPrefix(consts.SchedulerType)),
			Entry("update master", Label(consts.GetStatefulSetPrefix(consts.MasterType)), consts.MasterType, consts.GetStatefulSetPrefix(consts.MasterType)),
			Entry("update master-caches", Label(consts.GetStatefulSetPrefix(consts.MasterCacheType)), consts.MasterCacheType, consts.GetStatefulSetPrefix(consts.MasterCacheType)),
			Entry("update rpc-proxy", Label(consts.GetStatefulSetPrefix(consts.RpcProxyType)), consts.RpcProxyType, consts.GetStatefulSetPrefix(consts.RpcProxyType)),
		)
	}) // update plan strategy

})

var _ = Describe("Spec version lock test", Label("version_lock"), Ordered, func() {
	var testNamespace string

	BeforeAll(func(ctx context.Context) {
		currentSpec := CurrentSpecReport()

		namespaceObject := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-e2e-",
				Labels: map[string]string{
					"app.kubernetes.io/component": "test",
					"app.kubernetes.io/name":      "test-" + strings.Join(currentSpec.Labels(), "-"),
					"app.kubernetes.io/part-of":   "ytsaurus-dev",
				},
				Annotations: map[string]string{
					"kubernetes.io/description": currentSpec.LeafNodeText,
				},
			},
		}

		Expect(k8sClient.Create(ctx, &namespaceObject)).Should(Succeed())

		testNamespace = namespaceObject.Name // Fetch unique namespace name
	})

	Context("Spec Version Constraint Lock", func() {
		const (
			operatorVersion = version.DevelVersion
		)

		var (
			builder  *testutil.YtsaurusBuilder
			ytsaurus *ytv1.Ytsaurus
		)

		newYtsaurus := func() *ytv1.Ytsaurus {
			builder = &testutil.YtsaurusBuilder{
				Images:    testutil.CurrentImages,
				Namespace: testNamespace,
			}

			builder.CreateMinimal()
			builder.WithBaseComponents()
			return builder.Ytsaurus
		}

		BeforeEach(func() {
			ytsaurus = newYtsaurus()
		})

		// A known string is required for tests to work reliably, other there is no way to
		// write proper assertions, at least not easily.
		It("Expect the tests to use a sentinel version string", func() {
			Expect(version.GetVersion()).To(Equal(operatorVersion))
		})

		DescribeTable("YT Spec version requirement works for all available operators",
			func(constraint string, errMsg string) {
				ytsaurus.Spec.RequiresOperatorVersion = constraint

				err := k8sClient.Create(context.Background(), ytsaurus)
				if errMsg != "" {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).Should(MatchRegexp(errMsg))
				} else {
					Expect(err).To(Not(HaveOccurred()))

					DeferCleanup(func() {
						err := k8sClient.Delete(context.Background(), ytsaurus)
						Expect(err).NotTo(HaveOccurred())
					})
				}
			},

			Entry("Handles empty version in spec", "", ""),
			Entry("Rejects invalid version constraint", "abcd", `spec.requiresOperatorVersion: Invalid value: "abcd": improper constraint: abcd`),
			Entry("Handles success case for exact version match", operatorVersion, ""),
			Entry("Handles failure case for exact version match", "= 2.0.0", "current operator version .* does not satisfy the spec version constraint: .*"),
			Entry("Handles success case for not equal", "!= 2.0.0", ""),
			Entry("Handles failure case for not equal", "!= "+operatorVersion, "current operator version .* does not satisfy the spec version constraint: .*"),
			Entry("Handles success case for greater version", "> 0.0.0-beta", ""),
			Entry("Handles failure case for greater version", "> 2.0.0", "current operator version .* does not satisfy the spec version constraint: .*"),
			Entry("Handles success case for lower version", "< 2.0.0", ""),
			Entry("Handles failure case for lower version", "< "+operatorVersion, "current operator version .* does not satisfy the spec version constraint: .*"),
			Entry("Handles success case for any patch version", "~"+operatorVersion, ""),
			Entry("Handles failure case for any patch version", "~2.0", "current operator version .* does not satisfy the spec version constraint: .*"),
		)
	})

	AfterAll(func(ctx context.Context) {
		namespaceObject := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		}

		Expect(k8sClient.Delete(ctx, &namespaceObject)).Should(Succeed())
	})
})

func checkClusterHealth(ctx context.Context, ytClient yt.Client) {
	By("Checking that cluster is alive")
	var res []string
	Expect(ytClient.ListNode(ctx, ypath.Path("/"), &res, nil)).Should(Succeed())

	By("Checking cluster alerts", func() {
		var clusterHealth ClusterHealthReport
		clusterHealth.Collect(ctx, ytClient)
		Expect(clusterHealth.Alerts).To(BeEmpty())
		Expect(clusterHealth.Errors).To(BeEmpty())
	})

	By("Checking that tablet cell bundles are in `good` health")
	Eventually(func() bool {
		notGoodBundles, err := components.GetNotGoodTabletCellBundles(ctx, ytClient)
		if err != nil {
			return false
		}
		return len(notGoodBundles) == 0
	}, reactionTimeout, pollInterval).Should(BeTrue())
}

func createTestUser(ytClient yt.Client) {
	By("Create a test user")
	Eventually(func() (bool, error) {
		_, err := ytClient.CreateObject(specCtx, yt.NodeUser, &yt.CreateObjectOptions{
			Attributes: map[string]any{"name": "test-user"},
		})
		// FIXME(khlebnikov): Access denied for user "admin": ...
		if yterrors.ContainsErrorCode(err, yterrors.CodeAuthorizationError) {
			log.Error(err, "CreateObject")
			return false, nil
		}
		return true, err
	}).Should(BeTrue())

	By("Check that test user cannot access things they shouldn't")
	hasPermission, err := ytClient.CheckPermission(specCtx, "test-user", yt.PermissionWrite, ypath.Path("//sys/groups/superusers"), nil)
	Expect(err).Should(Succeed())
	Expect(hasPermission.Action).Should(Equal(yt.ActionDeny))
}

func checkPodLabelCount(pods []corev1.Pod, labelKey string, labelValue string, expectedCount int) {
	podCount := 0
	for _, pod := range pods {
		if pod.Labels[labelKey] == labelValue {
			podCount++
		}
	}
	Expect(podCount).Should(Equal(expectedCount))
}

func checkPodLabels(ctx context.Context, namespace string) {
	cluster := testutil.YtsaurusName
	pods := getAllPods(ctx, namespace)

	for _, pod := range pods {
		log.Info("Pod", "name", pod.Name, "labels", pod.Labels)
	}

	Expect(pods).Should(ContainElement(And(
		HaveField("Name", "dnd-0"),
		HaveField("Labels", And(
			HaveKeyWithValue("app.kubernetes.io/name", "yt-data-node"),
			HaveKeyWithValue("app.kubernetes.io/component", "yt-data-node"),
			HaveKeyWithValue("yt_component", fmt.Sprintf("%v-yt-data-node", cluster)),
			HaveKeyWithValue("app.kubernetes.io/managed-by", "ytsaurus-k8s-operator"),
			HaveKeyWithValue("app.kubernetes.io/part-of", fmt.Sprintf("yt-%v", cluster)),
			HaveKeyWithValue("ytsaurus.tech/cluster-name", cluster),
		)),
	)))
	Expect(pods).Should(ContainElement(And(
		HaveField("Name", "dnd-dn-t-1"),
		HaveField("Labels", And(
			HaveKeyWithValue("app.kubernetes.io/name", "yt-data-node"),
			HaveKeyWithValue("app.kubernetes.io/component", "yt-data-node-dn-t"),
			HaveKeyWithValue("yt_component", fmt.Sprintf("%v-yt-data-node-dn-t", cluster)),
			HaveKeyWithValue("app.kubernetes.io/managed-by", "ytsaurus-k8s-operator"),
			HaveKeyWithValue("app.kubernetes.io/part-of", fmt.Sprintf("yt-%v", cluster)),
			HaveKeyWithValue("ytsaurus.tech/cluster-name", cluster),
		)),
	)))
	checkPodLabelCount(pods, "app.kubernetes.io/name", "yt-data-node", 6)
	checkPodLabelCount(pods, "app.kubernetes.io/component", "yt-data-node", 3)
	checkPodLabelCount(pods, "app.kubernetes.io/component", "yt-data-node-dn-t", 3)

	Expect(pods).Should(ContainElement(And(
		HaveField("Name", "ms-0"),
		HaveField("Labels", And(
			HaveKeyWithValue("app.kubernetes.io/name", "yt-master"),
			HaveKeyWithValue("app.kubernetes.io/component", "yt-master"),
			HaveKeyWithValue("yt_component", fmt.Sprintf("%v-yt-master", cluster)),
			HaveKeyWithValue("app.kubernetes.io/managed-by", "ytsaurus-k8s-operator"),
			HaveKeyWithValue("app.kubernetes.io/part-of", fmt.Sprintf("yt-%v", cluster)),
			HaveKeyWithValue("ytsaurus.tech/cluster-name", cluster),
		)),
	)))
	checkPodLabelCount(pods, "app.kubernetes.io/name", "yt-master", 1)

	// Init jobs have their own suffixes.
	checkPodLabelCount(pods, "app.kubernetes.io/name", "yt-scheduler", 1)
}

func checkChunkLocations(ytClient yt.Client) {
	// https://github.com/ytsaurus/ytsaurus-k8s-operator/issues/396
	// we expect enable_real_chunk_locations being set to true for all currently tested/supported versions.
	// FIXME(khlebnikov): Check master reigh.
	realChunkLocationPath := "//sys/@config/node_tracker/enable_real_chunk_locations"
	var realChunkLocationsValue bool
	err := ytClient.GetNode(specCtx, ypath.Path(realChunkLocationPath), &realChunkLocationsValue, nil)
	Expect(err).Should(Or(Succeed(), Satisfy(yterrors.ContainsResolveError)))
	if err == nil {
		Expect(realChunkLocationsValue).Should(BeTrue())
	}

	var values []yson.ValueWithAttrs
	Expect(ytClient.ListNode(specCtx, ypath.Path(consts.ComponentCypressPath(consts.DataNodeType)), &values, &yt.ListNodeOptions{
		Attributes: []string{"use_imaginary_chunk_locations"},
	})).Should(Succeed())

	for _, node := range values {
		Expect(node.Attrs["use_imaginary_chunk_locations"]).ShouldNot(BeTrue())
	}
}

type TestOperation struct {
	Client yt.Client
	Spec   *ytspec.Spec
	Id     yt.OperationID
}

func (o *TestOperation) Start() error {
	id, err := o.Client.StartOperation(specCtx, o.Spec.Type, o.Spec, nil)
	if err == nil {
		log.Info("Operation started", "id", id)
		o.Id = id
	}
	return err
}

func (o *TestOperation) Status(ctx context.Context) (*yt.OperationStatus, error) {
	return o.Client.GetOperation(ctx, o.Id, nil)
}

func (o *TestOperation) Abort() error {
	err := o.Client.AbortOperation(specCtx, o.Id, nil)
	if err == nil {
		log.Info("Operation aborted", "id", o.Id)
	}
	return err
}

func NewOprationStatusTracker() func(opStatus *yt.OperationStatus) bool {
	var prevState yt.OperationState
	var prevJobs yt.TotalJobCounter
	return func(opStatus *yt.OperationStatus) bool {
		if opStatus == nil {
			return false
		}
		changed := false
		jobs := ptr.Deref(opStatus.BriefProgress.TotalJobCounter, yt.TotalJobCounter{})
		if prevState != opStatus.State || !reflect.DeepEqual(&prevJobs, &jobs) {
			log.Info("Operation progress",
				"id", opStatus.ID,
				"state", opStatus.State,
				"running", jobs.Running,
				"completed", jobs.Completed,
				"total", jobs.Total,
			)
			prevState = opStatus.State
			prevJobs = jobs
			changed = true
		}
		return changed
	}
}

func (o *TestOperation) Wait() *yt.OperationStatus {
	var opStatus *yt.OperationStatus
	trackStatus := NewOprationStatusTracker()
	Eventually(func(ctx context.Context) bool {
		var err error
		opStatus, err = o.Status(ctx)
		Expect(err).NotTo(HaveOccurred())
		trackStatus(opStatus)
		if opStatus.State.IsFinished() {
			Expect(opStatus.State).Should(Equal(yt.StateCompleted))
			return true
		}
		return false
	}, operationTimeout, operationPollInterval, specCtx).Should(BeTrue())
	return opStatus
}

func (o *TestOperation) Run() *yt.OperationStatus {
	err := o.Start()
	Expect(err).NotTo(HaveOccurred())
	return o.Wait()
}

func NewVanillaOperation(ytClient yt.Client) *TestOperation {
	return &TestOperation{
		Client: ytClient,
		Spec: &ytspec.Spec{
			Type:  yt.OperationVanilla,
			Title: "e2e test operation",
			Tasks: map[string]*ytspec.UserScript{
				"test": {
					Command:     "true",
					CPULimit:    operationCPULimit,
					MemoryLimit: operationMemoryLimit,
					JobCount:    1,
				},
			},
			MaxFailedJobCount: 1,
		},
	}
}

func runAndCheckSortOperation(ytClient yt.Client) mapreduce.Operation {
	testTablePathIn := ypath.Path("//tmp/testexec")
	testTablePathOut := ypath.Path("//tmp/testexec-out")
	_, err := ytClient.CreateNode(
		specCtx,
		testTablePathIn,
		yt.NodeTable,
		nil,
	)
	Expect(err).Should(Succeed())
	_, err = ytClient.CreateNode(
		specCtx,
		testTablePathOut,
		yt.NodeTable,
		nil,
	)
	Expect(err).Should(Succeed())

	type Row struct {
		Key string `yson:"key"`
	}
	keys := []string{
		"xxx",
		"aaa",
		"bbb",
	}
	writer, err := ytClient.WriteTable(specCtx, testTablePathIn, nil)
	Expect(err).Should(Succeed())
	for _, key := range keys {
		err = writer.Write(Row{Key: key})
		Expect(err).Should(Succeed())
	}
	err = writer.Commit()
	Expect(err).Should(Succeed())

	mrCli := mapreduce.New(ytClient)
	op, err := mrCli.Sort(&ytspec.Spec{
		Type:            yt.OperationSort,
		InputTablePaths: []ypath.YPath{testTablePathIn},
		OutputTablePath: testTablePathOut,
		SortBy:          []string{"key"},
		TimeLimit:       yson.Duration(3 * time.Minute),
	})
	Expect(err).Should(Succeed())
	err = op.Wait()
	Expect(err).Should(Succeed())

	reader, err := ytClient.ReadTable(specCtx, testTablePathOut, nil)
	Expect(err).Should(Succeed())

	var rows []string
	for reader.Next() {
		row := Row{}
		err = reader.Scan(&row)
		Expect(err).Should(Succeed())
		rows = append(rows, row.Key)
	}

	sort.Strings(keys)
	Expect(rows).Should(Equal(keys))
	return op
}

func NewMapTestOperation(ytClient yt.Client) *TestOperation {
	testTablePathIn := ypath.Path("//tmp/testmap-in")
	testTablePathOut := ypath.Path("//tmp/testmap-out")
	_, err := ytClient.CreateNode(
		specCtx,
		testTablePathIn,
		yt.NodeTable,
		nil,
	)
	Expect(err).Should(Succeed())
	_, err = ytClient.CreateNode(
		specCtx,
		testTablePathOut,
		yt.NodeTable,
		nil,
	)
	Expect(err).Should(Succeed())

	type Row struct {
		Key string `yson:"key"`
	}
	keys := []string{
		"a",
		"b",
		"c",
	}
	writer, err := ytClient.WriteTable(specCtx, testTablePathIn, nil)
	Expect(err).Should(Succeed())
	for _, key := range keys {
		err = writer.Write(Row{Key: key})
		Expect(err).Should(Succeed())
	}
	err = writer.Commit()
	Expect(err).Should(Succeed())

	return &TestOperation{
		Client: ytClient,
		Spec: &ytspec.Spec{
			Type:             yt.OperationMap,
			Title:            "e2e test map operation",
			InputTablePaths:  []ypath.YPath{testTablePathIn},
			OutputTablePaths: []ypath.YPath{testTablePathOut},
			Mapper: &ytspec.UserScript{
				Command:     "cat",
				CPULimit:    operationCPULimit,
				MemoryLimit: operationMemoryLimit,
			},
			MaxFailedJobCount: 1,
		},
	}
}

var _ = Describe("Check operator", Label("operator"), func() {
	It("Tests operator metrics are available", Label("metrics"), func(ctx context.Context) {
		metricsURL, err := getOperatorMetricsURL(ctx)
		Expect(err).NotTo(HaveOccurred())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, http.NoBody)
		Expect(err).NotTo(HaveOccurred())
		rsp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(rsp.Body.Close)
		Expect(rsp.StatusCode).To(Equal(http.StatusOK))
		body, err := io.ReadAll(rsp.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(body).NotTo(BeEmpty())
	})
})
