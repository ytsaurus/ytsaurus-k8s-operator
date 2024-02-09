package controllers

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
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
	h := newTestHelper(t, "ytsaurus-from-scratch")
	h.start()
	defer h.stop()

	remoteYtsaurusSpec := buildMinimalYtsaurus(h, ytsaurusName)
	deployObject(h, &remoteYtsaurusSpec)

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

			Discovery: ytv1.DiscoverySpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount: 3,
				},
			},
			PrimaryMasters: ytv1.MastersSpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount: 3,
				},
				CellTag: 1,
			},
			HTTPProxies: []ytv1.HTTPProxiesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 3},
					ServiceType:  v1.ServiceTypeNodePort,
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
