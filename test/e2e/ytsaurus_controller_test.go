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
	"time"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.ytsaurus.tech/yt/go/mapreduce"
	"go.ytsaurus.tech/yt/go/mapreduce/spec"
	"go.ytsaurus.tech/yt/go/yt/ytrpc"
	"go.ytsaurus.tech/yt/go/yterrors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"

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
	upgradeTimeout   = time.Minute * 7
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
		types.NamespacedName{Name: svcName, Namespace: namespace},
		&svc),
	).Should(Succeed())

	k8sNode := corev1.Node{}
	Expect(k8sClient.Get(ctx,
		types.NamespacedName{Name: getKindControlPlaneName(), Namespace: namespace},
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
	err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &msPod)
	Expect(err).Should(Succeed())
	return msPod
}

func deleteYtsaurus(ctx context.Context, ytsaurus *ytv1.Ytsaurus) {
	if err := k8sClient.Delete(ctx, ytsaurus); err != nil {
		log.Error(err, "Deleting ytsaurus failed")
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "master-data-ms-0",
			Namespace: ytsaurus.Namespace,
		},
	}

	if err := k8sClient.Delete(ctx, pvc); err != nil {
		log.Error(err, "Deleting ytsaurus pvc failed")
	}

	if err := k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ytsaurus.Namespace}}); err != nil {
		log.Error(err, "Deleting namespace failed")
	}
}

func runYtsaurus(ytsaurus *ytv1.Ytsaurus) {
	Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ytsaurus.Namespace}})).Should(Succeed())

	Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())

	ytsaurusLookupKey := types.NamespacedName{Name: ytsaurus.Name, Namespace: ytsaurus.Namespace}

	Eventually(func() bool {
		createdYtsaurus := &ytv1.Ytsaurus{}
		err := k8sClient.Get(ctx, ytsaurusLookupKey, createdYtsaurus)
		return err == nil
	}, reactionTimeout, pollInterval).Should(BeTrue())

	pods := []string{"ms-0", "hp-0"}
	if ytsaurus.Spec.Discovery.InstanceCount != 0 {
		pods = append(pods, "ds-0")
	}
	for _, dataNodeGroup := range ytsaurus.Spec.DataNodes {
		pods = append(pods, fmt.Sprintf("%v-0", consts.FormatComponentStringWithDefault("dnd", dataNodeGroup.Name)))
	}

	if len(ytsaurus.Spec.ExecNodes) != 0 {
		pods = append(pods, "end-0")
	}

	By("Checking that ytsaurus state is equal to `Running`")
	EventuallyYtsaurus(ctx, ytsaurusLookupKey, bootstrapTimeout).Should(HaveClusterState(ytv1.ClusterStateRunning))

	By("Check pods are running")
	for _, podName := range pods {
		pod := &corev1.Pod{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: podName, Namespace: ytsaurus.Namespace}, pod)
		Expect(err).NotTo(HaveOccurred())
		Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
	}
}

func createRemoteYtsaurus(remoteYtsaurus *ytv1.RemoteYtsaurus) {
	Expect(k8sClient.Create(ctx, remoteYtsaurus)).Should(Succeed())
	lookupKey := types.NamespacedName{Name: remoteYtsaurus.Name, Namespace: remoteYtsaurus.Namespace}
	Eventually(func() bool {
		createdYtsaurus := &ytv1.RemoteYtsaurus{}
		err := k8sClient.Get(ctx, lookupKey, createdYtsaurus)
		return err == nil
	}, reactionTimeout, pollInterval).Should(BeTrue())
}

func deleteRemoteYtsaurus(ctx context.Context, remoteYtsaurus *ytv1.RemoteYtsaurus) {
	if err := k8sClient.Delete(ctx, remoteYtsaurus); err != nil {
		log.Error(err, "Deleting remote ytsaurus failed")
	}
}

