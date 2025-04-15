package controllers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"go.ytsaurus.tech/yt/go/mapreduce"
	"go.ytsaurus.tech/yt/go/mapreduce/spec"
	"go.ytsaurus.tech/yt/go/schema"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	"go.ytsaurus.tech/yt/go/yt/ytrpc"
	"go.ytsaurus.tech/yt/go/yterrors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

const (
	pollInterval     = time.Millisecond * 250
	reactionTimeout  = time.Second * 150
	bootstrapTimeout = time.Minute * 3
	upgradeTimeout   = time.Minute * 10
)

var getYtClient = getYtHTTPClient

func getYtHTTPClient(g *ytconfig.Generator, namespace string) yt.Client {
	ytClient, err := ythttp.NewClient(&yt.Config{
		Proxy:                 getHTTPProxyAddress(g, namespace),
		Token:                 consts.DefaultAdminPassword,
		DisableProxyDiscovery: true,
	})
	Expect(err).Should(Succeed())

	return ytClient
}

func getYtRPCClient(g *ytconfig.Generator, namespace string) yt.Client {
	ytClient, err := ytrpc.NewClient(&yt.Config{
		Proxy:                 getHTTPProxyAddress(g, namespace),
		RPCProxy:              getRPCProxyAddress(g, namespace),
		Token:                 consts.DefaultAdminPassword,
		DisableProxyDiscovery: true,
	})
	Expect(err).Should(Succeed())
	return ytClient
}

func getHTTPProxyAddress(g *ytconfig.Generator, namespace string) string {
	proxy := os.Getenv("E2E_YT_HTTP_PROXY")
	if proxy != "" {
		return proxy
	}
	// This one is used in real code in YtsaurusClient.
	proxy = os.Getenv("YTOP_PROXY")
	if proxy != "" {
		return proxy
	}

	if os.Getenv("E2E_YT_PROXY") != "" {
		panic("E2E_YT_PROXY is deprecated, use E2E_YT_HTTP_PROXY")
	}

	return getServiceAddress(g.GetHTTPProxiesServiceName(consts.DefaultHTTPProxyRole), namespace)
}

func getRPCProxyAddress(g *ytconfig.Generator, namespace string) string {
	proxy := os.Getenv("E2E_YT_RPC_PROXY")
	if proxy != "" {
		return proxy
	}

	return getServiceAddress(g.GetRPCProxiesServiceName(consts.DefaultName), namespace)
}

func getServiceAddress(svcName string, namespace string) string {
	svc := corev1.Service{}
	Expect(k8sClient.Get(ctx,
		client.ObjectKey{Name: svcName, Namespace: namespace},
		&svc),
	).Should(Succeed())

	k8sNode := corev1.Node{}
	Expect(k8sClient.Get(ctx,
		client.ObjectKey{Name: getKindControlPlaneName(), Namespace: namespace},
		&k8sNode),
	).Should(Succeed())

	Expect(svc.Spec.Type).To(Equal(corev1.ServiceTypeNodePort))
	Expect(svc.Spec.IPFamilies[0]).To(Equal(corev1.IPv4Protocol))
	nodePort := svc.Spec.Ports[0].NodePort

	nodeAddress := ""
	for _, address := range k8sNode.Status.Addresses {
		if address.Type == corev1.NodeInternalIP && net.ParseIP(address.Address).To4() != nil {
			nodeAddress = address.Address
			break
		}
	}
	Expect(nodeAddress).ToNot(BeEmpty())
	return fmt.Sprintf("%s:%v", nodeAddress, nodePort)
}

func getKindControlPlaneName() string {
	kindClusterName := os.Getenv("KIND_CLUSTER_NAME")
	if kindClusterName == "" {
		kindClusterName = "kind"
	}
	return kindClusterName + "-control-plane"
}

func getMasterPod(name, namespace string) corev1.Pod {
	msPod := corev1.Pod{}
	err := k8sClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &msPod)
	Expect(err).Should(Succeed())
	return msPod
}

