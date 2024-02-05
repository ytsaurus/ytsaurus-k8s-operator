package controllers

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ptr "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

const (
	eventuallyWaitTime = 10 * time.Second
	eventuallyTickTime = 500 * time.Millisecond
)

const (
	remoteYtsaurusHostname   = "test-hostname"
	remoteYtsaurusName       = "test-remote-ytsaurus"
	remoteExecNodesName      = "test-remote-exec-nodes"
	statefulSetName          = "end-test-remote-exec-nodes"
	execNodeConfigMapName    = "yt-exec-node-config"
	execNodeConfigMapYsonKey = "ytserver-exec-node.yson"
)

var (
	testYtsaurusImage = "test-ytsaurus-image"
)

// TestRemoteExecNodesFromScratch ensures that remote exec nodes resources are
// created with correct connection to the specified remote Ytsaurus.
func TestRemoteExecNodesFromScratch(t *testing.T) {
	h := newTestHelper(t, "remote-exec-nodes-test-from-scratch")
	h.start()
	defer h.stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	deployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	deployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	fetchEventually(h, statefulSetName, &v1.StatefulSet{})

	ysonNodeConfig := fetchConfigMapData(h, execNodeConfigMapName, execNodeConfigMapYsonKey)
	require.NotEmpty(t, ysonNodeConfig)
	require.Contains(t, ysonNodeConfig, remoteYtsaurusHostname)
}

// TestRemoteExecNodesYtsaurusChanges ensures that if remote Ytsaurus CRD changes its hostnames
// remote nodes changes its configs accordingly.
func TestRemoteExecNodesYtsaurusChanges(t *testing.T) {
	h := newTestHelper(t, "remote-exec-nodes-test-host-change")
	h.start()
	defer h.stop()

	remoteYtsaurus := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	deployObject(h, &remoteYtsaurus)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	deployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	ysonNodeConfig := fetchConfigMapData(h, execNodeConfigMapName, execNodeConfigMapYsonKey)
	require.NotEmpty(t, ysonNodeConfig)
	require.Contains(t, ysonNodeConfig, remoteYtsaurusHostname)

	hostnameChanged := remoteYtsaurusHostname + "-changed"
	remoteYtsaurus.Spec.MasterConnectionSpec.HostAddresses = []string{hostnameChanged}
	updateObject(h, &ytv1.RemoteYtsaurus{}, &remoteYtsaurus)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)

	fetchAndCheckEventually(
		h,
		execNodeConfigMapName,
		&corev1.ConfigMap{},
		func(obj client.Object) bool {
			data := obj.(*corev1.ConfigMap).Data
			ysonNodeConfig = data[execNodeConfigMapYsonKey]
			return strings.Contains(ysonNodeConfig, hostnameChanged)
		},
	)
}

// TestRemoteExecNodesImageUpdate ensures that if remote exec nodes images changes controller
// sets new image for nodes' stateful set.
func TestRemoteExecNodesImageUpdate(t *testing.T) {
	h := newTestHelper(t, "remote-exec-nodes-test-image-update")
	h.start()
	defer h.stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	deployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	deployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	fetchEventually(h, statefulSetName, &v1.StatefulSet{})

	updatedImage := testYtsaurusImage + "-changed"
	nodes.Spec.Image = &updatedImage
	updateObject(h, &ytv1.RemoteExecNodes{}, &nodes)

	fetchAndCheckEventually(
		h,
		statefulSetName,
		&v1.StatefulSet{},
		func(obj client.Object) bool {
			sts := obj.(*v1.StatefulSet)
			return sts.Spec.Template.Spec.Containers[0].Image == updatedImage
		},
	)
}

