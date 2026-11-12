package controllers_test

import (
	"context"
	"fmt"
	"time"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/controllers"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

const (
	ytsaurusName = "testsaurus"
	dndsNameOne  = "dn-1"
)

var _ = Describe("Ytsaurus Controller", func() {
	var h *testutil.TestHelper
	var namespace string

	JustBeforeEach(func() {
		h = testutil.NewTestHelper(GinkgoTB(), namespace, "..")
		reconcilerSetup := func(mgr ctrl.Manager) error {
			return (&controllers.YtsaurusReconciler{
				BaseReconciler: controllers.BaseReconciler{
					ClusterDomain: "cluster.local",
					Client:        mgr.GetClient(),
					Scheme:        mgr.GetScheme(),
					Recorder:      mgr.GetEventRecorderFor("ytsaurus-controller"),
				},
			}).SetupWithManager(mgr)
		}
		h.Start(reconcilerSetup)
	})

	waitClusterState := func(h *testutil.TestHelper, expectedState ytv1.ClusterState, minObservedGeneration int64) {
		h.Logf("[ Wait for YTsaurus %s state ]", expectedState)
		testutil.FetchAndCheckEventually(
			h,
			ytsaurusName,
			&ytv1.Ytsaurus{},
			fmt.Sprintf("cluster state is %s, gen is %d", expectedState, minObservedGeneration),
			func(obj client.Object) bool {
				state := obj.(*ytv1.Ytsaurus).Status.State
				observedGen := obj.(*ytv1.Ytsaurus).Status.ObservedGeneration
				if !(state == expectedState && observedGen >= minObservedGeneration) {
					h.Logf(
						"state condition is NOT yet satisfied: %s == %s; gen %d >= %d",
						state, expectedState, observedGen, minObservedGeneration,
					)
					return false
				}
				h.Logf(
					"state condition is satisfied: %s == %s; gen %d >= %d",
					state, expectedState, observedGen, minObservedGeneration,
				)
				return true
			},
		)
	}

	Describe("Ytsaurus operations", func() {
		Context("With separate component resources", func() {
			BeforeEach(func() {
				namespace = "ytsaurus-components"
			})

			It("should reconcile attached nodes and proxies when they are created", func(ctx context.Context) {
				ytsaurusResource := testutil.BuildMinimalYtsaurus(namespace, ytsaurusName)
				ytsaurusResource.Spec.EphemeralCluster = true
				ytsaurusResource.Spec.PrimaryMasters.MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.Discovery.MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.HTTPProxies[0].MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.DataNodes[0].MinReadyInstanceCount = ptr.To(int32(0))
				testutil.DeployObject(h, &ytsaurusResource)
				waitClusterState(h, ytv1.ClusterStateRunning, ytsaurusResource.Generation)

				meta := metav1.ObjectMeta{Name: "attached", Namespace: namespace}
				parent := ytv1.YtsaurusReference{Name: ytsaurusName}
				instance := ytv1.InstanceSpec{InstanceCount: 1, MinReadyInstanceCount: ptr.To(int32(0))}
				dataSpec := *ytsaurusResource.Spec.DataNodes[0].DeepCopy()
				dataSpec.Name = "attached"

				resources := []struct {
					object      client.Object
					statefulSet string
				}{
					{&ytv1.YtsaurusDataNodes{
						ObjectMeta: meta,
						Spec:       ytv1.YtsaurusDataNodesSpec{Ytsaurus: parent, DataNodesSpec: dataSpec},
					}, "dnd-attached"},
					{&ytv1.YtsaurusTabletNodes{
						ObjectMeta: meta,
						Spec: ytv1.YtsaurusTabletNodesSpec{
							Ytsaurus:        parent,
							TabletNodesSpec: ytv1.TabletNodesSpec{InstanceSpec: instance, Name: "attached"},
						},
					}, "tnd-attached"},
					{&ytv1.YtsaurusHTTPProxies{
						ObjectMeta: meta,
						Spec: ytv1.YtsaurusHTTPProxiesSpec{
							Ytsaurus:        parent,
							HTTPProxiesSpec: ytv1.HTTPProxiesSpec{InstanceSpec: instance, Role: "attached", HttpPort: ptr.To(int32(8080))},
						},
					}, "hp-attached"},
					{&ytv1.YtsaurusRPCProxies{
						ObjectMeta: meta,
						Spec: ytv1.YtsaurusRPCProxiesSpec{
							Ytsaurus:       parent,
							RPCProxiesSpec: ytv1.RPCProxiesSpec{InstanceSpec: instance, Role: "attached"},
						},
					}, "rp-attached"},
				}

				for _, resource := range resources {
					Expect(h.GetK8sClient().Create(ctx, resource.object)).To(Succeed())
					testutil.FetchEventually(h, resource.statefulSet, &appsv1.StatefulSet{})
				}
			})
		})

		Context("With a separate update plan", func() {
			BeforeEach(func() { namespace = "ytsaurus-update-plan" })

			deployRunningCluster := func() ytv1.Ytsaurus {
				ytsaurusResource := testutil.BuildMinimalYtsaurus(namespace, ytsaurusName)
				ytsaurusResource.Spec.EphemeralCluster = true
				ytsaurusResource.Spec.PrimaryMasters.MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.Discovery.MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.HTTPProxies[0].MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.DataNodes[0].MinReadyInstanceCount = ptr.To(int32(0))
				testutil.DeployObject(h, &ytsaurusResource)
				waitClusterState(h, ytv1.ClusterStateRunning, ytsaurusResource.Generation)
				return ytsaurusResource
			}

			waitPlanState := func(plan *ytv1.YtsaurusUpdatePlan, state ytv1.ComponentState) {
				testutil.FetchAndCheckEventually(h, plan.Name, &ytv1.YtsaurusUpdatePlan{}, "plan state is "+string(state), func(obj client.Object) bool {
					stored := obj.(*ytv1.YtsaurusUpdatePlan)
					return stored.Status.State == state && stored.Status.ObservedGeneration == plan.Generation
				})
			}

			waitPlanDeleted := func(ctx context.Context, plan *ytv1.YtsaurusUpdatePlan) {
				Eventually(func() bool {
					return apierrors.IsNotFound(h.GetK8sClient().Get(ctx, client.ObjectKeyFromObject(plan), &ytv1.YtsaurusUpdatePlan{}))
				}).WithTimeout(30 * time.Second).Should(BeTrueBecause("the one-shot plan must be deleted when the cluster is running"))
			}

			DescribeTable("executes all selected batches and handles one-shot cleanup", func(ctx context.Context, oneShot bool) {
				ytsaurusResource := deployRunningCluster()
				originalMasterImage := ytsaurusResource.Spec.PrimaryMasters.Image
				image := testYtsaurusImage + "-planned"
				ytsaurusResource.Spec.Discovery.Image = &image
				ytsaurusResource.Spec.HTTPProxies[0].Image = &image
				ytsaurusResource.Spec.PrimaryMasters.Image = &image
				testutil.UpdateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)
				waitClusterState(h, ytv1.ClusterStateUpdateBlocked, ytsaurusResource.Generation)

				plan := &ytv1.YtsaurusUpdatePlan{
					ObjectMeta: metav1.ObjectMeta{Name: "plan", Namespace: namespace},
					Spec: ytv1.YtsaurusUpdatePlanSpec{
						Ytsaurus:   ytv1.YtsaurusReference{Name: ytsaurusName},
						OneShot:    oneShot,
						UpdatePlan: ytv1.UpdatePlan{{Class: ytv1.ComponentClassNothing}},
					},
				}
				testutil.DeployObject(h, plan)
				waitPlanState(plan, ytv1.ComponentStateBlocked)

				// Only the plan changes: its watch must wake the blocked cluster.
				plan.Spec.UpdatePlan = ytv1.UpdatePlan{{Class: ytv1.ComponentClassStateless, Concurrency: ptr.To(int32(1))}}
				testutil.UpdateObject(h, &ytv1.YtsaurusUpdatePlan{}, plan)
				for _, name := range []string{"ds", "hp"} {
					testutil.FetchAndCheckEventually(h, name, &appsv1.StatefulSet{}, "selected image updated", func(obj client.Object) bool {
						return obj.(*appsv1.StatefulSet).Spec.Template.Spec.Containers[0].Image == image
					})
				}
				waitClusterState(h, ytv1.ClusterStateUpdateBlocked, ytsaurusResource.Generation)
				waitPlanState(plan, ytv1.ComponentStateBlocked)
				var masters appsv1.StatefulSet
				testutil.GetObject(h, "ms", &masters)
				Expect(masters.Spec.Template.Spec.Containers[0].Image).NotTo(Equal(image))

				// The unselected master update keeps even a one-shot plan blocked.
				// Reverting that change lets the cluster finish and consume the plan.
				ytsaurusResource.Spec.PrimaryMasters.Image = originalMasterImage
				testutil.UpdateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)
				waitClusterState(h, ytv1.ClusterStateRunning, ytsaurusResource.Generation)
				if oneShot {
					waitPlanDeleted(ctx, plan)
				} else {
					waitPlanState(plan, ytv1.ComponentStateReady)
				}
			}, Entry("one-shot plan", true), Entry("persistent plan", false))

			It("removes a one-shot plan when no updates are needed", func(ctx context.Context) {
				deployRunningCluster()
				plan := &ytv1.YtsaurusUpdatePlan{
					ObjectMeta: metav1.ObjectMeta{Name: "plan", Namespace: namespace},
					Spec: ytv1.YtsaurusUpdatePlanSpec{
						Ytsaurus: ytv1.YtsaurusReference{Name: ytsaurusName},
						OneShot:  true,
					},
				}
				testutil.DeployObject(h, plan)
				waitPlanDeleted(ctx, plan)
			})
		})

		Context("When creating Ytsaurus from scratch", func() {
			BeforeEach(func() {
				namespace = "ytsaurus-from-scratch"
			})

			It("should create all resources and reach running state", func(ctx context.Context) {

				ytsaurusResource := testutil.BuildMinimalYtsaurus(namespace, ytsaurusName)
				// FIXME(khlebnikov): This test is broken by design.
				ytsaurusResource.Spec.EphemeralCluster = true
				ytsaurusResource.Spec.PrimaryMasters.MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.Discovery.MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.HTTPProxies[0].MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.DataNodes[0].MinReadyInstanceCount = ptr.To(int32(0))
				testutil.DeployObject(h, &ytsaurusResource)

				for _, compName := range []string{
					"discovery",
					"master",
					"http-proxy",
				} {
					testutil.FetchAndCheckConfigMapContainsEventually(
						h,
						"yt-"+compName+"-config",
						"ytserver-"+compName+".yson",
						"ms-0.masters."+namespace+".svc.cluster.local:9010",
					)
				}
				testutil.FetchAndCheckConfigMapContainsEventually(
					h,
					"yt-data-node-"+dndsNameOne+"-config",
					"ytserver-data-node.yson",
					"ms-0.masters."+namespace+".svc.cluster.local:9010",
				)

				for _, stsName := range []string{
					"ds",
					"ms",
					"hp",
					"dnd-" + dndsNameOne,
				} {
					testutil.FetchEventually(
						h,
						stsName,
						&appsv1.StatefulSet{},
					)
				}

				testutil.FetchAndCheckEventually(
					h,
					"yt-client-secret",
					&corev1.Secret{},
					"secret with not empty token",
					func(obj client.Object) bool {
						secret := obj.(*corev1.Secret)
						return len(secret.Data["YT_TOKEN"]) != 0
					},
				)
				waitClusterState(h, ytv1.ClusterStateRunning, ytsaurusResource.Generation)
			})
		})

		Context("When updating stateless component", func() {
			BeforeEach(func() {
				namespace = "upd-discovery"
			})

			It("should update discovery component image", func(ctx context.Context) {

				ytsaurusResource := testutil.BuildMinimalYtsaurus(namespace, ytsaurusName)
				// FIXME(khlebnikov): This test is broken by design.
				ytsaurusResource.Spec.EphemeralCluster = true
				ytsaurusResource.Spec.PrimaryMasters.MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.Discovery.MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.HTTPProxies[0].MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.DataNodes[0].MinReadyInstanceCount = ptr.To(int32(0))
				testutil.DeployObject(h, &ytsaurusResource)

				waitClusterState(h, ytv1.ClusterStateRunning, ytsaurusResource.Generation)

				imageUpdated := testYtsaurusImage + "-updated"
				ytsaurusResource.Spec.Discovery.Image = &imageUpdated
				GinkgoWriter.Println("[ Updating discovery with stateless update plan ]")
				ytsaurusResource.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{Class: ytv1.ComponentClassStateless}}
				testutil.UpdateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)

				waitClusterState(h, ytv1.ClusterStateRunning, ytsaurusResource.Generation)

				sts := appsv1.StatefulSet{}
				testutil.GetObject(h, "ds", &sts)
				Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal(imageUpdated))
			})
		})

		Context("When updating master with disabled full update", func() {
			BeforeEach(func() {
				namespace = "upd-master"
			})

			It("should block master update", func(ctx context.Context) {

				ytsaurusResource := testutil.BuildMinimalYtsaurus(namespace, ytsaurusName)
				// FIXME(khlebnikov): This test is broken by design.
				ytsaurusResource.Spec.EphemeralCluster = true
				ytsaurusResource.Spec.PrimaryMasters.MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.Discovery.MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.HTTPProxies[0].MinReadyInstanceCount = ptr.To(int32(0))
				ytsaurusResource.Spec.DataNodes[0].MinReadyInstanceCount = ptr.To(int32(0))
				testutil.DeployObject(h, &ytsaurusResource)

				waitClusterState(h, ytv1.ClusterStateRunning, ytsaurusResource.Generation)

				imageUpdated := testYtsaurusImage + "-updated"
				ytsaurusResource.Spec.PrimaryMasters.Image = &imageUpdated
				GinkgoWriter.Println("[ Updating master with stateless update plan (should block) ]")
				ytsaurusResource.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{Class: ytv1.ComponentClassStateless}}
				testutil.UpdateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)

				waitClusterState(h, ytv1.ClusterStateUpdateBlocked, ytsaurusResource.Generation)
			})
		})
	})
})
