package controllers

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const (
	namespace = "default"

	timeout  = time.Second * 90
	interval = time.Millisecond * 250
)

func getYtClient(g *ytconfig.Generator) yt.Client {
	httpProxyService := corev1.Service{}
	Expect(k8sClient.Get(ctx,
		types.NamespacedName{Name: g.GetHTTPProxiesServiceName(consts.DefaultHTTPProxyRole), Namespace: namespace},
		&httpProxyService),
	).Should(Succeed())

	port := httpProxyService.Spec.Ports[0].NodePort

	k8sNode := corev1.Node{}
	Expect(k8sClient.Get(ctx,
		types.NamespacedName{Name: "kind-control-plane", Namespace: namespace},
		&k8sNode),
	).Should(Succeed())

	httpProxyAddress := ""
	for _, address := range k8sNode.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			httpProxyAddress = address.Address
		}
	}

	ytClient, err := ythttp.NewClient(&yt.Config{
		Proxy: fmt.Sprintf("%s:%v", httpProxyAddress, port),
		Token: consts.DefaultAdminPassword,
	})
	Expect(err).Should(Succeed())

	return ytClient
}

func deleteYtsaurus(ctx context.Context, ytsaurus *ytv1.Ytsaurus) {
	logger := log.FromContext(ctx)

	if err := k8sClient.Delete(ctx, ytsaurus); err != nil {
		logger.Error(err, "Deleting ytsaurus failed")
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: v1.ObjectMeta{
			Name:      "master-data-ms-0",
			Namespace: namespace,
		},
	}

	if err := k8sClient.Delete(ctx, pvc); err != nil {
		logger.Error(err, "Deleting ytsaurus pvc failed")
	}
}

func runYtsaurus(ytsaurus *ytv1.Ytsaurus) {
	Expect(k8sClient.Create(ctx, ytsaurus)).Should(Succeed())

	ytsaurusLookupKey := types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}

	Eventually(func() bool {
		createdYtsaurus := &ytv1.Ytsaurus{}
		err := k8sClient.Get(ctx, ytsaurusLookupKey, createdYtsaurus)
		if err != nil {
			return false
		}
		return true
	}, timeout, interval).Should(BeTrue())

	By("Check pods are running")
	for _, podName := range []string{"ds-0", "ms-0", "hp-default-0", "dnd-0", "end-0"} {
		Eventually(func() bool {
			pod := &corev1.Pod{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: podName, Namespace: namespace}, pod)
			if err != nil {
				return false
			}
			return pod.Status.Phase == corev1.PodRunning
		}, timeout, interval).Should(BeTrue())
	}

	By("Checking that ytsaurus state is equal to `Running`")
	Eventually(func() bool {
		ytsaurus := &ytv1.Ytsaurus{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)
		if err != nil {
			return false
		}
		return ytsaurus.Status.State == ytv1.ClusterStateRunning
	}, timeout*2, interval).Should(BeTrue())
}

func runImpossibleUpdateAndRollback(ytsaurus *ytv1.Ytsaurus, ytClient yt.Client) {

	By("Run cluster update")
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)).Should(Succeed())
	ytsaurus.Spec.CoreImage = "ytsaurus/ytsaurus:dev"
	Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())

	Eventually(func() bool {
		ytsaurus := &ytv1.Ytsaurus{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)
		if err != nil {
			return false
		}
		return ytsaurus.Status.State == ytv1.ClusterStateUpdating &&
			ytsaurus.Status.UpdateStatus.State == ytv1.UpdateStateImpossibleToStart
	}, timeout, interval).Should(BeTrue())

	By("Set previous core image")
	Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)).Should(Succeed())
	ytsaurus.Spec.CoreImage = "ytsaurus/ytsaurus:23.1-latest"
	Expect(k8sClient.Update(ctx, ytsaurus)).Should(Succeed())

	By("Wait for running")
	Eventually(func() bool {
		ytsaurus := &ytv1.Ytsaurus{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)
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
		It("Should run and update Ytsaurus", func() {

			By("Creating a Ytsaurus resource")
			ctx := context.Background()

			ytsaurus := ytv1.CreateBaseYtsaurusResource(namespace)

			g := ytconfig.NewGenerator(ytsaurus, "local")

			defer deleteYtsaurus(ctx, ytsaurus)
			runYtsaurus(ytsaurus)

			By("Creating ytsaurus client")

			ytClient := getYtClient(g)

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
			}, timeout, interval).Should(BeTrue())

			By("Run cluster update")

			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: ytv1.YtsaurusName, Namespace: namespace}, ytsaurus)).Should(Succeed())
			ytsaurus.Spec.CoreImage = "ytsaurus/ytsaurus:dev"
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
			}, timeout*3, interval).Should(BeTrue())

			By("Check that cluster alive after update")

			Expect(ytClient.ListNode(ctx, ypath.Path("/"), &res, nil)).Should(Succeed())
		})

		It("Should run and try to update Ytsaurus with tablet cell bundle which is not in `good` health", func() {
			By("Creating a Ytsaurus resource")
			ctx := context.Background()

			ytsaurus := ytv1.CreateBaseYtsaurusResource(namespace)

			g := ytconfig.NewGenerator(ytsaurus, "local")

			defer deleteYtsaurus(ctx, ytsaurus)
			runYtsaurus(ytsaurus)

			By("Creating ytsaurus client")
			ytClient := getYtClient(g)

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
					"//sys/cluster_nodes/tnd-%v.tablet-nodes.default.svc.cluster.local:9012/@banned", i)), true, nil))
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

			ytsaurus := ytv1.CreateBaseYtsaurusResource(namespace)
			ytsaurus.Spec.TabletNodes = make([]ytv1.TabletNodesSpec, 0)

			g := ytconfig.NewGenerator(ytsaurus, "local")

			defer deleteYtsaurus(ctx, ytsaurus)
			runYtsaurus(ytsaurus)

			By("Creating ytsaurus client")
			ytClient := getYtClient(g)

			By("Check that cluster alive")
			res := make([]string, 0)
			Expect(ytClient.ListNode(ctx, ypath.Path("/"), &res, nil)).Should(Succeed())

			By("Create a chunk")
			_, err := ytClient.CreateNode(ctx, ypath.Path("//tmp/a"), yt.NodeTable, nil)
			Expect(err).Should(Succeed())
			writer, err := ytClient.WriteTable(ctx, ypath.Path("//tmp/a"), nil)
			Expect(err).Should(Succeed())
			Expect(writer.Write(testRow{A: "123"})).Should(Succeed())
			Expect(writer.Commit()).Should(Succeed())

			By("Ban all data nodes")
			for i := 0; i < int(ytsaurus.Spec.DataNodes[0].InstanceCount); i++ {
				Expect(ytClient.SetNode(ctx, ypath.Path(fmt.Sprintf(
					"//sys/cluster_nodes/dnd-%v.data-nodes.default.svc.cluster.local:9012/@banned", i)), true, nil))
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
	})

})