func runImpossibleUpdateAndRollback(ytsaurus *ytv1.Ytsaurus, ytClient yt.Client) {
	By("Run cluster impossible update")
	ytsaurus.Spec.CoreImage = testutil.CoreImageSecond
	UpdateObject(ctx, ytsaurus)

	EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveClusterUpdateState(ytv1.UpdateStateImpossibleToStart))

	By("Set previous core image")
	ytsaurus.Spec.CoreImage = testutil.CoreImageFirst
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
	var objects []client.Object
	var namespaceWatcher *NamespaceWatcher
	var name client.ObjectKey
	var ytsaurus *ytv1.Ytsaurus
	var ytClient yt.Client
	var g *ytconfig.Generator

	// NOTE: execution order for each test spec:
	// - BeforeEach 	(configuration)
	// - JustBeforeEach 	(creation)
	// - It			(test itself)
	// - JustAfterEach	(diagnosis)
	// - AfterEach, DeferCleanup	(cleanup)
	//
	// See:
	// https://onsi.github.io/ginkgo/#separating-creation-and-configuration-justbeforeeach
	// https://onsi.github.io/ginkgo/#spec-cleanup-aftereach-and-defercleanup
	// https://onsi.github.io/ginkgo/#separating-diagnostics-collection-and-teardown-justaftereach

	BeforeEach(func() {
		By("Creating namespace")
		currentSpec := CurrentSpecReport()
		namespaceObject := corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "test-e2e-",
				Labels: map[string]string{
					"app.kubernetes.io/component": "test",
					"app.kubernetes.io/name":      "test-" + strings.Join(currentSpec.Labels(), "-"),
				},
				Annotations: map[string]string{
					"kubernetes.io/description": currentSpec.LeafNodeText,
				},
			},
		}
		Expect(k8sClient.Create(ctx, &namespaceObject)).Should(Succeed())
		namespace = namespaceObject.Name // Fetch unique namespace name
		namespaceWatcher = NewNamespaceWatcher(ctx, namespace)
		namespaceWatcher.Start()

		By("Creating minimal Ytsaurus spec")
		ytsaurus = testutil.CreateMinimalYtsaurusResource(namespace)
		objects = []client.Object{ytsaurus}

		name = client.ObjectKey{
			Name:      ytsaurus.Name,
			Namespace: namespace,
		}

		By("Logging all events in namespace")
		logEventsCleanup := LogObjectEvents(ctx, namespace)
		DeferCleanup(func() {
			logEventsCleanup()
			namespaceWatcher.Stop()
		})
	})

	JustBeforeEach(func(ctx context.Context) {
		By("Creating resource objects")
		for _, object := range objects {
			By(fmt.Sprintf("Creating %v %v", GetObjectGVK(object), object.GetName()))
			object.SetNamespace(namespace)
			Expect(k8sClient.Create(ctx, object)).Should(Succeed())
		}

		By("Checking that Ytsaurus state is equal to `Running`")
		EventuallyYtsaurus(ctx, ytsaurus, bootstrapTimeout).Should(HaveClusterStateRunning())

		g = ytconfig.NewGenerator(ytsaurus, "local")

		ytClient = createYtsaurusClient(ytsaurus, namespace)
		checkClusterViability(ytClient)
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
			testutil.WithBaseYtsaurusComponents(ytsaurus)
		})

		JustBeforeEach(func() {
			By("Getting pods before actions")
			podsBeforeUpdate = getComponentPods(ctx, namespace)
		})

		DescribeTableSubtree("Updating Ytsaurus image",
			func(newImage string) {
				BeforeEach(func() {
					testutil.WithNamedDataNodes(ytsaurus, ptr.To("dn-t"))
				})

				It("Triggers cluster update", func(ctx context.Context) {
					By("Checking jobs order")
					completedJobs := namespaceWatcher.GetCompletedJobNames()
					Expect(completedJobs).Should(Equal(getInitializingStageJobNames()))

					checkPodLabels(ctx, namespace)

					ytsaurus.Spec.CoreImage = newImage
					UpdateObject(ctx, ytsaurus)

					EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())

					By("Waiting cluster update completes")
					EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
					checkClusterBaseViability(ytClient)
					checkChunkLocations(ytClient)

					podsAfterFullUpdate := getComponentPods(ctx, namespace)
					pods := getChangedPods(podsBeforeUpdate, podsAfterFullUpdate)
					Expect(pods.Created).To(BeEmpty(), "created")
					Expect(pods.Deleted).To(BeEmpty(), "deleted")
					Expect(pods.Updated).To(ConsistOf(maps.Keys(podsBeforeUpdate)), "updated")

					CurrentlyObject(ctx, ytsaurus).Should(HaveObservedGeneration())
				})
			},
			Entry("When update Ytsaurus 23.2 -> 24.1", Label("basic"), testutil.CoreImageSecond),
			Entry("When update Ytsaurus 24.1 -> 24.2", Label("basic"), testutil.CoreImageNextVer),
		)

		Context("Test UpdateSelector", Label("selector"), func() {

			BeforeEach(func() {
				By("Setting 24.2+ image for update selector tests")
				// This image is used for update selector tests because for earlier images
				// there will be migration of imaginary chunks locations which restarts datanodes
				// and makes it hard to test updateSelector.
				// For 24.2+ image no migration is needed.
				ytsaurus.Spec.CoreImage = testutil.CoreImageNextVer
			})

			It("Should be updated according to UpdateSelector=Everything", func(ctx context.Context) {

				By("Run cluster update with selector: nothing")
				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassNothing}}
				updateSpecToTriggerAllComponentUpdate(ytsaurus)
				UpdateObject(ctx, ytsaurus)

				By("Ensure cluster doesn't start updating for 5 seconds")
				ConsistentlyYtsaurus(ctx, name, 5*time.Second).Should(HaveClusterStateRunning())
				podsAfterBlockedUpdate := getComponentPods(ctx, namespace)
				Expect(podsBeforeUpdate).To(
					Equal(podsAfterBlockedUpdate),
					"pods shouldn't be recreated when update is blocked",
				)

				By("Update cluster update with strategy full")
				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassEverything}}
				updateSpecToTriggerAllComponentUpdate(ytsaurus)
				UpdateObject(ctx, ytsaurus)

				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents(
					"Discovery",
					"Master",
					"DataNode",
					"HttpProxy",
					"ExecNode",
					"TabletNode",
					"Scheduler",
					"ControllerAgent",
				))

				By("Wait cluster update with full update complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
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
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents("ExecNode"))

				By("Wait cluster update with selector:ExecNodesOnly complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				checkClusterBaseViability(ytClient)

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
				updateSpecToTriggerAllComponentUpdate(ytsaurus)
				UpdateObject(ctx, ytsaurus)

				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents("TabletNode"))

				By("Wait cluster update with selector:TabletNodesOnly complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				checkClusterBaseViability(ytClient)

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
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents("Master"))

				By("Wait cluster update with selector:MasterOnly complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				checkClusterBaseViability(ytClient)
				podsAfterMasterUpdate := getComponentPods(ctx, namespace)
				pods := getChangedPods(podsBeforeUpdate, podsAfterMasterUpdate)
				Expect(pods.Created).To(BeEmpty(), "created")
				Expect(pods.Deleted).To(BeEmpty(), "deleted")
				Expect(pods.Updated).To(ConsistOf("ms-0"), "updated")

				By("Run cluster update with selector:StatelessOnly")
				ytsaurus.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{
					Class: consts.ComponentClassStateless,
				}}
				updateSpecToTriggerAllComponentUpdate(ytsaurus)
				UpdateObject(ctx, ytsaurus)

				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveObservedGeneration())
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents(
					"Discovery",
					"HttpProxy",
					"ExecNode",
					"Scheduler",
					"ControllerAgent",
				))

				By("Wait cluster update with selector:StatelessOnly complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				checkClusterBaseViability(ytClient)
				podsAfterStatelessUpdate := getComponentPods(ctx, namespace)
				pods = getChangedPods(podsAfterMasterUpdate, podsAfterStatelessUpdate)
				Expect(pods.Deleted).To(BeEmpty(), "deleted")
				Expect(pods.Created).To(BeEmpty(), "created")
				Expect(pods.Updated).To(ConsistOf("ca-0", "ds-0", "end-0", "hp-0", "sch-0"), "updated")
			})

			It("Should update only specified data node group", func(ctx context.Context) {
				By("Adding second data node group")
				ytsaurus.Spec.DataNodes = append(ytsaurus.Spec.DataNodes, ytv1.DataNodesSpec{
					InstanceSpec: testutil.CreateDataNodeInstanceSpec(1),
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
				Expect(ytsaurus).Should(HaveClusterUpdatingComponentsExactly("dn-2"))

				By("Wait cluster update with selector:DataNodesOnly complete")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())
				checkClusterBaseViability(ytClient)

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
				overrides = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "config-overrides",
						Namespace: namespace,
					},
				}
				objects = append(objects, overrides)

				ytsaurus.Spec.ConfigOverrides = &corev1.LocalObjectReference{
					Name: overrides.Name,
				}
			})

			It("ConfigOverrides update shout trigger reconciliation", func(ctx context.Context) {
				By("Updating config overrides")
				overrides.Data = map[string]string{
					"ytserver-discovery.yson": "{resource_limits = {total_memory = 123456789;};}",
				}
				UpdateObject(ctx, overrides)

				By("Waiting cluster update starts")
				EventuallyYtsaurus(ctx, ytsaurus, reactionTimeout).Should(HaveClusterStateUpdating())
				Expect(ytsaurus).Should(HaveClusterUpdatingComponents("Discovery"))

				By("Waiting cluster update completes")
				EventuallyYtsaurus(ctx, ytsaurus, upgradeTimeout).Should(HaveClusterStateRunning())

				podsAfterUpdate := getComponentPods(ctx, namespace)
				pods := getChangedPods(podsBeforeUpdate, podsAfterUpdate)
				Expect(pods.Created).To(BeEmpty(), "created")
				Expect(pods.Deleted).To(BeEmpty(), "deleted")
				Expect(pods.Updated).To(ConsistOf("ds-0"), "updated")
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
					getKindControlPlaneName(),
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
				ytsaurus = testutil.WithTabletNodes(ytsaurus)
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
				for i := 0; i < int(ytsaurus.Spec.TabletNodes[0].InstanceCount); i++ {
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
				for i := 0; i < int(ytsaurus.Spec.DataNodes[0].InstanceCount); i++ {
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
				ytsaurus = testutil.WithQueryTracker(ytsaurus)
			})

			It("Should run with query tracker and check that query tracker rpc address set up correctly", func(ctx context.Context) {
				By("Check that query tracker channel exists in cluster_connection")
				Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/@cluster_connection/query_tracker/stages/production/channel"), nil)).Should(BeTrue())
			})

		}) // update query-tracker

		Context("With yql agent", Label("yql-agent"), func() {
			BeforeEach(func() {
				ytsaurus = testutil.WithQueryTracker(ytsaurus)
				ytsaurus = testutil.WithYqlAgent(ytsaurus)
			})

			It("Should run with yql agent and check that yql agent channel options set up correctly", func(ctx context.Context) {
				By("Creating ytsaurus client")
				ytClient := createYtsaurusClient(ytsaurus, namespace)

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
				ytsaurus = testutil.WithQueueAgent(ytsaurus)
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
				ytsaurus = testutil.WithBaseYtsaurusComponents(ytsaurus)

				By("Adding remote exec nodes")

				// Ensure that no local exec nodes exist, only remote ones (which will be created later).
				ytsaurus.Spec.ExecNodes = []ytv1.ExecNodesSpec{}

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
							CoreImage: testutil.CoreImageFirst,
						},
						ExecNodesSpec: ytv1.ExecNodesSpec{
							InstanceSpec: testutil.CreateExecNodeInstanceSpec(),
						},
					},
				}
				objects = append(objects, remoteNodes)
			})

			It("Should create ytsaurus with remote exec nodes and execute a job", func(ctx context.Context) {

				By("Running sort operation to ensure exec node works")
				op := runAndCheckSortOperation(ytClient)
				op.ID()

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
							CoreImage: testutil.CoreImageFirst,
						},
						DataNodesSpec: ytv1.DataNodesSpec{
							InstanceSpec: testutil.CreateDataNodeInstanceSpec(3),
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
				ytsaurus = testutil.WithDataNodes(ytsaurus)

				By("Adding remote tablet nodes")
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
							CoreImage: testutil.CoreImageFirst,
						},
						TabletNodesSpec: ytv1.TabletNodesSpec{
							InstanceSpec: testutil.CreateTabletNodeSpec(3),
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

		Context("With RPC proxy", Label("rpc-proxy"), func() {

			BeforeEach(func() {
				ytsaurus = testutil.WithRPCProxies(ytsaurus)
			})

			It("Rpc proxies should require authentication", func(ctx context.Context) {
				// Just in case, so dirty dev environment wouldn't interfere.
				Expect(os.Unsetenv("YT_TOKEN")).Should(Succeed())

				By("Checking RPC proxy without token does not work")
				cli, err := ytrpc.NewClient(&yt.Config{
					Proxy:                 getHTTPProxyAddress(g, namespace),
					RPCProxy:              getRPCProxyAddress(g, namespace),
					DisableProxyDiscovery: true,
				})
				Expect(err).Should(Succeed())

				_, err = cli.NodeExists(ctx, ypath.Path("/"), nil)
				Expect(yterrors.ContainsErrorCode(err, yterrors.CodeRPCAuthenticationError)).Should(BeTrue())

				By("Checking RPC proxy works with token")
				cli = getYtRPCClient(g, namespace)
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

	}) // integration
})

func checkClusterBaseViability(ytClient yt.Client) {
	By("Check that cluster is alive")

	res := make([]string, 0)
	Expect(ytClient.ListNode(ctx, ypath.Path("/"), &res, nil)).Should(Succeed())

	By("Check that tablet cell bundles are in `good` health")
	Eventually(func() bool {
		notGoodBundles, err := components.GetNotGoodTabletCellBundles(ctx, ytClient)
		if err != nil {
			return false
		}
		return len(notGoodBundles) == 0
	}, reactionTimeout, pollInterval).Should(BeTrue())
}

func checkClusterViability(ytClient yt.Client) {
	checkClusterBaseViability(ytClient)

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
	realChunkLocationPath := "//sys/@config/node_tracker/enable_real_chunk_locations"
	var realChunkLocationsValue bool
	Expect(ytClient.GetNode(ctx, ypath.Path(realChunkLocationPath), &realChunkLocationsValue, nil)).Should(Succeed())
	Expect(realChunkLocationsValue).Should(BeTrue())

	var values []yson.ValueWithAttrs
	Expect(ytClient.ListNode(ctx, ypath.Path("//sys/data_nodes"), &values, &yt.ListNodeOptions{
		Attributes: []string{"use_imaginary_chunk_locations"},
	})).Should(Succeed())

	for _, node := range values {
		Expect(node.Attrs["use_imaginary_chunk_locations"]).ShouldNot(BeTrue())
	}
}

func createYtsaurusClient(ytsaurus *ytv1.Ytsaurus, namespace string) yt.Client {
	By("Creating ytsaurus client")
	g := ytconfig.NewGenerator(ytsaurus, "local")
	return getYtClient(g, namespace)
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
	op, err := mrCli.Sort(&spec.Spec{
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
