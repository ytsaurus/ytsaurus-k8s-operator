package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

const (
	timeout  = time.Second * 90
	interval = time.Millisecond * 250
)

func getYtClient(g *ytconfig.Generator, namespace string) yt.Client {
	httpProxyService := corev1.Service{}
	Expect(k8sClient.Get(ctx,
		types.NamespacedName{Name: g.GetHTTPProxiesServiceName(consts.DefaultHTTPProxyRole), Namespace: namespace},
		&httpProxyService),
	).Should(Succeed())

	k8sNode := corev1.Node{}

	Expect(k8sClient.Get(ctx,
		types.NamespacedName{Name: getKindControlPlaneName(), Namespace: namespace},
		&k8sNode),
	).Should(Succeed())

	ytProxy := os.Getenv("E2E_YT_PROXY")
	if ytProxy == "" {
		httpProxyAddress := ""
		for _, address := range k8sNode.Status.Addresses {
			if address.Type == corev1.NodeInternalIP {
				httpProxyAddress = address.Address
			}
		}
		port := httpProxyService.Spec.Ports[0].NodePort
		ytProxy = fmt.Sprintf("%s:%v", httpProxyAddress, port)

	}

	ytClient, err := ythttp.NewClient(&yt.Config{
		Proxy:                 ytProxy,
		Token:                 consts.DefaultAdminPassword,
		DisableProxyDiscovery: true,
	})
	Expect(err).Should(Succeed())

	return ytClient
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
	logger := log.FromContext(ctx)

	if err := k8sClient.Delete(ctx, ytsaurus); err != nil {
		logger.Error(err, "Deleting ytsaurus failed")
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      "master-data-ms-0",
			Namespace: ytsaurus.Namespace,
		},
	}

	if err := k8sClient.Delete(ctx, pvc); err != nil {
		logger.Error(err, "Deleting ytsaurus pvc failed")
	}

	if err := k8sClient.Delete(ctx, &corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: ytsaurus.Namespace}}); err != nil {
		logger.Error(err, "Deleting namespace failed")
	}
}

func runYtsaurus(ytsaurus *ytv1.Ytsaurus) {
	Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: v1.ObjectMeta{Name: ytsaurus.Namespace}})).Should(Succeed())

	Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())

	ytsaurusLookupKey := types.NamespacedName{Name: ytsaurus.Name, Namespace: ytsaurus.Namespace}

	Eventually(func() bool {
		createdYtsaurus := &ytv1.Ytsaurus{}
		err := k8sClient.Get(ctx, ytsaurusLookupKey, createdYtsaurus)
		if err != nil {
			return false
		}
		return true
	}, timeout, interval).Should(BeTrue())

	By("Check pods are running")
	pods := []string{"ms-0", "hp-0"}
	if ytsaurus.Spec.Discovery.InstanceCount != 0 {
		pods = append(pods, "ds-0")
	}
	if len(ytsaurus.Spec.DataNodes) != 0 {
		pods = append(pods, "dnd-0")
	}
	if len(ytsaurus.Spec.ExecNodes) != 0 {
		pods = append(pods, "end-0")
	}
	for _, podName := range pods {
		Eventually(func() bool {
			pod := &corev1.Pod{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: podName, Namespace: ytsaurus.Namespace}, pod)
			if err != nil {
				return false
			}
			return pod.Status.Phase == corev1.PodRunning
		}, timeout, interval).Should(BeTrue())
	}

	By("Checking that ytsaurus state is equal to `Running`")
	Eventually(func() ytv1.ClusterState {
		ytsaurus := &ytv1.Ytsaurus{}
		err := k8sClient.Get(ctx, ytsaurusLookupKey, ytsaurus)
		if err != nil {
			return ytv1.ClusterStateCreated
		}
		return ytsaurus.Status.State
	}, timeout*2, interval).Should(Equal(ytv1.ClusterStateRunning))
}

