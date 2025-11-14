package controllers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/controllers"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	remoteTabletNodesName      = "test-remote-tablet-nodes"
	statefulSetNameTabletNodes = "tnd-test-remote-tablet-nodes"
	tabletNodeConfigMapName    = "yt-tablet-node-config"
	tabletNodeConfigMapYsonKey = "ytserver-tablet-node.yson"
)

func setupRemoteTabletNodesReconciler() func(mgr ctrl.Manager) error {
	return func(mgr ctrl.Manager) error {
		return (&controllers.RemoteTabletNodesReconciler{
			BaseReconciler: controllers.BaseReconciler{
				ClusterDomain: "cluster.local",
				Client:        mgr.GetClient(),
				Scheme:        mgr.GetScheme(),
				Recorder:      mgr.GetEventRecorderFor("remotetabletnodes-controller"),
			},
		}).SetupWithManager(mgr)
	}
}

// TestRemoteTabletNodesFromScratch ensures that remote tablet nodes resources are
// created with correct connection to the specified remote Ytsaurus.
func TestRemoteTabletNodesFromScratch(t *testing.T) {
	h := startHelperWithController(t, "remote-tablet-nodes-test-from-scratch",
		setupRemoteTabletNodesReconciler(),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteTabletNodes(h, remoteYtsaurusName, remoteTabletNodesName)
	testutil.DeployObject(h, &nodes)
	waitRemoteTabletNodesDeployed(h, remoteTabletNodesName)

	testutil.FetchEventually(h, statefulSetNameTabletNodes, &appsv1.StatefulSet{})

	ysonNodeConfig := testutil.FetchConfigMapData(h, tabletNodeConfigMapName, tabletNodeConfigMapYsonKey)
	require.NotEmpty(t, ysonNodeConfig)
	require.Contains(t, ysonNodeConfig, remoteYtsaurusHostname)
}

// TestRemoteTabletNodesYtsaurusChanges ensures that if remote Ytsaurus CRD changes its hostnames
// remote nodes changes its configs accordingly.
func TestRemoteTabletNodesYtsaurusChanges(t *testing.T) {
	h := startHelperWithController(t, "remote-tablet-nodes-test-host-change",
		setupRemoteTabletNodesReconciler(),
	)
	defer h.Stop()

	remoteYtsaurus := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurus)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteTabletNodes(h, remoteYtsaurusName, remoteTabletNodesName)
	testutil.DeployObject(h, &nodes)
	waitRemoteTabletNodesDeployed(h, remoteTabletNodesName)

	ysonNodeConfig := testutil.FetchConfigMapData(h, tabletNodeConfigMapName, tabletNodeConfigMapYsonKey)
	require.NotEmpty(t, ysonNodeConfig)
	require.Contains(t, ysonNodeConfig, remoteYtsaurusHostname)

	hostnameChanged := remoteYtsaurusHostname + "-changed"
	remoteYtsaurus.Spec.MasterConnectionSpec.HostAddresses = []string{hostnameChanged}
	testutil.UpdateObject(h, &ytv1.RemoteYtsaurus{}, &remoteYtsaurus)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)

	testutil.FetchAndCheckEventually(
		h,
		tabletNodeConfigMapName,
		&corev1.ConfigMap{},
		"config map exists and contains changed hostname",
		func(obj client.Object) bool {
			data := obj.(*corev1.ConfigMap).Data
			ysonNodeConfig = data[tabletNodeConfigMapYsonKey]
			return strings.Contains(ysonNodeConfig, hostnameChanged)
		},
	)
}

