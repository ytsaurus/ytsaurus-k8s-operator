package controllers_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"go.ytsaurus.tech/yt/go/mapreduce"
	"go.ytsaurus.tech/yt/go/mapreduce/spec"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
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
	if len(ytsaurus.Spec.DataNodes) != 0 {
		pods = append(pods, "dnd-0")
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

	By("Checking that remote nodes state is equal to `Running`")
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

func runImpossibleUpdateAndRollback(ytsaurus *ytv1.Ytsaurus, ytClient yt.Client) {
	name := types.NamespacedName{Name: ytsaurus.Name, Namespace: ytsaurus.Namespace}

	By("Run cluster impossible update")
	Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())
	ytsaurus.Spec.CoreImage = ytv1.CoreImageSecond
	Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())

	EventuallyYtsaurus(ctx, name, reactionTimeout).Should(HaveClusterUpdateState(ytv1.UpdateStateImpossibleToStart))

	By("Set previous core image")
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())
		ytsaurus.Spec.CoreImage = ytv1.CoreImageFirst
		g.Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())
	}, reactionTimeout, pollInterval).Should(Succeed())

	By("Wait for running")
	EventuallyYtsaurus(ctx, name, upgradeTimeout).Should(HaveClusterState(ytv1.ClusterStateRunning))

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
		It("Master shouldn't be recreated before WaitingForPodsCreation state if config changes", func(ctx context.Context) {
			namespace := "test3"
			ytsaurus := ytv1.CreateMinimalYtsaurusResource(namespace)
			ytsaurusKey := types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}

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

			ytsaurus := ytv1.CreateMinimalYtsaurusResource(namespace)
			ytsaurus = ytv1.WithTabletNodes(ytsaurus)

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

			ytsaurus := ytv1.CreateMinimalYtsaurusResource(namespace)
			ytsaurus = ytv1.WithDataNodes(ytsaurus)
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

		It("Should run with query tracker and check that access control objects set up correctly", func(ctx context.Context) {
			By("Creating a Ytsaurus resource")

			namespace := "querytrackeraco"

			ytsaurus := ytv1.CreateBaseYtsaurusResource(namespace)
			ytsaurus = ytv1.WithQueryTracker(ytsaurus)

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

			ytsaurus := ytv1.CreateBaseYtsaurusResource(namespace)
			// Ensure that no local exec nodes exist, only remote ones (which will be created later).
			ytsaurus.Spec.ExecNodes = []ytv1.ExecNodesSpec{}
			g := ytconfig.NewGenerator(ytsaurus, "local")

			remoteYtsaurus := &ytv1.RemoteYtsaurus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ytv1.RemoteResourceName,
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
				ObjectMeta: metav1.ObjectMeta{Name: ytv1.RemoteResourceName, Namespace: namespace},
				Spec: ytv1.RemoteExecNodesSpec{
					RemoteClusterSpec: &corev1.LocalObjectReference{
						Name: ytv1.RemoteResourceName,
					},
					CommonSpec: ytv1.CommonSpec{
						CoreImage: ytv1.CoreImageFirst,
					},
					ExecNodesSpec: ytv1.ExecNodesSpec{
						InstanceSpec: ytv1.CreateExecNodeInstanceSpec(),
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
					ContainSubstring("end-"+ytv1.RemoteResourceName),
					"actual status: %s", status,
				)
			}
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

func getSimpleUpdateScenario(namespace, newImage string) func(ctx context.Context) {
	return func(ctx context.Context) {

		By("Creating a Ytsaurus resource")

		ytsaurus := ytv1.CreateBaseYtsaurusResource(namespace)
		name := types.NamespacedName{Name: ytsaurus.GetName(), Namespace: namespace}

		g := ytconfig.NewGenerator(ytsaurus, "local")

		DeferCleanup(deleteYtsaurus, ytsaurus)
		runYtsaurus(ytsaurus)

		By("Creating ytsaurus client")

		ytClient := getYtClient(g, namespace)

		checkClusterViability(ytClient)

		By("Run cluster update")

		Expect(k8sClient.Get(ctx, name, ytsaurus)).Should(Succeed())
		ytsaurus.Spec.CoreImage = newImage
		Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())

		EventuallyYtsaurus(ctx, name, reactionTimeout).Should(HaveClusterState(ytv1.ClusterStateUpdating))

		By("Wait cluster update complete")

		EventuallyYtsaurus(ctx, name, upgradeTimeout).Should(HaveClusterState(ytv1.ClusterStateRunning))

		checkClusterBaseViability(ytClient)
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
