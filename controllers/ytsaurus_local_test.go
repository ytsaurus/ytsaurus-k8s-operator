package controllers_test

import (
	"fmt"
	"testing"
	"time"

	"k8s.io/utils/ptr"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/controllers"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

const (
	ytsaurusName      = "testsaurus"
	testYtsaurusImage = "test-ytsaurus-image"
	dndsNameOne       = "dn-1"
)

func prepareTest(t *testing.T, namespace string) *testutil.TestHelper {
	h := testutil.NewTestHelper(t, namespace, "..")
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
	return h
}

func waitClusterState(h *testutil.TestHelper, expectedState ytv1.ClusterState, minObservedGeneration int64) {
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

func TestYtsaurusFromScratch(t *testing.T) {
	namespace := "ytsaurus-from-scratch"
	h := prepareTest(t, namespace)
	defer h.Stop()

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
}

func TestYtsaurusUpdateStatelessComponent(t *testing.T) {
	namespace := "upd-discovery"
	h := prepareTest(t, namespace)
	defer h.Stop()

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
	t.Log("[ Updating discovery with disabled full update ]")
	ytsaurusResource.Spec.EnableFullUpdate = false
	testutil.UpdateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)

	waitClusterState(h, ytv1.ClusterStateRunning, ytsaurusResource.Generation)

	sts := appsv1.StatefulSet{}
	testutil.GetObject(h, "ds", &sts)
	require.Equal(t, imageUpdated, sts.Spec.Template.Spec.Containers[0].Image)
}

func TestYtsaurusUpdateMasterBlocked(t *testing.T) {
	namespace := "upd-master"
	h := prepareTest(t, namespace)
	defer h.Stop()

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
	t.Log("[ Updating master with disabled full update (should block) ]")
	ytsaurusResource.Spec.EnableFullUpdate = false
	testutil.UpdateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)

	// It is a bit hard to test if something NOT happened, so we just wait a little and check if update happened.
	t.Log("[ Sleep for 5 seconds]")
	time.Sleep(5 * time.Second)

	t.Log("[ Check that cluster is running but not updated ]")
	ytsaurus := ytv1.Ytsaurus{}
	testutil.GetObject(h, ytsaurusName, &ytsaurus)
	require.Equal(t, ytv1.ClusterStateRunning, ytsaurus.Status.State)
	sts := appsv1.StatefulSet{}
	testutil.GetObject(h, "ms", &sts)
	require.NotEqual(t, imageUpdated, sts.Spec.Template.Spec.Containers[0].Image)
}