// TestRemoteExecNodesChangeInstanceCount ensures that if remote nodes instance count changed in spec
// it is reflected in stateful set spec.
func TestRemoteExecNodesChangeInstanceCount(t *testing.T) {
	h := newTestHelper(t, "remote-exec-nodes-test-change-instance-count")
	h.start()
	defer h.stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	deployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	deployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	fetchEventually(h, statefulSetName, &v1.StatefulSet{})

	newInstanceCount := int32(3)
	nodes.Spec.InstanceCount = newInstanceCount
	updateObject(h, &ytv1.RemoteExecNodes{}, &nodes)

	fetchAndCheckEventually(
		h,
		statefulSetName,
		&v1.StatefulSet{},
		func(obj client.Object) bool {
			sts := obj.(*v1.StatefulSet)
			return *sts.Spec.Replicas == newInstanceCount
		},
	)
}

// TestRemoteExecNodesStatusRunningZeroPods ensures that remote exec nodes CRD reaches correct release status
// in zero pods case.
func TestRemoteExecNodesStatusRunningZeroPods(t *testing.T) {
	h := newTestHelper(t, "remote-exec-nodes-test-status-running-zero-pods")
	h.start()
	defer h.stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	deployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	deployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	fetchAndCheckEventually(
		h,
		remoteExecNodesName,
		&ytv1.RemoteExecNodes{},
		func(obj client.Object) bool {
			remoteNodes := obj.(*ytv1.RemoteExecNodes)
			return remoteNodes.Status.ReleaseStatus == ytv1.RemoteExecNodeReleaseStatusRunning
		},
	)
}

// TestRemoteExecNodesStatusRunningZeroPods ensures that remote exec nodes CRD reaches correct release status
// in non-zero pods case.
func TestRemoteExecNodesStatusRunningWithPods(t *testing.T) {
	h := newTestHelper(t, "remote-exec-nodes-test-status-running-with-pods")
	h.start()
	defer h.stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	deployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)

	// For some reason ArePodsReady check is ok with having zero pods while MinReadyInstanceCount = 1,
	// so here we are creating pending pods before remote exec nodes deploy to obtain Pending status for test purposes.
	// Will investigate and possibly fix ArePodsReady behaviour later.
	pod := buildExecNodePod(h)
	deployObject(h, &pod)

	nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
	nodes.Spec.InstanceSpec.InstanceCount = 1
	nodes.Spec.InstanceSpec.MinReadyInstanceCount = ptr.Int(1)
	deployObject(h, &nodes)
	waitRemoteExecNodesDeployed(h, remoteExecNodesName)

	fetchAndCheckEventually(
		h,
		remoteExecNodesName,
		&ytv1.RemoteExecNodes{},
		func(obj client.Object) bool {
			remoteNodes := obj.(*ytv1.RemoteExecNodes)
			return remoteNodes.Status.ReleaseStatus == ytv1.RemoteExecNodeReleaseStatusPending
		},
	)

	pod.Status.Phase = corev1.PodRunning
	err := h.getK8sClient().Status().Update(context.Background(), &pod)
	require.NoError(t, err)

	fetchAndCheckEventually(
		h,
		remoteExecNodesName,
		&ytv1.RemoteExecNodes{},
		func(obj client.Object) bool {
			remoteNodes := obj.(*ytv1.RemoteExecNodes)
			return remoteNodes.Status.ReleaseStatus == ytv1.RemoteExecNodeReleaseStatusRunning
		},
	)
}

// Helpers.
func getObject(h *testHelper, key string, emptyObject client.Object) {
	k8sCli := h.getK8sClient()
	err := k8sCli.Get(context.Background(), h.getObjectKey(key), emptyObject)
	require.NoError(h.t, err)
}
func deployObject(h *testHelper, object client.Object) {
	k8sCli := h.getK8sClient()
	err := k8sCli.Create(context.Background(), object)
	require.NoError(h.t, err)
}
func updateObject(h *testHelper, emptyObject, newObject client.Object) {
	k8sCli := h.getK8sClient()

	//key := client.ObjectKey{Name: newObject.GetName(), Namespace: newObject.GetNamespace()}
	getObject(h, newObject.GetName(), emptyObject)

	newObject.SetResourceVersion(emptyObject.GetResourceVersion())
	err := k8sCli.Update(context.Background(), newObject)
	require.NoError(h.t, err)
}