func runRemoteExecNodes(remoteExecNodes *ytv1.RemoteExecNodes) {
	Expect(k8sClient.Create(ctx, remoteExecNodes)).Should(Succeed())
	lookupKey := types.NamespacedName{Name: remoteExecNodes.Name, Namespace: remoteExecNodes.Namespace}
	Eventually(func() bool {
		createdYtsaurus := &ytv1.RemoteExecNodes{}
		err := k8sClient.Get(ctx, lookupKey, createdYtsaurus)
		return err == nil
	}, reactionTimeout, pollInterval).Should(BeTrue())

	By("Checking that remote exec nodes state is equal to `Running`")
	Eventually(
		func() (*ytv1.RemoteExecNodes, error) {
			nodes := &ytv1.RemoteExecNodes{}
			err := k8sClient.Get(ctx, lookupKey, nodes)
			return nodes, err
		},
		reactionTimeout*2,
		pollInterval,
	).Should(HaveField("Status.ReleaseStatus", ytv1.RemoteExecNodeReleaseStatusRunning))
}

func deleteRemoteExecNodes(ctx context.Context, remoteExecNodes *ytv1.RemoteExecNodes) {
	if err := k8sClient.Delete(ctx, remoteExecNodes); err != nil {
		log.Error(err, "Deleting remote ytsaurus failed")
	}
}

func runRemoteDataNodes(remoteDataNodes *ytv1.RemoteDataNodes) {
	Expect(k8sClient.Create(ctx, remoteDataNodes)).Should(Succeed())
	lookupKey := types.NamespacedName{Name: remoteDataNodes.Name, Namespace: remoteDataNodes.Namespace}
	Eventually(func() bool {
		createdYtsaurus := &ytv1.RemoteDataNodes{}
		err := k8sClient.Get(ctx, lookupKey, createdYtsaurus)
		return err == nil
	}, reactionTimeout, pollInterval).Should(BeTrue())

	By("Checking that remote data nodes state is equal to `Running`")
	Eventually(
		func() (*ytv1.RemoteDataNodes, error) {
			nodes := &ytv1.RemoteDataNodes{}
			err := k8sClient.Get(ctx, lookupKey, nodes)
			return nodes, err
		},
		reactionTimeout*2,
		pollInterval,
	).Should(HaveField("Status.ReleaseStatus", ytv1.RemoteDataNodeReleaseStatusRunning))
}

func deleteRemoteDataNodes(ctx context.Context, remoteDataNodes *ytv1.RemoteDataNodes) {
	if err := k8sClient.Delete(ctx, remoteDataNodes); err != nil {
		log.Error(err, "Deleting remote ytsaurus failed")
	}
}

func runImpossibleUpdateAndRollback(ytsaurus *ytv1.Ytsaurus, ytClient yt.Client) {
	name := types.NamespacedName{Name: ytsaurus.Name, Namespace: ytsaurus.Namespace}

	By("Run cluster impossible update")
	Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())
	ytsaurus.Spec.CoreImage = testutil.CoreImageSecond
	Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())

	EventuallyYtsaurus(ctx, name, reactionTimeout).Should(HaveClusterUpdateState(ytv1.UpdateStateImpossibleToStart))

	By("Set previous core image")
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())
		ytsaurus.Spec.CoreImage = testutil.CoreImageFirst
		g.Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())
	}, reactionTimeout, pollInterval).Should(Succeed())

	By("Wait for running")
	EventuallyYtsaurus(ctx, name, upgradeTimeout).Should(HaveClusterState(ytv1.ClusterStateRunning))

	By("Check that cluster alive after update")
	res := make([]string, 0)
	Expect(ytClient.ListNode(ctx, ypath.Path("/"), &res, nil)).Should(Succeed())
}

func createConfigOverridesMap(namespace, name, key, value string) {
	resource := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{key: value},
	}
	Expect(k8sClient.Create(ctx, &resource)).Should(Succeed())
}

type testRow struct {
	A string `yson:"a"`
}