func runImpossibleUpdateAndRollback(ytsaurus *ytv1.Ytsaurus, ytClient yt.Client) {

	By("Run cluster update")
	name := ytsaurus.Name
	namespace := ytsaurus.Namespace

	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ytsaurus)).Should(Succeed())
	ytsaurus.Spec.CoreImage = ytv1.CoreImageSecond
	Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())

	Eventually(func() bool {
		ytsaurus := &ytv1.Ytsaurus{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ytsaurus)
		if err != nil {
			return false
		}
		return ytsaurus.Status.State == ytv1.ClusterStateUpdating &&
			ytsaurus.Status.UpdateStatus.State == ytv1.UpdateStateImpossibleToStart
	}, timeout, interval).Should(BeTrue())

	By("Set previous core image")
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ytsaurus)).Should(Succeed())
		ytsaurus.Spec.CoreImage = ytv1.CoreImageFirst
		g.Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())
	}, timeout, interval).Should(Succeed())

	By("Wait for running")
	Eventually(func() bool {
		ytsaurus := &ytv1.Ytsaurus{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, ytsaurus)
		if err != nil {
			return false
		}
		return ytsaurus.Status.State == ytv1.ClusterStateRunning
	}, timeout*3, interval).Should(BeTrue())

	By("Check that cluster alive after update")
	res := make([]string, 0)
	Expect(ytClient.ListNode(ctx, ypath.Path("/"), &res, nil)).Should(Succeed())
}

type testRow struct {
	A string `yson:"a"`
}