func buildRemoteYtsaurus(h *testHelper, remoteYtsaurusName, remoteYtsaurusHostname string) ytv1.RemoteYtsaurus {
	remoteYtsaurus := ytv1.RemoteYtsaurus{
		ObjectMeta: h.getObjectMeta(remoteYtsaurusName),
		Spec: ytv1.RemoteYtsaurusSpec{
			MasterConnectionSpec: ytv1.MasterConnectionSpec{
				CellTag: 100,
				HostAddresses: []string{
					remoteYtsaurusHostname,
				},
			},
		},
	}
	return remoteYtsaurus
}
func buildRemoteExecNodes(h *testHelper, remoteYtsaurusName, remoteExecNodesName string) ytv1.RemoteExecNodes {
	return ytv1.RemoteExecNodes{
		ObjectMeta: h.getObjectMeta(remoteExecNodesName),
		Spec: ytv1.RemoteExecNodesSpec{
			RemoteClusterSpec: &(corev1.LocalObjectReference{
				Name: remoteYtsaurusName,
			}),
			ExecNodesSpec: ytv1.ExecNodesSpec{
				InstanceSpec: ytv1.InstanceSpec{
					Image: &testYtsaurusImage,
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
func buildExecNodePod(h *testHelper) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "end-0",
			Namespace: h.namespace,
			Labels: map[string]string{
				consts.YTComponentLabelName: strings.Join(
					[]string{
						remoteExecNodesName,
						consts.YTComponentLabelExecNode,
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

func waitRemoteYtsaurusDeployed(h *testHelper, remoteYtsaurusName string) {
	fetchEventually(h, remoteYtsaurusName, &ytv1.RemoteYtsaurus{})
}
func waitRemoteExecNodesDeployed(h *testHelper, remoteExecNodesName string) {
	fetchEventually(h, remoteExecNodesName, &ytv1.RemoteExecNodes{})
}

func fetchEventually(h *testHelper, key string, obj client.Object) {
	h.t.Logf("start waiting for %v to be found", key)
	fetchAndCheckEventually(
		h,
		key,
		obj,
		func(obj client.Object) bool {
			return true
		})
}
func fetchAndCheckEventually(h *testHelper, key string, obj client.Object, condition func(obj client.Object) bool) {
	h.t.Logf("start waiting for %v to be found with condition", key)
	k8sCli := h.getK8sClient()
	eventually(
		h,
		func() bool {
			err := k8sCli.Get(context.Background(), h.getObjectKey(key), obj)
			if err != nil {
				if errors.IsNotFound(err) {
					h.t.Logf("object %v not found.", key)
					return false
				}
				require.NoError(h.t, err)
			}
			return condition(obj)
		},
	)
}
func eventually(h *testHelper, condition func() bool) {
	h.t.Logf("start waiting for condition")
	waitTime := eventuallyWaitTime
	// Useful when debugging test.
	waitTimeFromEnv := os.Getenv("TEST_EVENTUALLY_WAIT_TIME")
	if waitTimeFromEnv != "" {
		var err error
		waitTime, err = time.ParseDuration(waitTimeFromEnv)
		if err != nil {
			h.t.Fatalf("failed to parse TEST_EVENTUALLY_WAIT_TIME=%s", waitTimeFromEnv)
		}
	}
	require.Eventually(
		h.t,
		condition,
		waitTime,
		eventuallyTickTime,
	)
}
func fetchConfigMapData(h *testHelper, objectKey, mapKey string) string {
	configMap := corev1.ConfigMap{}
	fetchEventually(h, objectKey, &configMap)
	data := configMap.Data
	ysonNodeConfig := data[mapKey]
	return ysonNodeConfig
}