// TestRemoteTabletNodesImageUpdate ensures that if remote tablet nodes images changes, controller
// sets new image for nodes' stateful set.
func TestRemoteTabletNodesImageUpdate(t *testing.T) {
	h := startHelperWithController(t, "remote-tablet-nodes-test-image-update",
		setupRemoteTabletNodesReconciler(),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteTabletNodes(h, remoteYtsaurusName, remoteTabletNodesName)
	testutil.DeployObject(h, &nodes)
	waitRemoteTabletNodesDeployed(h, remoteTabletNodesName)

	testutil.FetchEventually(h, statefulSetNameTabletNodes, &appsv1.StatefulSet{})

	updatedImage := testYtsaurusImage + "-changed"
	nodes.Spec.Image = &updatedImage
	testutil.UpdateObject(h, &ytv1.RemoteTabletNodes{}, &nodes)

	testutil.FetchAndCheckEventually(
		h,
		statefulSetNameTabletNodes,
		&appsv1.StatefulSet{},
		"image updated in sts spec",
		func(obj client.Object) bool {
			sts := obj.(*appsv1.StatefulSet)
			return sts.Spec.Template.Spec.Containers[0].Image == updatedImage
		},
	)
}

// TestRemoteTabletNodesChangeInstanceCount ensures that if remote nodes instance count changed in spec,
// it is reflected in stateful set spec.
func TestRemoteTabletNodesChangeInstanceCount(t *testing.T) {
	h := startHelperWithController(t, "remote-tablet-nodes-test-change-instance-count",
		setupRemoteTabletNodesReconciler(),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteTabletNodes(h, remoteYtsaurusName, remoteTabletNodesName)
	testutil.DeployObject(h, &nodes)
	waitRemoteTabletNodesDeployed(h, remoteTabletNodesName)

	testutil.FetchEventually(h, statefulSetNameTabletNodes, &appsv1.StatefulSet{})

	newInstanceCount := int32(3)
	nodes.Spec.InstanceCount = newInstanceCount
	testutil.UpdateObject(h, &ytv1.RemoteTabletNodes{}, &nodes)

	testutil.FetchAndCheckEventually(
		h,
		statefulSetNameTabletNodes,
		&appsv1.StatefulSet{},
		"expected replicas count",
		func(obj client.Object) bool {
			sts := obj.(*appsv1.StatefulSet)
			return *sts.Spec.Replicas == newInstanceCount
		},
	)
}

// TestRemoteTabletNodesStatusRunningZeroPods ensures that remote tablet nodes CRD reaches correct release status
// in zero pods case.
func TestRemoteTabletNodesStatusRunningZeroPods(t *testing.T) {
	h := startHelperWithController(t, "remote-tablet-nodes-test-status-running-zero-pods",
		setupRemoteTabletNodesReconciler(),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteTabletNodes(h, remoteYtsaurusName, remoteTabletNodesName)
	testutil.DeployObject(h, &nodes)
	waitRemoteTabletNodesDeployed(h, remoteTabletNodesName)

	testutil.FetchAndCheckEventually(
		h,
		remoteTabletNodesName,
		&ytv1.RemoteTabletNodes{},
		"remote nodes status running",
		func(obj client.Object) bool {
			remoteNodes := obj.(*ytv1.RemoteTabletNodes)
			return remoteNodes.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusRunning
		},
	)
}

// TestRemoteTabletNodesStatusRunningZeroPods ensures that remote tablet nodes CRD reaches correct release status
// in non-zero pods case.
func TestRemoteTabletNodesStatusRunningWithPods(t *testing.T) {
	h := startHelperWithController(t, "remote-tablet-nodes-test-status-running-with-pods",
		setupRemoteTabletNodesReconciler(),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)

	// For some reason ArePodsReady check is ok with having zero pods while MinReadyInstanceCount = 1,
	// so here we are creating pending pods before remote tablet nodes deploy to obtain Pending status for test purposes.
	// Will investigate and possibly fix ArePodsReady behaviour later.
	pod := buildTabletNodePod(h)
	testutil.DeployObject(h, &pod)

	nodes := buildRemoteTabletNodes(h, remoteYtsaurusName, remoteTabletNodesName)
	nodes.Spec.InstanceSpec.InstanceCount = 1
	nodes.Spec.InstanceSpec.MinReadyInstanceCount = ptr.To(1)
	testutil.DeployObject(h, &nodes)
	waitRemoteTabletNodesDeployed(h, remoteTabletNodesName)

	testutil.FetchAndCheckEventually(
		h,
		remoteTabletNodesName,
		&ytv1.RemoteTabletNodes{},
		"remote tablet nodes status pending",
		func(obj client.Object) bool {
			remoteNodes := obj.(*ytv1.RemoteTabletNodes)
			return remoteNodes.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusPending
		},
	)

	pod.Status.Phase = corev1.PodRunning
	err := h.GetK8sClient().Status().Update(context.Background(), &pod)
	require.NoError(t, err)

	testutil.FetchAndCheckEventually(
		h,
		remoteTabletNodesName,
		&ytv1.RemoteTabletNodes{},
		"remote nodes status running",
		func(obj client.Object) bool {
			remoteNodes := obj.(*ytv1.RemoteTabletNodes)
			return remoteNodes.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusRunning
		},
	)
}

func buildRemoteTabletNodes(h *testutil.TestHelper, remoteYtsaurusName, remoteTabletNodesName string) ytv1.RemoteTabletNodes {
	return ytv1.RemoteTabletNodes{
		ObjectMeta: h.GetObjectMeta(remoteTabletNodesName),
		Spec: ytv1.RemoteTabletNodesSpec{
			RemoteClusterSpec: &(corev1.LocalObjectReference{
				Name: remoteYtsaurusName,
			}),
			TabletNodesSpec: ytv1.TabletNodesSpec{
				InstanceSpec: ytv1.InstanceSpec{
					Image: ptr.To(testYtsaurusImage),
					Locations: []ytv1.LocationSpec{
						{
							LocationType: ytv1.LocationTypeChunkStore,
							Path:         "/yt/hdd1/chunk-store",
						},
						{
							LocationType: ytv1.LocationTypeChunkStore,
							Path:         "/yt/hdd2/chunk-store",
						},
					},
				},
			},
		},
	}
}

func buildTabletNodePod(h *testutil.TestHelper) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dn-0",
			Namespace: h.Namespace,
			Labels: map[string]string{
				consts.YTComponentLabelName: remoteTabletNodesName + "-" + consts.ComponentLabel(consts.TabletNodeType),
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "main-container",
				Image: testYtsaurusImage,
			}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodPending},
	}
}

func waitRemoteTabletNodesDeployed(h *testutil.TestHelper, remoteTabletNodesName string) {
	testutil.FetchEventually(h, remoteTabletNodesName, &ytv1.RemoteTabletNodes{})
}
