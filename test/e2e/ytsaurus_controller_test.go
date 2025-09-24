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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	chytBootstrapTimeout = time.Minute * 2

	operationPollInterval = time.Millisecond * 250
	operationTimeout      = time.Second * 120
)

func getYtClient(proxyAddress string) yt.Client {
	ytClient, err := ythttp.NewClient(&yt.Config{
		Proxy:                 proxyAddress,
		Token:                 consts.DefaultAdminPassword,
		DisableProxyDiscovery: true,
	})
	Expect(err).Should(Succeed())

	return ytClient
}

func getYtRPCClient(proxyAddress, rpcProxyAddress string) yt.Client {
	ytClient, err := ytrpc.NewClient(&yt.Config{
		Proxy:                 proxyAddress,
		RPCProxy:              rpcProxyAddress,
		Token:                 consts.DefaultAdminPassword,
		DisableProxyDiscovery: true,
	})
	Expect(err).Should(Succeed())
	return ytClient
}

func getHTTPProxyAddress(g *ytconfig.Generator, namespace, portName string) (string, error) {
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

	return getServiceAddress(namespace, serviceName, portName)
}

func getRPCProxyAddress(g *ytconfig.Generator, namespace string) (string, error) {
	proxy := os.Getenv("E2E_YT_RPC_PROXY")
	if proxy != "" {
		return proxy, nil
	}

	serviceName := g.GetRPCProxiesServiceName(consts.DefaultName)

	portName := consts.YTRPCPortName
	if g.GetClusterFeatures().RPCProxyHavePublicAddress {
		portName = consts.YTPublicRPCPortName
	}

	return getServiceAddress(namespace, serviceName, portName)
}

func getMasterPod(name, namespace string) corev1.Pod {
	msPod := corev1.Pod{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &msPod)
	Expect(err).Should(Succeed())
	return msPod
}

func runImpossibleUpdateAndRollback(ytsaurus *ytv1.Ytsaurus, ytClient yt.Client) {
	By("Run cluster impossible update")
	Expect(ytsaurus.Spec.CoreImage).To(Equal(testutil.CurrentImages.Core))
	ytsaurus.Spec.CoreImage = testutil.FutureImages.Core
	UpdateObject(ctx, ytsaurus)

	EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveClusterUpdateState(ytv1.UpdateStateImpossibleToStart))

	By("Set previous core image")
	ytsaurus.Spec.CoreImage = testutil.CurrentImages.Core
	UpdateObject(ctx, ytsaurus)

	By("Wait for running")
	EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())

	By("Check that cluster alive after update")
	res := make([]string, 0)
	Expect(ytClient.ListNode(ctx, ypath.Path("/"), &res, nil)).Should(Succeed())
}

type testRow struct {
	A string `yson:"a"`
}