var _ = Describe("Basic test for Ytsaurus controller", func() {
	Context("When setting up the test environment", func() {
		It(
			"Should run and update Ytsaurus within same major version",
			getSimpleUpdateScenario("test-minor-update", ytv1.CoreImageSecond),
		)
		It(
			"Should run and update Ytsaurus to the next major version",
			getSimpleUpdateScenario("test-major-update", ytv1.CoreImageNextVer),
		)

		// This is a test for specific regression bug when master pods are recreated during PossibilityCheck stage.
		It("Master shouldn't be recreated before WaitingForPodsCreation state if config changes", func() {
			namespace := "test3"
			ytsaurus := ytv1.CreateMinimalYtsaurusResource(namespace)
			ytsaurusKey := types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}

			By("Creating a Ytsaurus resource")
			ctx := context.Background()
			g := ytconfig.NewGenerator(ytsaurus, "local")
			defer deleteYtsaurus(ctx, ytsaurus)
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
			Expect(len(masters)).Should(Equal(1))
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
			Eventually(func() bool {
				ytsaurus := &ytv1.Ytsaurus{}
				err := k8sClient.Get(ctx, ytsaurusKey, ytsaurus)
				if err != nil {
					return false
				}
				return ytsaurus.Status.State == ytv1.ClusterStateUpdating &&
					ytsaurus.Status.UpdateStatus.State == ytv1.UpdateStatePossibilityCheck
			}, timeout, interval).Should(BeTrue())

			By("Check that master pod was NOT recreated at the PossibilityCheck stage")
			time.Sleep(1 * time.Second)
			msPod = getMasterPod(g.GetMasterPodNames()[0], namespace)
			msPodCreationSecondTimestamp := msPod.CreationTimestamp
			fmt.Fprintf(GinkgoWriter, "ms pods ts: \n%v\n%v\n", msPodCreationFirstTimestamp, msPodCreationSecondTimestamp)
			Expect(msPodCreationFirstTimestamp == msPodCreationSecondTimestamp).Should(BeTrue())
		})

		It("Should run and try to update Ytsaurus with tablet cell bundle which is not in `good` health", func() {
			By("Creating a Ytsaurus resource")
			ctx := context.Background()

			namespace := "test4"

			ytsaurus := ytv1.CreateBaseYtsaurusResource(namespace)

			g := ytconfig.NewGenerator(ytsaurus, "local")

			defer deleteYtsaurus(ctx, ytsaurus)
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
			}, timeout*3, interval).Should(BeTrue())

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
			}, timeout, interval).Should(BeTrue())

			runImpossibleUpdateAndRollback(ytsaurus, ytClient)
		})

		It("Should run and try to update Ytsaurus with lvc", func() {
			By("Creating a Ytsaurus resource")
			ctx := context.Background()

			namespace := "test5"

			ytsaurus := ytv1.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.TabletNodes = make([]ytv1.TabletNodesSpec, 0)

			g := ytconfig.NewGenerator(ytsaurus, "local")

			defer deleteYtsaurus(ctx, ytsaurus)
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
				g.Expect(err).Should(BeNil())
				g.Expect(writer.Write(testRow{A: "123"})).Should(Succeed())
				g.Expect(writer.Commit()).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Ban all data nodes")
			for i := 0; i < int(ytsaurus.Spec.DataNodes[0].InstanceCount); i++ {
				Expect(ytClient.SetNode(ctx, ypath.Path(fmt.Sprintf(
					"//sys/cluster_nodes/dnd-%v.data-nodes.%v.svc.cluster.local:9012/@banned", i, namespace)), true, nil))
			}

			By("Waiting for lvc > 0")
			Eventually(func() bool {
				lvcCount := 0
				err := ytClient.GetNode(ctx, ypath.Path("//sys/lost_vital_chunks/@count"), &lvcCount, nil)
				if err != nil {
					return false
				}
				return lvcCount > 0
			}, timeout, interval).Should(BeTrue())

			runImpossibleUpdateAndRollback(ytsaurus, ytClient)
		})

		It("Should run with query tracker and check that access control object namespace 'queries' and object 'nobody' exists", func() {
			By("Creating a Ytsaurus resource")
			ctx := context.Background()

			namespace := "querytrackeraco"

			ytsaurus := ytv1.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.TabletNodes = []ytv1.TabletNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1},
				},
			}
			ytsaurus.Spec.QueryTrackers = &ytv1.QueryTrackerSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1}}

			g := ytconfig.NewGenerator(ytsaurus, "local")

			defer deleteYtsaurus(ctx, ytsaurus)
			runYtsaurus(ytsaurus)

			By("Creating ytsaurus client")
			ytClient := getYtClient(g, namespace)

			By("Check that access control object namespace 'queries' exists")
			Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/access_control_object_namespaces/queries"), nil)).Should(Equal(true))

			By("Check that access control object 'nobody' in namespace 'queries' exists")
			Expect(ytClient.NodeExists(ctx, ypath.Path("//sys/access_control_object_namespaces/queries/nobody"), nil)).Should(Equal(true))
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
	}, timeout, interval).Should(BeTrue())
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

func getSimpleUpdateScenario(namespace, newImage string) func() {
	return func() {

		By("Creating a Ytsaurus resource")
		ctx := context.Background()

		ytsaurus := ytv1.CreateBaseYtsaurusResource(namespace)

		g := ytconfig.NewGenerator(ytsaurus, "local")

		defer deleteYtsaurus(ctx, ytsaurus)
		runYtsaurus(ytsaurus)

		By("Creating ytsaurus client")

		ytClient := getYtClient(g, namespace)

		checkClusterViability(ytClient)

		By("Run cluster update")

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)).Should(Succeed())
		ytsaurus.Spec.CoreImage = newImage
		Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())

		Eventually(func() bool {
			ytsaurus := &ytv1.Ytsaurus{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)
			if err != nil {
				return false
			}
			return ytsaurus.Status.State == ytv1.ClusterStateUpdating
		}, timeout, interval).Should(BeTrue())

		Eventually(func() bool {
			ytsaurus := &ytv1.Ytsaurus{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)
			if err != nil {
				return false
			}
			return ytsaurus.Status.State == ytv1.ClusterStateRunning
		}, timeout*5, interval).Should(BeTrue())

		checkClusterBaseViability(ytClient)
	}
}
