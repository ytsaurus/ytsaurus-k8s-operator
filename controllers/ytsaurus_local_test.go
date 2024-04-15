package controllers_test

import (
	"path/filepath"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/controllers"
	"github.com/ytsaurus/yt-k8s-operator/pkg/testutil"
)

const (
	ytsaurusName      = "testsaurus"
	testYtsaurusImage = "test-ytsaurus-image"
	dndsNameOne       = "dn-1"
)

func TestYtsaurusFromScratch(t *testing.T) {
	namespace := "ytsaurus-from-scratch"
	h := testutil.NewTestHelper(t, namespace, filepath.Join("..", "config", "crd", "bases"))
	reconcilerSetup := func(mgr ctrl.Manager) error {
		return (&controllers.YtsaurusReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("ytsaurus-controller"),
		}).SetupWithManager(mgr)
	}
	h.Start(reconcilerSetup)
	defer h.Stop()

	ytsaurusResource := testutil.BuildMinimalYtsaurus(namespace, ytsaurusName)
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

	testutil.FetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		"cluster state is running",
		func(obj client.Object) bool {
			state := obj.(*ytv1.Ytsaurus).Status.State
			return state == ytv1.ClusterStateRunning
		},
	)
}

func TestYtsaurusUpdateStatelessComponent(t *testing.T) {
	namespace := "upd-discovery"
	h := testutil.NewTestHelper(t, namespace, filepath.Join("..", "config", "crd", "bases"))
	reconcilerSetup := func(mgr ctrl.Manager) error {
		return (&controllers.YtsaurusReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("ytsaurus-controller"),
		}).SetupWithManager(mgr)
	}
	h.Start(reconcilerSetup)
	defer h.Stop()

	ytsaurusResource := testutil.BuildMinimalYtsaurus(namespace, ytsaurusName)
	testutil.DeployObject(h, &ytsaurusResource)

	testutil.FetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		"cluster state is updating",
		func(obj client.Object) bool {
			state := obj.(*ytv1.Ytsaurus).Status.State
			return state == ytv1.ClusterStateUpdating
		},
	)

	imageUpdated := testYtsaurusImage + "-updated"
	ytsaurusResource.Spec.Discovery.Image = &imageUpdated
	t.Log("[ Updating discovery with disabled full update ]")
	ytsaurusResource.Spec.EnableFullUpdate = false
	testutil.UpdateObject(h, &ytv1.Ytsaurus{}, &ytsaurusResource)

	t.Log("[ Wait for YTsaurus Running status ]")
	testutil.FetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		"cluster state is running",
		func(obj client.Object) bool {
			state := obj.(*ytv1.Ytsaurus).Status.State
			return state == ytv1.ClusterStateRunning
		},
	)
}