var _ = Describe("Basic test for Ytsaurus controller", func() {
	Context("When setting up the test environment", func() {
		It(
			"Should run and update Ytsaurus within same major version", Label("basic"), getSimpleUpdateScenario("test-minor-update", testutil.CoreImageSecond),
		)
		It(
			"Should run and update Ytsaurus to the next major version",
			getSimpleUpdateScenario("test-major-update", testutil.CoreImageNextVer),
		)
		It(
			"Should be updated according to UpdateSelector=Everything", func(ctx context.Context) {
				namespace := "testslcteverything"

				By("Creating a Ytsaurus resource")
				ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
				DeferCleanup(deleteYtsaurus, ytsaurus)
				name := types.NamespacedName{Name: ytsaurus.GetName(), Namespace: namespace}
				deployAndCheck(ytsaurus, namespace)
				podsBeforeUpdate := getComponentPods(ctx, namespace)

				By("Run cluster update with selector: nothing")
				Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())
				ytsaurus.Spec.UpdateSelector = ytv1.UpdateSelectorNothing
				// We want change in all yson configs, new discovery instance will trigger that.
				ytsaurus.Spec.Discovery.InstanceCount += 1
				Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())

				By("Ensure cluster doesn't start updating for 5 seconds")
				ConsistentlyYtsaurus(ctx, name, 5*time.Second).Should(HaveClusterState(ytv1.ClusterStateRunning))
				podsAfterBlockedUpdate := getComponentPods(ctx, namespace)
				Expect(podsBeforeUpdate).To(
					Equal(podsAfterBlockedUpdate),
					"pods shouldn't be recreated when update is blocked",
				)

				By("Update cluster update with strategy full")
				Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())
				ytsaurus.Spec.UpdateSelector = ytv1.UpdateSelectorEverything
				ytsaurus.Spec.Discovery.InstanceCount += 1
				Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())
				EventuallyYtsaurus(ctx, name, reactionTimeout).Should(HaveClusterUpdatingComponents())

				By("Wait cluster update with full update complete")
				EventuallyYtsaurus(ctx, name, upgradeTimeout).Should(HaveClusterState(ytv1.ClusterStateRunning))
				podsAfterFullUpdate := getComponentPods(ctx, namespace)

				podDiff := diffPodsCreation(podsBeforeUpdate, podsAfterFullUpdate)
				Expect(podDiff.created.Equal(NewStringSetFromItems("ds-1", "ds-2"))).To(BeTrue(), "unexpected pod diff created %v", podDiff.created)
				Expect(podDiff.deleted.IsEmpty()).To(BeTrue(), "unexpected pod diff deleted %v", podDiff.deleted)
				Expect(podDiff.recreated.Equal(NewStringSetFromMap(podsBeforeUpdate))).To(BeTrue(), "unexpected pod diff recreated %v", podDiff.recreated)
			},
		)
		It(
			"Should be updated according to UpdateSelector=TabletNodesOnly,ExecNodesOnly", func(ctx context.Context) {
				namespace := "testslctnodes"

				By("Creating a Ytsaurus resource")
				ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
				DeferCleanup(deleteYtsaurus, ytsaurus)
				name := types.NamespacedName{Name: ytsaurus.GetName(), Namespace: namespace}

				deployAndCheck(ytsaurus, namespace)
				podsBeforeUpdate := getComponentPods(ctx, namespace)

				By("Run cluster update with selector:ExecNodesOnly")
				Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())
				ytsaurus.Spec.UpdateSelector = ytv1.UpdateSelectorExecNodesOnly
				ytsaurus.Spec.Discovery.InstanceCount += 1
				Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())
				EventuallyYtsaurus(ctx, name, reactionTimeout).Should(HaveClusterUpdatingComponents("ExecNode"))

				By("Wait cluster update with selector:ExecNodesOnly complete")
				EventuallyYtsaurus(ctx, name, upgradeTimeout).Should(HaveClusterState(ytv1.ClusterStateRunning))
				ytClient := createYtsaurusClient(ytsaurus, namespace)
				checkClusterBaseViability(ytClient)

				podsAfterEndUpdate := getComponentPods(ctx, namespace)
				podDiff := diffPodsCreation(podsBeforeUpdate, podsAfterEndUpdate)
				Expect(podDiff.created.IsEmpty()).To(BeTrue(), "unexpected pod diff created %v", podDiff.created)
				Expect(podDiff.deleted.IsEmpty()).To(BeTrue(), "unexpected pod diff deleted %v", podDiff.deleted)
				Expect(podDiff.recreated.Equal(NewStringSetFromItems("end-0"))).To(
					BeTrue(), "unexpected pod diff recreated %v", podDiff.recreated)

				By("Run cluster update with selector:TabletNodesOnly")
				Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())
				ytsaurus.Spec.UpdateSelector = ytv1.UpdateSelectorTabletNodesOnly
				ytsaurus.Spec.Discovery.InstanceCount += 1
				Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())
				EventuallyYtsaurus(ctx, name, reactionTimeout).Should(HaveClusterUpdatingComponents("TabletNode"))

				By("Wait cluster update with selector:TabletNodesOnly complete")
				EventuallyYtsaurus(ctx, name, upgradeTimeout).Should(HaveClusterState(ytv1.ClusterStateRunning))
				checkClusterBaseViability(ytClient)

				podsAfterTndUpdate := getComponentPods(ctx, namespace)
				podDiff = diffPodsCreation(podsAfterEndUpdate, podsAfterTndUpdate)
				Expect(podDiff.created.IsEmpty()).To(BeTrue(), "unexpected pod diff created %v", podDiff.created)
				Expect(podDiff.deleted.IsEmpty()).To(BeTrue(), "unexpected pod diff deleted %v", podDiff.deleted)
				Expect(podDiff.recreated.Equal(NewStringSetFromItems("tnd-0", "tnd-1", "tnd-2"))).To(
					BeTrue(), "unexpected pod diff recreated %v", podDiff.recreated)
			},
		)
		It(
			"Should be updated according to UpdateSelector=MasterOnly,StatelessOnly", Label("basic"), func(ctx context.Context) {
				namespace := "testslctother"

				By("Creating a Ytsaurus resource")
				ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
				DeferCleanup(deleteYtsaurus, ytsaurus)
				name := types.NamespacedName{Name: ytsaurus.GetName(), Namespace: namespace}

				deployAndCheck(ytsaurus, namespace)
				podsBeforeUpdate := getComponentPods(ctx, namespace)

				By("Run cluster update with selector:MasterOnly")
				Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())
				ytsaurus.Spec.UpdateSelector = ytv1.UpdateSelectorMasterOnly
				ytsaurus.Spec.Discovery.InstanceCount += 1
				Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())
				EventuallyYtsaurus(ctx, name, reactionTimeout).Should(HaveClusterUpdatingComponents("Master"))

				By("Wait cluster update with selector:MasterOnly complete")
				EventuallyYtsaurus(ctx, name, upgradeTimeout).Should(HaveClusterState(ytv1.ClusterStateRunning))
				ytClient := createYtsaurusClient(ytsaurus, namespace)
				checkClusterBaseViability(ytClient)
				podsAfterMasterUpdate := getComponentPods(ctx, namespace)
				podDiff := diffPodsCreation(podsBeforeUpdate, podsAfterMasterUpdate)
				Expect(podDiff.created.IsEmpty()).To(BeTrue(), "unexpected pod diff created %v", podDiff.created)
				Expect(podDiff.deleted.IsEmpty()).To(BeTrue(), "unexpected pod diff deleted %v", podDiff.deleted)
				Expect(podDiff.recreated.Equal(NewStringSetFromItems("ms-0"))).To(
					BeTrue(), "unexpected pod diff recreated %v", podDiff.recreated)

				By("Run cluster update with selector:StatelessOnly")
				Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())
				ytsaurus.Spec.UpdateSelector = ytv1.UpdateSelectorStatelessOnly
				ytsaurus.Spec.Discovery.InstanceCount += 1
				Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())
				EventuallyYtsaurus(ctx, name, reactionTimeout).Should(
					HaveClusterUpdatingComponents("Discovery", "HttpProxy", "ExecNode", "Scheduler", "ControllerAgent"),
				)
				By("Wait cluster update with selector:StatelessOnly complete")
				EventuallyYtsaurus(ctx, name, upgradeTimeout).Should(HaveClusterState(ytv1.ClusterStateRunning))
				checkClusterBaseViability(ytClient)
				podsAfterStatelessUpdate := getComponentPods(ctx, namespace)
				podDiff = diffPodsCreation(podsAfterMasterUpdate, podsAfterStatelessUpdate)
				// Only with StatelessOnly strategy those pending ds pods should be finally created.
				Expect(podDiff.created.Equal(NewStringSetFromItems("ds-1", "ds-2"))).To(
					BeTrue(), "unexpected pod diff created %v", podDiff.created)
				Expect(podDiff.deleted.IsEmpty()).To(BeTrue(), "unexpected pod diff deleted %v", podDiff.deleted)
				Expect(podDiff.recreated.Equal(
					NewStringSetFromItems("ca-0", "end-0", "sch-0", "hp-0", "ds-0")),
				).To(BeTrue(), "unexpected pod diff recreated %v", podDiff.recreated)
			},
		)
		// This is a test for specific regression bug when master pods are recreated during PossibilityCheck stage.
		It("Master shouldn't be recreated before WaitingForPodsCreation state if config changes", func(ctx context.Context) {
			namespace := "test3"
			ytsaurus := testutil.CreateMinimalYtsaurusResource(namespace)
			ytsaurusKey := types.NamespacedName{Name: testutil.YtsaurusName, Namespace: namespace}

			By("Creating a Ytsaurus resource")
			g := ytconfig.NewGenerator(ytsaurus, "local")
			DeferCleanup(deleteYtsaurus, ytsaurus)
			runYtsaurus(ytsaurus)

			By("Creating ytsaurus client")
			ytClient := getYtClient(g, namespace)

			By("Check that cluster alive")
			res := make([]string, 0)
			Expect(ytClient.ListNode(ctx, ypath.Path("/"), &res, nil)).Should(Succeed())

			By("Store master pod creation time")
			msPod := getMasterPod(g.GetMasterPodNames()[0], namespace)
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
			Expect(k8sClient.Get(ctx, ytsaurusKey, ytsaurus)).Should(Succeed())
			ytsaurus.Spec.HostNetwork = true
			ytsaurus.Spec.PrimaryMasters.HostAddresses = []string{
				getKindControlPlaneName(),
			}
			Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())

			By("Waiting PossibilityCheck")
			EventuallyYtsaurus(ctx, ytsaurusKey, reactionTimeout).Should(HaveClusterUpdateState(ytv1.UpdateStatePossibilityCheck))

			By("Check that master pod was NOT recreated at the PossibilityCheck stage")
			time.Sleep(1 * time.Second)
			msPod = getMasterPod(g.GetMasterPodNames()[0], namespace)
			msPodCreationSecondTimestamp := msPod.CreationTimestamp
			fmt.Fprintf(GinkgoWriter, "ms pods ts: \n%v\n%v\n", msPodCreationFirstTimestamp, msPodCreationSecondTimestamp)
			Expect(msPodCreationFirstTimestamp.Equal(&msPodCreationSecondTimestamp)).Should(BeTrue())
		})

		It("Should run and try to update Ytsaurus with tablet cell bundle which is not in `good` health", func(ctx context.Context) {
			By("Creating a Ytsaurus resource")

			namespace := "test4"

			ytsaurus := testutil.CreateMinimalYtsaurusResource(namespace)
			ytsaurus = testutil.WithTabletNodes(ytsaurus)

			g := ytconfig.NewGenerator(ytsaurus, "local")

			DeferCleanup(deleteYtsaurus, ytsaurus)
			runYtsaurus(ytsaurus)

			By("Creating ytsaurus client")
			ytClient := getYtClient(g, namespace)

			By("Check that cluster alive")

			res := make([]string, 0)
			Expect(ytClient.ListNode(ctx, ypath.Path("/"), &res, nil)).Should(Succeed())

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

		It("Should run and try to update Ytsaurus with lvc", func(ctx context.Context) {
			By("Creating a Ytsaurus resource")

			namespace := "test5"

			ytsaurus := testutil.CreateMinimalYtsaurusResource(namespace)
			ytsaurus = testutil.WithDataNodes(ytsaurus)
			ytsaurus.Spec.TabletNodes = make([]ytv1.TabletNodesSpec, 0)

			g := ytconfig.NewGenerator(ytsaurus, "local")

			DeferCleanup(deleteYtsaurus, ytsaurus)
			runYtsaurus(ytsaurus)

			By("Creating ytsaurus client")
			ytClient := getYtClient(g, namespace)

			By("Check that cluster alive")
			res := make([]string, 0)
			Expect(ytClient.ListNode(ctx, ypath.Path("/"), &res, nil)).Should(Succeed())

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

		It("Should run with query tracker and check that query tracker rpc address set up correctly", Label("basic"), func(ctx context.Context) {
			By("Creating a Ytsaurus resource")

			namespace := "querytrackeraddress"

			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus = testutil.WithQueryTracker(ytsaurus)

			g := ytconfig.NewGenerator(ytsaurus, "local")

			DeferCleanup(deleteYtsaurus, ytsaurus)
			runYtsaurus(ytsaurus)

			By("Creating ytsaurus client")
			ytClient := getYtClient(g, namespace)

			By("Check that query tracker channel exists in cluster_connection")
			Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/@cluster_connection/query_tracker/stages/production/channel"), nil)).Should(BeTrue())
		})

		It("Should run with query tracker and check that access control objects set up correctly", Label("basic"), func(ctx context.Context) {
			By("Creating a Ytsaurus resource")

			namespace := "querytrackeraco"

			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			ytsaurus = testutil.WithQueryTracker(ytsaurus)
			ytsaurus = testutil.WithQueueAgent(ytsaurus)

			g := ytconfig.NewGenerator(ytsaurus, "local")

			DeferCleanup(deleteYtsaurus, ytsaurus)
			runYtsaurus(ytsaurus)

			By("Creating ytsaurus client")
			ytClient := getYtClient(g, namespace)

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

		It("Should create ytsaurus with remote exec nodes and execute a job", func() {
			By("Creating a Ytsaurus resource")
			ctx := context.Background()

			namespace := "remoteexec"

			ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
			// Ensure that no local exec nodes exist, only remote ones (which will be created later).
			ytsaurus.Spec.ExecNodes = []ytv1.ExecNodesSpec{}
			g := ytconfig.NewGenerator(ytsaurus, "local")

			remoteYtsaurus := &ytv1.RemoteYtsaurus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testutil.RemoteResourceName,
					Namespace: namespace,
				},
				Spec: ytv1.RemoteYtsaurusSpec{
					MasterConnectionSpec: ytv1.MasterConnectionSpec{
						CellTag: ytsaurus.Spec.PrimaryMasters.CellTag,
						HostAddresses: []string{
							"ms-0.masters.remoteexec.svc.cluster.local",
						},
					},
				},
			}

			remoteNodes := &ytv1.RemoteExecNodes{
				ObjectMeta: metav1.ObjectMeta{Name: testutil.RemoteResourceName, Namespace: namespace},
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

			defer deleteYtsaurus(ctx, ytsaurus)
			runYtsaurus(ytsaurus)

			defer deleteRemoteYtsaurus(ctx, remoteYtsaurus)
			createRemoteYtsaurus(remoteYtsaurus)

			defer deleteRemoteExecNodes(ctx, remoteNodes)
			runRemoteExecNodes(remoteNodes)

			By("Creating ytsaurus client")
			ytClient := getYtClient(g, namespace)

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
			Expect(statuses).Should(Not(BeEmpty()))
			for _, status := range statuses {
				Expect(status.Address).Should(
					ContainSubstring("end-"+testutil.RemoteResourceName),
					"actual status: %s", status,
				)
			}
		})

		It("Should create ytsaurus with remote data nodes and write to a table", func() {
			By("Creating a Ytsaurus resource")
			ctx := context.Background()

			namespace := "remotedata"

			// We have to create only the minimal YT - with master and discovery servers
			ytsaurus := testutil.CreateMinimalYtsaurusResource(namespace)
			g := ytconfig.NewGenerator(ytsaurus, "local")

			remoteYtsaurus := &ytv1.RemoteYtsaurus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testutil.RemoteResourceName,
					Namespace: namespace,
				},
				Spec: ytv1.RemoteYtsaurusSpec{
					MasterConnectionSpec: ytv1.MasterConnectionSpec{
						CellTag: ytsaurus.Spec.PrimaryMasters.CellTag,
						HostAddresses: []string{
							"ms-0.masters.remotedata.svc.cluster.local",
						},
					},
				},
			}

			remoteNodes := &ytv1.RemoteDataNodes{
				ObjectMeta: metav1.ObjectMeta{Name: testutil.RemoteResourceName, Namespace: namespace},
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

			defer deleteYtsaurus(ctx, ytsaurus)
			runYtsaurus(ytsaurus)

			defer deleteRemoteYtsaurus(ctx, remoteYtsaurus)
			createRemoteYtsaurus(remoteYtsaurus)

			defer deleteRemoteDataNodes(ctx, remoteNodes)
			runRemoteDataNodes(remoteNodes)

			By("Creating ytsaurus client")
			ytClient := getYtClient(g, namespace)

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
		It(
			"Rpc proxies should require authentication",
			func(ctx context.Context) {
				namespace := "testrpc"
				ytsaurus := testutil.CreateMinimalYtsaurusResource(namespace)
				ytsaurus = testutil.WithRPCProxies(ytsaurus)
				DeferCleanup(deleteYtsaurus, ytsaurus)
				deployAndCheck(ytsaurus, namespace)

				// Just in case, so dirty dev environment wouldn't interfere.
				Expect(os.Unsetenv("YT_TOKEN")).Should(Succeed())
				g := ytconfig.NewGenerator(ytsaurus, "local")
				cli, err := ytrpc.NewClient(&yt.Config{
					// N.B.: no credentials are passed here.
					RPCProxy:              getRPCProxyAddress(g, namespace),
					DisableProxyDiscovery: true,
				})
				Expect(err).Should(Succeed())

				_, err = cli.NodeExists(context.Background(), ypath.Path("/"), nil)
				Expect(yterrors.ContainsErrorCode(err, yterrors.CodeRPCAuthenticationError)).Should(BeTrue())
			},
		)

		It(
			"ConfigOverrides update shout trigger reconciliation",
			func(ctx context.Context) {
				namespace := "test-overrides"
				coName := "config-overrides"
				ytsaurus := testutil.CreateMinimalYtsaurusResource(namespace)
				ytsaurus.Spec.ConfigOverrides = &corev1.LocalObjectReference{Name: coName}
				DeferCleanup(deleteYtsaurus, ytsaurus)

				deployAndCheck(ytsaurus, namespace)
				log.Info("sleep a little, because after cluster is running some reconciliations still may happen " +
					"for some time (and for some reason) and we don't want to interfere before map creation")
				time.Sleep(3 * time.Second)

				dsPodName := "ds-0"
				msPodName := "ms-0"

				getPodByName := func(name string) (*corev1.Pod, error) {
					ds0Name := types.NamespacedName{Name: name, Namespace: namespace}
					dsPod := &corev1.Pod{}
					err := k8sClient.Get(ctx, ds0Name, dsPod)
					return dsPod, err
				}
				dsPod, err := getPodByName(dsPodName)
				Expect(err).Should(Succeed())
				msPod, err := getPodByName(msPodName)
				Expect(err).Should(Succeed())
				dsPodCreatedBefore := dsPod.CreationTimestamp.Time
				log.Info("ds created before", "ts", dsPodCreatedBefore)
				msPodCreatedBefore := msPod.CreationTimestamp.Time

				discoveryOverride := "{resource_limits = {total_memory = 123456789;};}"
				createConfigOverridesMap(namespace, coName, "ytserver-discovery.yson", discoveryOverride)

				Eventually(ctx, func() bool {
					pod, err := getPodByName(dsPodName)
					if apierrors.IsNotFound(err) {
						return false
					}
					dsPodCreatedAfter := pod.CreationTimestamp
					log.Info("ds created after", "ts", dsPodCreatedAfter)
					return dsPodCreatedAfter.After(dsPodCreatedBefore)
				}).WithTimeout(30 * time.Second).
					WithPolling(300 * time.Millisecond).
					Should(BeTrueBecause("ds pod should be recreated on configOverrides creation"))

				msPod, err = getPodByName(msPodName)
				Expect(err).Should(Succeed())
				Expect(msPod.CreationTimestamp.Time).Should(
					Equal(msPodCreatedBefore), "ms pods shouldn't be recreated",
				)
			},
		)

		It("Sensors should be annotated with host", func(ctx context.Context) {
			namespace := "testsolomon"
			ytsaurus := testutil.CreateMinimalYtsaurusResource(namespace)
			ytsaurus.Spec.HostNetwork = true
			DeferCleanup(deleteYtsaurus, ytsaurus)
			deployAndCheck(ytsaurus, namespace)

			By("Creating ytsaurus client")
			g := ytconfig.NewGenerator(ytsaurus, "local")
			ytClient := getYtClient(g, namespace)

			primaryMasters := make([]string, 0)
			Expect(ytClient.ListNode(ctx, ypath.Path("//sys/primary_masters"), &primaryMasters, nil)).Should(Succeed())
			Expect(primaryMasters).Should(Not(BeEmpty()))

			masterAddress := primaryMasters[0]

			monitoringPort := 0
			Expect(ytClient.GetNode(ctx, ypath.Path(fmt.Sprintf("//sys/primary_masters/%v/orchid/config/monitoring_port", masterAddress)), &monitoringPort, nil)).Should(Succeed())

			msPod := getMasterPod(g.GetMasterPodNames()[0], namespace)
			fmt.Fprintf(GinkgoWriter, "podIP: %v\n", msPod.Status.PodIP)

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
			fmt.Fprintf(GinkgoWriter, "host=%v, pod=%v\n", labels["host"], labels["pod"])
		})
	})

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
		fmt.Fprintf(GinkgoWriter, "PodName: %v, Labels: %v\n", pod.Name, pod.ObjectMeta.Labels)
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

func deployAndCheck(ytsaurus *ytv1.Ytsaurus, namespace string) {
	runYtsaurus(ytsaurus)

	ytClient := createYtsaurusClient(ytsaurus, namespace)
	checkClusterViability(ytClient)
}

func createYtsaurusClient(ytsaurus *ytv1.Ytsaurus, namespace string) yt.Client {
	By("Creating ytsaurus client")
	g := ytconfig.NewGenerator(ytsaurus, "local")
	return getYtClient(g, namespace)
}

func getSimpleUpdateScenario(namespace, newImage string) func(ctx context.Context) {
	return func(ctx context.Context) {
		By("Creating a Ytsaurus resource")
		ytsaurus := testutil.CreateBaseYtsaurusResource(namespace)
		testutil.WithNamedDataNodes(ytsaurus, ptr.To("dn-t"))
		DeferCleanup(deleteYtsaurus, ytsaurus)
		name := types.NamespacedName{Name: ytsaurus.GetName(), Namespace: namespace}
		deployAndCheck(ytsaurus, namespace)

		By("Run cluster update")
		podsBeforeUpdate := getComponentPods(ctx, namespace)
		Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())

		checkPodLabels(ctx, namespace)

		ytsaurus.Spec.CoreImage = newImage
		Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())
		EventuallyYtsaurus(ctx, name, reactionTimeout).Should(HaveClusterState(ytv1.ClusterStateUpdating))

		By("Wait cluster update complete")
		EventuallyYtsaurus(ctx, name, upgradeTimeout).Should(HaveClusterState(ytv1.ClusterStateRunning))
		g := ytconfig.NewGenerator(ytsaurus, "local")
		ytClient := getYtClient(g, namespace)
		checkClusterBaseViability(ytClient)

		podsAfterFullUpdate := getComponentPods(ctx, namespace)
		podDiff := diffPodsCreation(podsBeforeUpdate, podsAfterFullUpdate)
		newYt := ytv1.Ytsaurus{}
		err := k8sClient.Get(ctx, name, &newYt)
		Expect(err).Should(Succeed())

		Expect(podDiff.created.IsEmpty()).To(BeTrue(), "unexpected pod diff created %v", podDiff.created)
		Expect(podDiff.deleted.IsEmpty()).To(BeTrue(), "unexpected pod diff deleted %v", podDiff.deleted)
		Expect(podDiff.recreated.Equal(NewStringSetFromMap(podsBeforeUpdate))).To(BeTrue(), "unexpected pod diff recreated %v", podDiff.recreated)
		Expect(newYt.Status.ObservedGeneration).To(Equal(newYt.Generation))
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
