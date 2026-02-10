package controllers_test

import (
	"context"
	"strings"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/controllers"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

const (
	remoteExecNodesName      = "test-remote-exec-nodes"
	statefulSetNameExecNodes = "end-test-remote-exec-nodes"
	execNodeConfigMapName    = "yt-exec-node-config"
	execNodeConfigMapYsonKey = "ytserver-exec-node.yson"
)

var _ = Describe("RemoteExecNodes Controller", func() {
	var h *testutil.TestHelper
	var namespace string
	var reconcilerSetupFunc func(mgr ctrl.Manager) error

	JustBeforeEach(func() {
		h = testutil.NewTestHelper(GinkgoTB(), namespace, "..")
		h.Start(reconcilerSetupFunc)
	})

	BeforeEach(func() {
		reconcilerSetupFunc = func(mgr ctrl.Manager) error {
			return (&controllers.RemoteExecNodesReconciler{
				BaseReconciler: controllers.BaseReconciler{
					ClusterDomain: "cluster.local",
					Client:        mgr.GetClient(),
					Scheme:        mgr.GetScheme(),
					Recorder:      mgr.GetEventRecorderFor("remoteexecnodes-controller"),
				},
			}).SetupWithManager(mgr)
		}
	})

	Describe("RemoteExecNodes operations", func() {
		Context("When creating remote exec nodes from scratch", func() {
			BeforeEach(func() {
				namespace = "remote-exec-nodes-test-from-scratch"
			})

			It("should create resources with correct connection to remote Ytsaurus", func(ctx context.Context) {

				remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
				testutil.DeployObject(h, &remoteYtsaurusSpec)
				waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
				nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
				testutil.DeployObject(h, &nodes)
				waitRemoteExecNodesDeployed(h, remoteExecNodesName)

				testutil.FetchEventually(h, statefulSetNameExecNodes, &appsv1.StatefulSet{})

				ysonNodeConfig := testutil.FetchConfigMapData(h, execNodeConfigMapName, execNodeConfigMapYsonKey)
				Expect(ysonNodeConfig).NotTo(BeEmpty())
				Expect(ysonNodeConfig).To(ContainSubstring(remoteYtsaurusHostname))
			})
		})

		Context("When remote Ytsaurus changes hostnames", func() {
			BeforeEach(func() {
				namespace = "remote-exec-nodes-test-host-change"
			})

			It("should update remote nodes configs accordingly", func(ctx context.Context) {

				remoteYtsaurus := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
				testutil.DeployObject(h, &remoteYtsaurus)
				waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
				nodes := buildRemoteExecNodes(h, remoteYtsaurusName, remoteExecNodesName)
				testutil.DeployObject(h, &nodes)
				waitRemoteExecNodesDeployed(h, remoteExecNodesName)

				ysonNodeConfig := testutil.FetchConfigMapData(h, execNodeConfigMapName, execNodeConfigMapYsonKey)
				Expect(ysonNodeConfig).NotTo(BeEmpty())
				Expect(ysonNodeConfig).To(ContainSubstring(remoteYtsaurusHostname))

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
			})
		})

		Context("When updating image", func() {
			BeforeEach(func() {
				namespace = "remote-exec-nodes-test-image-update"
			})

			It("should set new image for nodes' stateful set", func(ctx context.Context) {

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
			})
		})

		Context("When changing instance count", func() {
			BeforeEach(func() {
				namespace = "remote-exec-nodes-test-change-instance-count"
			})

			It("should reflect change in stateful set spec", func(ctx context.Context) {

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
			})
		})

		Context("When checking status with zero pods", func() {
			BeforeEach(func() {
				namespace = "remote-exec-nodes-test-status-running-zero-pods"
			})

			It("should reach correct running status", func(ctx context.Context) {

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
						return remoteNodes.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusRunning
					},
				)
			})
		})

		Context("When checking status with pods", func() {
			BeforeEach(func() {
				namespace = "remote-exec-nodes-test-status-running-with-pods"
			})

			It("should reach correct running status after pods are ready", func(ctx context.Context) {

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
						return remoteNodes.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusPending
					},
				)

				pod.Status.Phase = corev1.PodRunning
				err := h.GetK8sClient().Status().Update(ctx, &pod)
				Expect(err).NotTo(HaveOccurred())

				testutil.FetchAndCheckEventually(
					h,
					remoteExecNodesName,
					&ytv1.RemoteExecNodes{},
					"remote nodes status running",
					func(obj client.Object) bool {
						remoteNodes := obj.(*ytv1.RemoteExecNodes)
						return remoteNodes.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusRunning
					},
				)
			})
		})
	})
})

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
				consts.YTComponentLabelName: remoteExecNodesName + "-" + consts.ComponentLabel(consts.ExecNodeType),
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
