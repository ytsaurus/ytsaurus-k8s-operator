package controllers_test

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	h := testutil.NewTestHelper(t, namespace)
	reconcilerSetup := func(mgr ctrl.Manager) error {
		return (&controllers.YtsaurusReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor("ytsaurus-controller"),
		}).SetupWithManager(mgr)
	}
	h.Start(reconcilerSetup)
	defer h.Stop()

	ytsaurusResource := buildMinimalYtsaurus(namespace, ytsaurusName)
	testutil.DeployObject(h, &ytsaurusResource)

	// emulate master init job succeeded
	testutil.MarkJobSucceeded(h, "yt-master-init-job-default")
	testutil.MarkJobSucceeded(h, "yt-client-init-job-user")

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

func buildMinimalYtsaurus(namespace, name string) ytv1.Ytsaurus {
	return ytv1.Ytsaurus{
		ObjectMeta: v1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: ytv1.YtsaurusSpec{
			CommonSpec: ytv1.CommonSpec{
				CoreImage:     testYtsaurusImage,
				UseShortNames: true,
			},
			IsManaged:        true,
			EnableFullUpdate: false,

			Discovery: ytv1.DiscoverySpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount: 3,
				},
			},
			PrimaryMasters: ytv1.MastersSpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount: 3,
					Locations: []ytv1.LocationSpec{
						{
							LocationType: "MasterChangelogs",
							Path:         "/yt/master-data/master-changelogs",
						},
						{
							LocationType: "MasterSnapshots",
							Path:         "/yt/master-data/master-snapshots",
						},
					},
				},
				MasterConnectionSpec: ytv1.MasterConnectionSpec{
					CellTag: 1,
				},
			},
			HTTPProxies: []ytv1.HTTPProxiesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 3},
					ServiceType:  corev1.ServiceTypeNodePort,
				},
			},
			DataNodes: []ytv1.DataNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 5,
						Locations: []ytv1.LocationSpec{
							{
								LocationType: "ChunkStore",
								Path:         "/yt/node-data/chunk-store",
							},
						},
					},
					ClusterNodesSpec: ytv1.ClusterNodesSpec{},
					Name:             dndsNameOne,
				},
			},
		},
	}
}
