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
		Context("When creating Ytsaurus from scratch", func() {
			BeforeEach(func() {
				namespace = "ytsaurus-from-scratch"
			})

			It("should create all resources and reach running state", func(ctx context.Context) {

				ytsaurusResource := testutil.BuildMinimalYtsaurus(namespace, ytsaurusName)
				// FIXME(khlebnikov): This test is broken by design.
				ytsaurusResource.Spec.EphemeralCluster = true
				ytsaurusResource.Spec.PrimaryMasters.MinReadyInstanceCount = ptr.To(0)
				ytsaurusResource.Spec.Discovery.MinReadyInstanceCount = ptr.To(0)
				ytsaurusResource.Spec.HTTPProxies[0].MinReadyInstanceCount = ptr.To(0)
				ytsaurusResource.Spec.DataNodes[0].MinReadyInstanceCount = ptr.To(0)
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
				ytsaurusResource.Spec.PrimaryMasters.MinReadyInstanceCount = ptr.To(0)
				ytsaurusResource.Spec.Discovery.MinReadyInstanceCount = ptr.To(0)
				ytsaurusResource.Spec.HTTPProxies[0].MinReadyInstanceCount = ptr.To(0)
				ytsaurusResource.Spec.DataNodes[0].MinReadyInstanceCount = ptr.To(0)
				testutil.DeployObject(h, &ytsaurusResource)

				waitClusterState(h, ytv1.ClusterStateRunning, ytsaurusResource.Generation)

				imageUpdated := testYtsaurusImage + "-updated"
				ytsaurusResource.Spec.Discovery.Image = &imageUpdated
				GinkgoWriter.Println("[ Updating discovery with disabled full update ]")
				ytsaurusResource.Spec.EnableFullUpdate = ptr.To(false)
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
				ytsaurusResource.Spec.PrimaryMasters.MinReadyInstanceCount = ptr.To(0)
				ytsaurusResource.Spec.Discovery.MinReadyInstanceCount = ptr.To(0)
				ytsaurusResource.Spec.HTTPProxies[0].MinReadyInstanceCount = ptr.To(0)
				ytsaurusResource.Spec.DataNodes[0].MinReadyInstanceCount = ptr.To(0)
				testutil.DeployObject(h, &ytsaurusResource)

				waitClusterState(h, ytv1.ClusterStateRunning, ytsaurusResource.Generation)

				imageUpdated := testYtsaurusImage + "-updated"
				ytsaurusResource.Spec.PrimaryMasters.Image = &imageUpdated
				GinkgoWriter.Println("[ Updating master with disabled full update (should block) ]")
				ytsaurusResource.Spec.EnableFullUpdate = ptr.To(false)
				testutil.UpdateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)

				By("Checking that cluster is running but not updated")
				Consistently(func() bool {
					ytsaurus := ytv1.Ytsaurus{}
					testutil.GetObject(h, ytsaurusName, &ytsaurus)
					if ytsaurus.Status.State != ytv1.ClusterStateRunning {
						return false
					}
					sts := appsv1.StatefulSet{}
					testutil.GetObject(h, "ms", &sts)
					return sts.Spec.Template.Spec.Containers[0].Image != imageUpdated
				}, 5*time.Second, 500*time.Millisecond).Should(BeTrue())
			})
		})
	})
})
