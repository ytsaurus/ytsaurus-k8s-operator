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
	remoteExecNodesName      = "test-remote-exec-nodes"
	statefulSetNameExecNodes = "end-test-remote-exec-nodes"
	execNodeConfigMapName    = "yt-exec-node-config"
	execNodeConfigMapYsonKey = "ytserver-exec-node.yson"
)

func setupRemoteExecNodesReconciler(controllerName string) func(mgr ctrl.Manager) error {
	return func(mgr ctrl.Manager) error {
		return (&controllers.RemoteDataNodesReconciler{
			Client:   mgr.GetClient(),
			Scheme:   mgr.GetScheme(),
			Recorder: mgr.GetEventRecorderFor(controllerName),
		}).SetupWithManager(mgr)
	}
}

// TestRemoteExecNodesFromScratch ensures that remote exec nodes resources are
// created with correct connection to the specified remote Ytsaurus.
func TestRemoteExecNodesFromScratch(t *testing.T) {
	h := startHelperWithController(t, "remote-exec-nodes-test-from-scratch",
		setupRemoteExecNodesReconciler("remoteexecnodes-controller"),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	testutil.DeployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	testutil.FetchEventually(h, statefulSetNameExecNodes, &appsv1.StatefulSet{})

	ysonNodeConfig := testutil.FetchConfigMapData(h, execNodeConfigMapName, execNodeConfigMapYsonKey)
	require.NotEmpty(t, ysonNodeConfig)
	require.Contains(t, ysonNodeConfig, remoteYtsaurusHostname)
}

// TestRemoteExecNodesYtsaurusChanges ensures that if remote Ytsaurus CRD changes its hostnames
// remote nodes changes its configs accordingly.
func TestRemoteExecNodesYtsaurusChanges(t *testing.T) {
	h := startHelperWithController(t, "remote-exec-nodes-test-host-change",
		setupRemoteExecNodesReconciler("remoteexecnodes-controller"),
	)
	defer h.Stop()

	remoteYtsaurus := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurus)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	testutil.DeployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	ysonNodeConfig := testutil.FetchConfigMapData(h, execNodeConfigMapName, execNodeConfigMapYsonKey)
	require.NotEmpty(t, ysonNodeConfig)
	require.Contains(t, ysonNodeConfig, remoteYtsaurusHostname)

	hostnameChanged := remoteYtsaurusHostname + "-changed"
	remoteYtsaurus.Spec.MasterConnectionSpec.HostAddresses = []string{hostnameChanged}
	testutil.UpdateObject(h, &ytv1.RemoteYtsaurus{}, &remoteYtsaurus)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)

	testutil.FetchAndCheckEventually(
		h,
		execNodeConfigMapName,
		&corev1.ConfigMap{},
		"config map exists and contains changed hostname",
		func(obj client.Object) bool {
			data := obj.(*corev1.ConfigMap).Data
			ysonNodeConfig = data[execNodeConfigMapYsonKey]
			return strings.Contains(ysonNodeConfig, hostnameChanged)
		},
	)
}

