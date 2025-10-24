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
	offshoreDataGatewaysName            = "test-offshore-data-gateways"
	statefulSetNameOffshoreDataGateways = "odg-test-offshore-data-gateways"
	offshoreDataGatewayConfigMapName    = "yt-offshore-data-gateway-config"
	offshoreDataGatewayConfigMapYsonKey = "ytserver-offshore-node-proxy.yson"
)

func setupOffshoreDataGatewaysReconciler() func(mgr ctrl.Manager) error {
	return func(mgr ctrl.Manager) error {
		return (&controllers.OffshoreDataGatewaysReconciler{
			ClusterDomain: "cluster.local",
			Client:        mgr.GetClient(),
			Scheme:        mgr.GetScheme(),
			Recorder:      mgr.GetEventRecorderFor("offshoredatagateways-controller"),
		}).SetupWithManager(mgr)
	}
}

// TestOffshoreDataGatewaysFromScratch ensures that offshore data gateways resources are
// created with correct connection to the specified remote Ytsaurus.
func TestOffshoreDataGatewaysFromScratch(t *testing.T) {
	h := startHelperWithController(t, "offshore-data-gateways-test-from-scratch",
		setupOffshoreDataGatewaysReconciler(),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	offshoreDataGateways := buildOffshoreDataGateways(h, remoteYtsaurusName, offshoreDataGatewaysName)
	testutil.DeployObject(h, &offshoreDataGateways)
	waitOffshoreDataGatewaysDeployed(h, offshoreDataGatewaysName)

	testutil.FetchEventually(h, statefulSetNameOffshoreDataGateways, &appsv1.StatefulSet{})

	ysonNodeConfig := testutil.FetchConfigMapData(h, offshoreDataGatewayConfigMapName, offshoreDataGatewayConfigMapYsonKey)
	require.NotEmpty(t, ysonNodeConfig)
	require.Contains(t, ysonNodeConfig, remoteYtsaurusHostname)
}

// TestOffshoreDataGatewaysYtsaurusChanges ensures that if remote Ytsaurus CRD changes its hostnames
// remote nodes changes its configs accordingly.
func TestOffshoreDataGatewaysYtsaurusChanges(t *testing.T) {
	h := startHelperWithController(t, "offshore-data-gateways-test-host-change",
		setupOffshoreDataGatewaysReconciler(),
	)
	defer h.Stop()

	remoteYtsaurus := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurus)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	offshoreDataGateways := buildOffshoreDataGateways(h, remoteYtsaurusName, offshoreDataGatewaysName)
	testutil.DeployObject(h, &offshoreDataGateways)
	waitOffshoreDataGatewaysDeployed(h, offshoreDataGatewaysName)

	ysonNodeConfig := testutil.FetchConfigMapData(h, offshoreDataGatewayConfigMapName, offshoreDataGatewayConfigMapYsonKey)
	require.NotEmpty(t, ysonNodeConfig)
	require.Contains(t, ysonNodeConfig, remoteYtsaurusHostname)

	hostnameChanged := remoteYtsaurusHostname + "-changed"
	remoteYtsaurus.Spec.MasterConnectionSpec.HostAddresses = []string{hostnameChanged}
	testutil.UpdateObject(h, &ytv1.RemoteYtsaurus{}, &remoteYtsaurus)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)

	testutil.FetchAndCheckEventually(
		h,
		offshoreDataGatewayConfigMapName,
		&corev1.ConfigMap{},
		"config map exists and contains changed hostname",
		func(obj client.Object) bool {
			data := obj.(*corev1.ConfigMap).Data
			ysonNodeConfig = data[offshoreDataGatewayConfigMapYsonKey]
			return strings.Contains(ysonNodeConfig, hostnameChanged)
		},
	)
}