var _ = Describe("Basic e2e test for Ytsaurus controller", Label("e2e"), func() {
	var namespace string
	var requiredImages []string
	var objects []client.Object
	var namespaceWatcher *NamespaceWatcher
	var name client.ObjectKey
	var ytBuilder *testutil.YtsaurusBuilder
	var ytsaurus *ytv1.Ytsaurus
	var chyt *ytv1.Chyt
	var ytProxyAddress string
	var ytClient yt.Client
	var generator *ytconfig.Generator
	var certBuilder *testutil.CertBuilder
	var clusterWithTLS bool
	var caBundleCertificates []byte
	var caBundleCertPool *x509.CertPool
	var ytHTTPSProxyAddress string
	var ytHTTPSClient yt.Client
	var ytRPCProxyAddress string
	var ytRPCClient yt.Client
	var ytRPCTLSClient yt.Client
	var remoteComponentNames map[consts.ComponentType][]string

	// NOTE: execution order for each test spec:
	// - BeforeEach               (configuration)
	// - JustBeforeEach           (creation, validation)
	// - It                       (test itself)
	// - JustAfterEach            (diagnosis, validation)
	// - AfterEach, DeferCleanup  (cleanup)
	//
	// See:
	// https://onsi.github.io/ginkgo/#separating-creation-and-configuration-justbeforeeach
	// https://onsi.github.io/ginkgo/#spec-cleanup-aftereach-and-defercleanup
	// https://onsi.github.io/ginkgo/#separating-diagnostics-collection-and-teardown-justaftereach

	withHTTPSProxy := func() {
		By("Adding HTTPS proxy TLS certificates")
		clusterWithTLS = true

		httpProxyCert := certBuilder.BuildCertificate(ytsaurus.Name+"-http-proxy", []string{
			generator.GetHTTPProxiesServiceAddress(""),
			generator.GetComponentLabeller(consts.HttpProxyType, "").GetInstanceAddressWildcard(),
		})
		objects = append(objects, httpProxyCert)

		ytsaurus.Spec.HTTPProxies[0].Transport = ytv1.HTTPTransportSpec{
			HTTPSSecret: &corev1.LocalObjectReference{
				Name: httpProxyCert.Name,
			},
		}
	}

	withRPCTLSProxy := func() {
		By("Adding RPC proxy TLS certificates")
		rpcProxyCert := certBuilder.BuildCertificate(ytsaurus.Name+"-rpc-proxy", []string{
			generator.GetComponentLabeller(consts.RpcProxyType, "").GetInstanceAddressWildcard(),
		})
		objects = append(objects, rpcProxyCert)

		ytsaurus.Spec.RPCProxies[0].Transport = ytv1.RPCTransportSpec{
			TLSSecret: &corev1.LocalObjectReference{
				Name: rpcProxyCert.Name,
			},
		}
	}

	BeforeEach(func() {
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
		DeferCleanup(LogObjectEvents(ctx, namespace))

		By("Logging some other objects in namespace")
		namespaceWatcher = NewNamespaceWatcher(ctx, namespace)
		namespaceWatcher.Start()
		DeferCleanup(namespaceWatcher.Stop)

		By("Creating minimal Ytsaurus spec")
		ytBuilder = &testutil.YtsaurusBuilder{
			Images:    testutil.CurrentImages,
			Namespace: namespace,
		}
		ytBuilder.CreateMinimal()
		ytsaurus = ytBuilder.Ytsaurus

		requiredImages = nil
		objects = []client.Object{ytsaurus}

		name = client.ObjectKey{
			Name:      ytsaurus.Name,
			Namespace: namespace,
		}

		generator = ytconfig.NewGenerator(ytsaurus, "cluster.local")
		remoteComponentNames = make(map[consts.ComponentType][]string)

		certBuilder = &testutil.CertBuilder{
			Namespace:   namespace,
			IPAddresses: getNodesAddresses(),
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

		DeferCleanup(AttachProgressReporter(func() string {
			ytProxyAddress, err := getHTTPProxyAddress(generator, namespace, consts.HTTPPortName)
			if err != nil {
				return fmt.Sprintf("cannot find proxy address: %v", err)
			}
			ytClient := getYtClient(ytProxyAddress)
			if _, err := ytClient.WhoAmI(ctx, nil); err != nil {
				return fmt.Sprintf("ytsaurus api error: %v", err)
			}
			var clusterHealth ClusterHealthReport
			clusterHealth.Collect(ytClient)
			return clusterHealth.String()
		}))

		DeferCleanup(AttachProgressReporter(func() string {
			failedPods := fetchFailedPods(namespace)
			if len(failedPods) != 0 {
				return fmt.Sprintf("Failed pods: %v", failedPods)
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

		By("Creating ytsaurus client")
		var err error
		ytProxyAddress, err = getHTTPProxyAddress(generator, namespace, consts.HTTPPortName)
		Expect(err).ToNot(HaveOccurred())

		log.Info("Ytsaurus access",
			"YT_PROXY", ytProxyAddress,
			"YT_TOKEN", consts.DefaultAdminPassword,
			"login", consts.DefaultAdminLogin,
			"password", consts.DefaultAdminPassword,
		)

		DeferCleanup(AttachProgressReporter(func() string {
			return fmt.Sprintf("YT_PROXY=%s YT_TOKEN=%s", ytProxyAddress, consts.DefaultAdminPassword)
		}))

		ytClient = getYtClient(ytProxyAddress)
		Expect(ytClient.WhoAmI(ctx, nil)).To(HaveField("Login", consts.DefaultAdminLogin))

		checkClusterHealth(ytClient)

		createTestUser(ytClient)
	})

	JustBeforeEach(func(ctx context.Context) {
		if !clusterWithTLS {
			return
		}

		var err error

		By("Fetching CA bundle certificates")
		caBundleCertificates, err = readFileObject(namespace, ytv1.FileObjectReference{
			Name: testutil.TestTrustBundleName,
			Key:  consts.CABundleFileName,
		})
		Expect(err).To(Succeed())
		Expect(caBundleCertificates).ToNot(BeEmpty())
		caBundleCertPool = x509.NewCertPool()
		Expect(caBundleCertPool.AppendCertsFromPEM(caBundleCertificates)).To(BeTrue())

		ytHTTPSProxyAddress, err = getHTTPProxyAddress(generator, namespace, consts.HTTPSPortName)
		Expect(err).ToNot(HaveOccurred())
		ytHTTPSProxyAddress = "https://" + ytHTTPSProxyAddress

		By("Checking YT Proxy HTTPS", func() {
			httpClient := http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs: caBundleCertPool,
					},
				},
			}
			resp, err := httpClient.Get(ytHTTPSProxyAddress + "/api")
			Expect(err).NotTo(HaveOccurred())
			defer func() {
				Expect(resp.Body.Close()).To(Succeed())
			}()

			bodyBytes, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
			Expect(string(bodyBytes)).To(Equal(`["v3","v4"]`))
		})

		By("Checking YT HTTPS Client")
		ytHTTPSClient, err = ythttp.NewClient(&yt.Config{
			Proxy:                    ytHTTPSProxyAddress,
			CertificateAuthorityData: caBundleCertificates,
			Token:                    consts.DefaultAdminPassword,
			DisableProxyDiscovery:    true,
		})
		Expect(err).Should(Succeed())
		Expect(ytHTTPSClient.WhoAmI(ctx, nil)).To(HaveField("Login", consts.DefaultAdminLogin))

		_, err = ytHTTPSClient.NodeExists(ctx, ypath.Path("/"), nil)
		Expect(err).Should(Succeed())
	})

	JustBeforeEach(func(ctx context.Context) {
		if len(ytsaurus.Spec.RPCProxies) == 0 {
			return
		}

		var err error

		// TODO(khlebnikov): Generalize for all components in cluster health report.
		By("Checking RPC proxies are registered")
		var rpcProxies []string
		Expect(ytClient.ListNode(ctx, ypath.Path("//sys/rpc_proxies"), &rpcProxies, nil)).Should(Succeed())
		Expect(rpcProxies).Should(HaveLen(int(ytsaurus.Spec.RPCProxies[0].InstanceCount)))

		By("Checking YT RPC Proxy discovery")
		proxies := discoverProxies("http://"+ytProxyAddress, nil)
		Expect(proxies).ToNot(BeEmpty())
		Expect(proxies).To(HaveEach(HaveSuffix(fmt.Sprintf(":%v", consts.RPCProxyPublicRPCPort))))

		By("Creating ytsaurus RPC client")
		ytRPCProxyAddress, err = getRPCProxyAddress(generator, namespace)
		Expect(err).ToNot(HaveOccurred())
		ytRPCClient = getYtRPCClient(ytProxyAddress, ytRPCProxyAddress)

		By("Checking RPC proxy is working")
		// Expect(ytTLSRPCClient.WhoAmI(ctx, nil)).To(HaveField("Login", consts.DefaultAdminLogin))
		_, err = ytRPCClient.NodeExists(ctx, ypath.Path("/"), nil)
		Expect(err).Should(Succeed())
	})

	JustBeforeEach(func(ctx context.Context) {
		if !clusterWithTLS || len(ytsaurus.Spec.RPCProxies) == 0 {
			return
		}

		var err error

		By("Checking YT RPC TLS Client")
		ytRPCTLSClient, err = ytrpc.NewClient(&yt.Config{
			Proxy:                    ytHTTPSProxyAddress,
			RPCProxy:                 ytRPCProxyAddress,
			CertificateAuthorityData: caBundleCertificates,
			Token:                    consts.DefaultAdminPassword,
			DisableProxyDiscovery:    true,
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
		Eventually(queryClickHouse, chytBootstrapTimeout, pollInterval).WithArguments(
			ytProxyAddress, "SELECT 1",
		).MustPassRepeatedly(3).Should(Equal("1\n"))
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

		By("Deleting namespace")
		namespaceObject := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		Expect(k8sClient.Delete(ctx, &namespaceObject)).Should(Succeed())
	})

	Context("Update scenarios", Label("update"), func() {
		var podsBeforeUpdate map[string]corev1.Pod

		BeforeEach(func() {
			By("Adding base components")
			ytBuilder.WithBaseComponents()
		})

		JustBeforeEach(func() {
			By("Getting pods before actions")
			podsBeforeUpdate = getComponentPods(ctx, namespace)
		})

		DescribeTableSubtree("Updating Ytsaurus image",
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
					checkClusterHealth(ytClient)
					checkChunkLocations(ytClient)

					podsAfterFullUpdate := getComponentPods(ctx, namespace)
					pods := getChangedPods(podsBeforeUpdate, podsAfterFullUpdate)
					Expect(pods.Created).To(BeEmpty(), "created")
					Expect(pods.Deleted).To(BeEmpty(), "deleted")
					Expect(pods.Updated).To(ConsistOf(maps.Keys(podsBeforeUpdate)), "updated")

					CurrentlyObject(ctx, ytsaurus).Should(HaveObservedGeneration())
				})
			},
			Entry("When update Ytsaurus 23.2 -> 24.1", Label("basic"), testutil.YtsaurusImage23_2, testutil.YtsaurusImage24_1),
			Entry("When update Ytsaurus 24.1 -> 24.2", Label("basic"), testutil.YtsaurusImage24_1, testutil.YtsaurusImage24_2),
			Entry("When update Ytsaurus 24.2 -> 25.1", Label("basic"), testutil.YtsaurusImage24_2, testutil.YtsaurusImage25_1),
			Entry("When update Ytsaurus 25.1 -> 25.2", Label("basic"), testutil.YtsaurusImage25_1, testutil.YtsaurusImage25_2),
		)

		Context("Test UpdateSelector", Label("selector"), func() {

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

				By("Ensure cluster doesn't start updating for 5 seconds")
				ConsistentlyYtsaurus(ctx, name, 5*time.Second).Should(HaveClusterStateRunning())

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

				checkClusterHealth(ytClient)

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

				checkClusterHealth(ytClient)

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

				checkClusterHealth(ytClient)

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

				checkClusterHealth(ytClient)
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

				checkClusterHealth(ytClient)

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
			It("Master shouldn't be recreated before WaitingForPodsCreation state if config changes", func(ctx context.Context) {

				By("Store master pod creation time")
				msPod := getMasterPod(testutil.MasterPodName, namespace)
				msPodCreationFirstTimestamp := msPod.CreationTimestamp

				By("Setting artificial conditions for deploy to stuck in PossibilityCheck")
				var masters []string
				err := ytClient.ListNode(ctx, ypath.Path("//sys/primary_masters"), &masters, nil)
				Expect(err).Should(Succeed())
				Expect(masters).Should(HaveLen(1))
				onlyMaster := masters[0]
				_, err = ytClient.CopyNode(
					ctx,
					ypath.Path("//sys/primary_masters/"+onlyMaster),
					ypath.Path("//sys/primary_masters/"+onlyMaster+"-fake-leader"),
					nil,
				)
				Expect(err).Should(Succeed())

				By("Run cluster update")
				ytsaurus.Spec.HostNetwork = true
				ytsaurus.Spec.PrimaryMasters.HostAddresses = []string{
					getKindControlPlaneNode().Name,
				}
				UpdateObject(ctx, ytsaurus)

				By("Waiting PossibilityCheck")
				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveClusterUpdateState(ytv1.UpdateStatePossibilityCheck))

				By("Check that master pod was NOT recreated at the PossibilityCheck stage")
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

		JustBeforeEach(func() {
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

		Context("With CRI job environment", Label("cri"), func() {

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

		Context("With CRI-O", Label("cri"), Label("crio"), func() {

			BeforeEach(func() {
				if os.Getenv("YTSAURUS_CRIO_READY") == "" {
					Skip("YTsaurus is not ready for CRI-O")
				}

				By("Adding exec nodes")
				ytBuilder.WithScheduler()
				ytBuilder.WithControllerAgents()
				ytBuilder.WithExecNodes()
				ytBuilder.WithDataNodes()

				By("Adding CRI-O job environment")
				ytBuilder.CRIService = ptr.To(ytv1.CRIServiceCRIO)
				ytBuilder.WithCRIJobEnvironment()
			})

			It("Verify CRI-O job environment", func(ctx context.Context) {
				// TODO(khlebnikov): Check docker image and resource limits.
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
				})
				Expect(err).Should(Succeed())

				_, err = cli.NodeExists(ctx, ypath.Path("/"), nil)
				Expect(yterrors.ContainsErrorCode(err, yterrors.CodeRPCAuthenticationError)).Should(BeTrue())

				By("Checking RPC proxy works with token")
				cli = getYtRPCClient(ytProxyAddress, ytRPCProxyAddress)
				// Expect(cli.WhoAmI(ctx, nil)).To(HaveField("Login", consts.DefaultAdminLogin))

				_, err = cli.NodeExists(ctx, ypath.Path("/"), nil)
				Expect(err).Should(Succeed())
			})

		}) // integration rpc-proxy

		Context("With host network", Label("host-network"), func() {

			BeforeEach(func() {
				ytsaurus.Spec.HostNetwork = true
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
				chyt = ytBuilder.CreateChyt()
				objects = append(objects, chyt)
			})

			It("Checks ClickHouse", func(ctx context.Context) {
				By("Creating table")
				Expect(queryClickHouse(
					ytProxyAddress,
					"CREATE TABLE `//tmp/chyt_test` ENGINE = YtTable() AS SELECT * FROM system.one",
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

				ytsaurus.Spec.CABundle = &ytv1.FileObjectReference{
					Name: testutil.TestTrustBundleName,
				}

				ytsaurus.Spec.NativeTransport = &ytv1.RPCTransportSpec{
					TLSSecret: &corev1.LocalObjectReference{
						Name: nativeServerCert.Name,
					},
					TLSRequired:                true,
					TLSPeerAlternativeHostName: ytsaurus.Name,
				}

				if images.MutualTLSReady {
					By("Enabling RPC proxy public address")
					ytsaurus.Spec.ClusterFeatures = &ytv1.ClusterFeatures{
						RPCProxyHavePublicAddress: true,
					}

					By("Activating mutual TLS interconnect")
					ytsaurus.Spec.NativeTransport.TLSInsecure = false
					ytsaurus.Spec.NativeTransport.TLSClientSecret = &corev1.LocalObjectReference{
						Name: nativeClientCert.Name,
					}
				} else {
					By("Activating TLS-only interconnect")
					ytsaurus.Spec.NativeTransport.TLSInsecure = true
				}

				By("Adding all components")
				ytsaurus.Spec.CoreImage = images.Core
				ytBuilder.Images = images
				ytBuilder.WithBaseComponents()
				ytBuilder.WithRPCProxies()
				ytBuilder.WithQueryTracker()
				ytBuilder.WithYqlAgent()
				ytBuilder.WithStrawberryController()

				withHTTPSProxy()
				withRPCTLSProxy()

				By("Adding CHYT instance")
				chyt = ytBuilder.CreateChyt()
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
				clickHouseID, err := queryClickHouseID(ytProxyAddress)
				Expect(err).To(Succeed())

				By("Creating table //tmp/chyt_test")
				Expect(queryClickHouse(
					ytProxyAddress,
					"CREATE TABLE `//tmp/chyt_test` ENGINE = YtTable() AS SELECT * FROM system.one;",
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

				// FIXME(khlebnikov): Workaround for bug yt client retrying logic.
				By("Waiting http proxy", func() {
					ytClient.Stop()
					ytClient = getYtClient(ytProxyAddress)
					Eventually(func(ctx context.Context) error {
						_, err := ytClient.WhoAmI(ctx, nil)
						return err
					}, bootstrapTimeout, pollInterval, ctx).MustPassRepeatedly(5).Should(Succeed())
					ytClient.Stop()
					ytClient = getYtClient(ytProxyAddress)

					// FIXME(khlebnikov): Workaround for DNS issues inside cluster.
					components, err := ListClusterComponents(ctx, ytClient)
					Expect(err).To(Succeed())
					for _, component := range components {
						Eventually(func(ctx context.Context) error {
							_, err := component.GetBusService(ctx)
							return err
						}, bootstrapTimeout, pollInterval, ctx).Should(Succeed())
					}
				})

				if images.StrawberryHandlesRestarts {
					By("Waiting for chyt operation restart by strawberry")
					Eventually(queryClickHouseID, chytBootstrapTimeout).WithArguments(ytProxyAddress).ToNot(Equal(clickHouseID))
				} else {
					By("Aborting chyt operation")
					Expect(ytClient.AbortOperation(ctx, yt.OperationID(clickHouseID), nil)).To(Succeed())
				}

				By("Waiting CHYT readiness")
				Eventually(queryClickHouse, chytBootstrapTimeout, pollInterval).WithArguments(
					ytProxyAddress, "SELECT 1",
				).MustPassRepeatedly(3).Should(Equal("1\n"))

				By("Checking table //tmp/chyt_test")
				Expect(queryClickHouse(
					ytProxyAddress,
					"SELECT * FROM `//tmp/chyt_test`;",
				)).To(Equal("0\n"))
			})
		},
			Entry("YTsaurus 24.2", Label("24.2"), testutil.YtsaurusImages24_2),
			Entry("YTsaurus 25.1", Label("25.1"), testutil.YtsaurusImages25_1),
			Entry("YTsaurus 25.2", Label("25.2"), testutil.YtsaurusImages25_2),
			Entry("YTsaurus twilight", Label("twilight"), testutil.TwilightImages),
			Entry("YTsaurus nightly", Label("nightly"), testutil.NightlyImages),
		) // integration tls

	}) // integration
})

func checkClusterHealth(ytClient yt.Client) {
	By("Checking that cluster is alive")
	var res []string
	Expect(ytClient.ListNode(ctx, ypath.Path("/"), &res, nil)).Should(Succeed())

	By("Checking cluster alerts", func() {
		var clusterHealth ClusterHealthReport
		clusterHealth.Collect(ytClient)
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
	_, err := ytClient.CreateObject(ctx, yt.NodeUser, &yt.CreateObjectOptions{Attributes: map[string]any{"name": "test-user"}})
	Expect(err).Should(Succeed())

	By("Check that test user cannot access things they shouldn't")
	hasPermission, err := ytClient.CheckPermission(ctx, "test-user", yt.PermissionWrite, ypath.Path("//sys/groups/superusers"), nil)
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
	err := ytClient.GetNode(ctx, ypath.Path(realChunkLocationPath), &realChunkLocationsValue, nil)
	Expect(err).Should(Or(Succeed(), Satisfy(yterrors.ContainsResolveError)))
	if err == nil {
		Expect(realChunkLocationsValue).Should(BeTrue())
	}

	var values []yson.ValueWithAttrs
	Expect(ytClient.ListNode(ctx, ypath.Path("//sys/data_nodes"), &values, &yt.ListNodeOptions{
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
	id, err := o.Client.StartOperation(ctx, o.Spec.Type, o.Spec, nil)
	if err == nil {
		log.Info("Operation started", "id", id)
		o.Id = id
	}
	return err
}

func (o *TestOperation) Status() (*yt.OperationStatus, error) {
	return o.Client.GetOperation(ctx, o.Id, nil)
}

func (o *TestOperation) Abort() error {
	err := o.Client.AbortOperation(ctx, o.Id, nil)
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
		opStatus, err = o.Status()
		Expect(err).NotTo(HaveOccurred())
		trackStatus(opStatus)
		if opStatus.State.IsFinished() {
			Expect(opStatus.State).Should(Equal(yt.StateCompleted))
			return true
		}
		return false
	}, operationTimeout, operationPollInterval, ctx).Should(BeTrue())
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
					Command:  "true",
					CPULimit: 0,
					JobCount: 1,
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
		ctx,
		testTablePathIn,
		yt.NodeTable,
		nil,
	)
	Expect(err).Should(Succeed())
	_, err = ytClient.CreateNode(
		ctx,
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
	writer, err := ytClient.WriteTable(ctx, testTablePathIn, nil)
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

	reader, err := ytClient.ReadTable(ctx, testTablePathOut, nil)
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
		ctx,
		testTablePathIn,
		yt.NodeTable,
		nil,
	)
	Expect(err).Should(Succeed())
	_, err = ytClient.CreateNode(
		ctx,
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
	writer, err := ytClient.WriteTable(ctx, testTablePathIn, nil)
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
				Command:  "cat",
				CPULimit: 0,
			},
			MaxFailedJobCount: 1,
		},
	}
}