// TestRemoteExecNodesImageUpdate ensures that if remote exec nodes images changes, controller
// sets new image for nodes' stateful set.
func TestRemoteExecNodesImageUpdate(t *testing.T) {
	h := startHelperWithController(t, "remote-exec-nodes-test-image-update",
		setupRemoteExecNodesReconciler("remoteexecnodes-controller"),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	testutil.DeployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	testutil.FetchEventually(h, statefulSetNameExecNodes, &appsv1.StatefulSet{})

	updatedImage := testYtsaurusImage + "-changed"
	nodes.Spec.Image = &updatedImage
	testutil.UpdateObject(h, &ytv1.RemoteExecNodes{}, &nodes)

	testutil.FetchAndCheckEventually(
		h,
		statefulSetNameExecNodes,
		&appsv1.StatefulSet{},
		"image updated in sts spec",
		func(obj client.Object) bool {
			sts := obj.(*appsv1.StatefulSet)
			return sts.Spec.Template.Spec.Containers[0].Image == updatedImage
		},
	)
}

// TestRemoteExecNodesChangeInstanceCount ensures that if remote nodes instance count changed in spec,
// it is reflected in stateful set spec.
func TestRemoteExecNodesChangeInstanceCount(t *testing.T) {
	h := startHelperWithController(t, "remote-exec-nodes-test-change-instance-count",
		setupRemoteExecNodesReconciler("remotedatanodes-controller"),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	testutil.DeployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	testutil.FetchEventually(h, statefulSetNameExecNodes, &appsv1.StatefulSet{})

	newInstanceCount := int32(3)
	nodes.Spec.InstanceCount = newInstanceCount
	testutil.UpdateObject(h, &ytv1.RemoteExecNodes{}, &nodes)

	testutil.FetchAndCheckEventually(
		h,
		statefulSetNameExecNodes,
		&appsv1.StatefulSet{},
		"expected replicas count",
		func(obj client.Object) bool {
			sts := obj.(*appsv1.StatefulSet)
			return *sts.Spec.Replicas == newInstanceCount
		},
	)
}

// TestRemoteExecNodesStatusRunningZeroPods ensures that remote exec nodes CRD reaches correct release status
// in zero pods case.
func TestRemoteExecNodesStatusRunningZeroPods(t *testing.T) {
	h := startHelperWithController(t, "remote-exec-nodes-test-status-running-zero-pods",
		setupRemoteExecNodesReconciler("remotedatanodes-controller"),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	testutil.DeployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	testutil.FetchAndCheckEventually(
		h,
		remoteExecNodesName,
		&ytv1.RemoteExecNodes{},
		"remote nodes status running",
		func(obj client.Object) bool {
			remoteNodes := obj.(*ytv1.RemoteExecNodes)
			return remoteNodes.Status.ReleaseStatus == ytv1.RemoteExecNodeReleaseStatusRunning
		},
	)
}

// TestRemoteExecNodesStatusRunningZeroPods ensures that remote exec nodes CRD reaches correct release status
// in non-zero pods case.
func TestRemoteExecNodesStatusRunningWithPods(t *testing.T) {
	// h := startHelperWithController(t, "remote-exec-nodes-test-status-running-with-pods")
	h := startHelperWithController(t, "remote-exec-nodes-test-status-running-with-pods",
		setupRemoteExecNodesReconciler("remoteexecnodes-controller"),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)

	// For some reason ArePodsReady check is ok with having zero pods while MinReadyInstanceCount = 1,
	// so here we are creating pending pods before remote exec nodes deploy to obtain Pending status for test purposes.
	// Will investigate and possibly fix ArePodsReady behaviour later.
	pod := buildExecNodePod(h)
	testutil.DeployObject(h, &pod)

	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	nodes.Spec.InstanceSpec.InstanceCount = 1
	nodes.Spec.InstanceSpec.MinReadyInstanceCount = ptr.To(1)
	testutil.DeployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	testutil.FetchAndCheckEventually(
		h,
		remoteExecNodesName,
		&ytv1.RemoteExecNodes{},
		"remote exec nodes status pending",
		func(obj client.Object) bool {
			remoteNodes := obj.(*ytv1.RemoteExecNodes)
			return remoteNodes.Status.ReleaseStatus == ytv1.RemoteExecNodeReleaseStatusPending
		},
	)

	pod.Status.Phase = corev1.PodRunning
	err := h.GetK8sClient().Status().Update(context.Background(), &pod)
	require.NoError(t, err)

	testutil.FetchAndCheckEventually(
		h,
		remoteExecNodesName,
		&ytv1.RemoteExecNodes{},
		"remote nodes status running",
		func(obj client.Object) bool {
			remoteNodes := obj.(*ytv1.RemoteExecNodes)
			return remoteNodes.Status.ReleaseStatus == ytv1.RemoteExecNodeReleaseStatusRunning
		},
	)
}

func buildRemoteExecNodes(h *testutil.TestHelper, remoteYtsaurusName, remoteExecNodesName string) ytv1.RemoteExecNodes {
	return ytv1.RemoteExecNodes{
		ObjectMeta: h.GetObjectMeta(remoteExecNodesName),
		Spec: ytv1.RemoteExecNodesSpec{
			RemoteClusterSpec: &(corev1.LocalObjectReference{
				Name: remoteYtsaurusName,
			}),
			ExecNodesSpec: ytv1.ExecNodesSpec{
				InstanceSpec: ytv1.InstanceSpec{
					Image: ptr.To(testYtsaurusImage),
					Locations: []ytv1.LocationSpec{
						{
							LocationType: ytv1.LocationTypeChunkCache,
							Path:         "/yt/hdd1/chunk-cache",
						},
						{
							LocationType: ytv1.LocationTypeSlots,
							Path:         "/yt/hdd2/slots",
						},
					},
				},
			},
		},
	}
}

func buildExecNodePod(h *testutil.TestHelper) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "end-0",
			Namespace: h.Namespace,
			Labels: map[string]string{
				consts.YTComponentLabelName: strings.Join(
					[]string{
						remoteExecNodesName,
						consts.ComponentLabel(consts.ExecNodeType),
					},
					"-",
				),
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

func waitRemoteExecNodesDeployed(h *testutil.TestHelper, remoteExecNodesName string) {
	testutil.FetchEventually(h, remoteExecNodesName, &ytv1.RemoteExecNodes{})
}