// TestOffshoreDataGatewaysImageUpdate ensures that if offshore data gateways images changes, controller
// sets new image for nodes' stateful set.
func TestOffshoreDataGatewaysImageUpdate(t *testing.T) {
	h := startHelperWithController(t, "offshore-data-gateways-test-image-update",
		setupOffshoreDataGatewaysReconciler(),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	offshoreDataGateways := buildOffshoreDataGateways(h, remoteYtsaurusName, offshoreDataGatewaysName)
	testutil.DeployObject(h, &offshoreDataGateways)
	waitOffshoreDataGatewaysDeployed(h, offshoreDataGatewaysName)

	testutil.FetchEventually(h, statefulSetNameOffshoreDataGateways, &appsv1.StatefulSet{})

	updatedImage := testYtsaurusImage + "-changed"
	offshoreDataGateways.Spec.Image = &updatedImage
	testutil.UpdateObject(h, &ytv1.OffshoreDataGateways{}, &offshoreDataGateways)

	testutil.FetchAndCheckEventually(
		h,
		statefulSetNameOffshoreDataGateways,
		&appsv1.StatefulSet{},
		"image updated in sts spec",
		func(obj client.Object) bool {
			sts := obj.(*appsv1.StatefulSet)
			return sts.Spec.Template.Spec.Containers[0].Image == updatedImage
		},
	)
}

// TestOffshoreDataGatewaysChangeInstanceCount ensures that if remote nodes instance count changed in spec,
// it is reflected in stateful set spec.
func TestOffshoreDataGatewaysChangeInstanceCount(t *testing.T) {
	h := startHelperWithController(t, "offshore-data-gateways-test-change-instance-count",
		setupOffshoreDataGatewaysReconciler(),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	offshoreDataGateways := buildOffshoreDataGateways(h, remoteYtsaurusName, offshoreDataGatewaysName)
	testutil.DeployObject(h, &offshoreDataGateways)
	waitOffshoreDataGatewaysDeployed(h, offshoreDataGatewaysName)

	testutil.FetchEventually(h, statefulSetNameOffshoreDataGateways, &appsv1.StatefulSet{})

	newInstanceCount := int32(3)
	offshoreDataGateways.Spec.InstanceCount = newInstanceCount
	testutil.UpdateObject(h, &ytv1.OffshoreDataGateways{}, &offshoreDataGateways)

	testutil.FetchAndCheckEventually(
		h,
		statefulSetNameOffshoreDataGateways,
		&appsv1.StatefulSet{},
		"expected replicas count",
		func(obj client.Object) bool {
			sts := obj.(*appsv1.StatefulSet)
			return *sts.Spec.Replicas == newInstanceCount
		},
	)
}

// TestOffshoreDataGatewaysStatusRunningZeroPods ensures that offshore data gateways CRD reaches correct release status
// in zero pods case.
func TestOffshoreDataGatewaysStatusRunningZeroPods(t *testing.T) {
	h := startHelperWithController(t, "offshore-data-gateways-test-status-running-zero-pods",
		setupOffshoreDataGatewaysReconciler(),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
	offshoreDataGateways := buildOffshoreDataGateways(h, remoteYtsaurusName, offshoreDataGatewaysName)
	testutil.DeployObject(h, &offshoreDataGateways)
	waitOffshoreDataGatewaysDeployed(h, offshoreDataGatewaysName)

	testutil.FetchAndCheckEventually(
		h,
		offshoreDataGatewaysName,
		&ytv1.OffshoreDataGateways{},
		"remote gateways status running",
		func(obj client.Object) bool {
			offshoreDataGateways := obj.(*ytv1.OffshoreDataGateways)
			return offshoreDataGateways.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusRunning
		},
	)
}

// TestOffshoreDataGatewaysStatusRunningZeroPods ensures that offshore data gateways CRD reaches correct release status
// in non-zero pods case.
func TestOffshoreDataGatewaysStatusRunningWithPods(t *testing.T) {
	h := startHelperWithController(t, "offshore-data-gateways-test-status-running-with-pods",
		setupOffshoreDataGatewaysReconciler(),
	)
	defer h.Stop()

	remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
	testutil.DeployObject(h, &remoteYtsaurusSpec)
	waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)

	// For some reason ArePodsReady check is ok with having zero pods while MinReadyInstanceCount = 1,
	// so here we are creating pending pods before offshore data gateways deploy to obtain Pending status for test purposes.
	// Will investigate and possibly fix ArePodsReady behaviour later.
	pod := buildOffshoreDataGatewayPod(h)
	testutil.DeployObject(h, &pod)

	offshoreDataGateways := buildOffshoreDataGateways(h, remoteYtsaurusName, offshoreDataGatewaysName)
	offshoreDataGateways.Spec.InstanceSpec.InstanceCount = 1
	offshoreDataGateways.Spec.InstanceSpec.MinReadyInstanceCount = ptr.To(1)
	testutil.DeployObject(h, &offshoreDataGateways)
	waitOffshoreDataGatewaysDeployed(h, offshoreDataGatewaysName)

	testutil.FetchAndCheckEventually(
		h,
		offshoreDataGatewaysName,
		&ytv1.OffshoreDataGateways{},
		"offshore data gateways status pending",
		func(obj client.Object) bool {
			offshoreDataGateways := obj.(*ytv1.OffshoreDataGateways)
			return offshoreDataGateways.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusPending
		},
	)

	pod.Status.Phase = corev1.PodRunning
	err := h.GetK8sClient().Status().Update(context.Background(), &pod)
	require.NoError(t, err)

	testutil.FetchAndCheckEventually(
		h,
		offshoreDataGatewaysName,
		&ytv1.OffshoreDataGateways{},
		"remote nodes status running",
		func(obj client.Object) bool {
			offshoreDataGateways := obj.(*ytv1.OffshoreDataGateways)
			return offshoreDataGateways.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusRunning
		},
	)
}

func buildOffshoreDataGateways(h *testutil.TestHelper, remoteYtsaurusName, offshoreDataGatewaysName string) ytv1.OffshoreDataGateways {
	return ytv1.OffshoreDataGateways{
		ObjectMeta: h.GetObjectMeta(offshoreDataGatewaysName),
		Spec: ytv1.OffshoreDataGatewaysSpec{
			RemoteClusterSpec: &(corev1.LocalObjectReference{
				Name: remoteYtsaurusName,
			}),
			OffshoreDataGatewaySpec: ytv1.OffshoreDataGatewaySpec{
				InstanceSpec: ytv1.InstanceSpec{
					Image: ptr.To(testYtsaurusImage),
				},
			},
		},
	}
}

func buildOffshoreDataGatewayPod(h *testutil.TestHelper) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dn-0",
			Namespace: h.Namespace,
			Labels: map[string]string{
				consts.YTComponentLabelName: offshoreDataGatewaysName + "-" + consts.ComponentLabel(consts.OffshoreDataGatewayType),
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

func waitOffshoreDataGatewaysDeployed(h *testutil.TestHelper, offshoreDataGatewaysName string) {
	testutil.FetchEventually(h, offshoreDataGatewaysName, &ytv1.OffshoreDataGateways{})
}
