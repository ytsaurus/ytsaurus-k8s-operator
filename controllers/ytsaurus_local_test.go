package controllers

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

const (
	ytsaurusName      = "testsaurus"
	testYtsaurusImage = "test-ytsaurus-image"
	dndsNameOne       = "dn-1"
)

func TestYtsaurusFromScratch(t *testing.T) {
	require.NoError(t, os.Setenv("ENABLE_NEW_FLOW", "true"))
	namespace := "ytsaurus-from-scratch"
	h := newTestHelper(t, namespace)
	h.start()
	defer h.stop()

	remoteYtsaurusSpec := buildMinimalYtsaurus(h, ytsaurusName)
	deployObject(h, &remoteYtsaurusSpec)

	for _, compName := range []string{
		"discovery",
		"master",
		"http-proxy",
	} {
		fetchAndCheckConfigMapContainsEventually(
			h,
			"yt-"+compName+"-config",
			"ytserver-"+compName+".yson",
			"ms-0.masters."+namespace+".svc.cluster.local:9010",
		)
	}

	for _, stsName := range []string{
		"ds",
		"ms",
		"hp",
	} {
		fetchAndCheckEventually(
			h,
			stsName,
			&appsv1.StatefulSet{},
			func(obj client.Object) bool { return true },
		)
	}

	fetchAndCheckEventually(
		h,
		"yt-client-secret",
		&corev1.Secret{},
		func(obj client.Object) bool {
			secret := obj.(*corev1.Secret)
			return len(secret.Data["YT_TOKEN"]) != 0
		},
	)

	fetchAndCheckEventually(
		h,
		ytsaurusName,
		&ytv1.Ytsaurus{},
		func(obj client.Object) bool {
			state := obj.(*ytv1.Ytsaurus).Status.State
			return state == ytv1.ClusterStateRunning
		},
	)
}

func buildMinimalYtsaurus(h *testHelper, name string) ytv1.Ytsaurus {
	remoteYtsaurus := ytv1.Ytsaurus{
		ObjectMeta: h.getObjectMeta(name),
		Spec: ytv1.YtsaurusSpec{
			CoreImage:        testYtsaurusImage,
			IsManaged:        true,
			EnableFullUpdate: false,
			UseShortNames:    true,

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
				CellTag: 1,
			},
			HTTPProxies: []ytv1.HTTPProxiesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 3},
					ServiceType:  corev1.ServiceTypeNodePort,
				},
			},
			DataNodes: []ytv1.DataNodesSpec{
				{
					InstanceSpec:     ytv1.InstanceSpec{InstanceCount: 5},
					ClusterNodesSpec: ytv1.ClusterNodesSpec{},
					Name:             dndsNameOne,
				},
			},
		},
	}
	return remoteYtsaurus
}
