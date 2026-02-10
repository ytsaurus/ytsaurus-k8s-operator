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
	remoteDataNodesName      = "test-remote-data-nodes"
	statefulSetNameDataNodes = "dnd-test-remote-data-nodes"
	dataNodeConfigMapName    = "yt-data-node-config"
	dataNodeConfigMapYsonKey = "ytserver-data-node.yson"
)

var _ = Describe("RemoteDataNodes Controller", func() {
	var h *testutil.TestHelper
	var namespace string
	var reconcilerSetupFunc func(mgr ctrl.Manager) error

	JustBeforeEach(func() {
		h = testutil.NewTestHelper(GinkgoTB(), namespace, "..")
		h.Start(reconcilerSetupFunc)
	})

	BeforeEach(func() {
		reconcilerSetupFunc = func(mgr ctrl.Manager) error {
			return (&controllers.RemoteDataNodesReconciler{
				BaseReconciler: controllers.BaseReconciler{
					ClusterDomain: "cluster.local",
					Client:        mgr.GetClient(),
					Scheme:        mgr.GetScheme(),
					Recorder:      mgr.GetEventRecorderFor("remotedatanodes-controller"),
				},
			}).SetupWithManager(mgr)
		}
	})

	Describe("RemoteDataNodes operations", func() {
		Context("When creating remote data nodes from scratch", func() {
			BeforeEach(func() {
				namespace = "remote-data-nodes-test-from-scratch"
			})

			It("should create resources with correct connection to remote Ytsaurus", func(ctx context.Context) {

				remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
				testutil.DeployObject(h, &remoteYtsaurusSpec)
				waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
				nodes := buildRemoteDataNodes(h, remoteYtsaurusName, remoteDataNodesName)
				testutil.DeployObject(h, &nodes)
				waitRemoteDataNodesDeployed(h, remoteDataNodesName)

				testutil.FetchEventually(h, statefulSetNameDataNodes, &appsv1.StatefulSet{})

				ysonNodeConfig := testutil.FetchConfigMapData(h, dataNodeConfigMapName, dataNodeConfigMapYsonKey)
				Expect(ysonNodeConfig).NotTo(BeEmpty())
				Expect(ysonNodeConfig).To(ContainSubstring(remoteYtsaurusHostname))
			})
		})

		Context("When remote Ytsaurus changes hostnames", func() {
			BeforeEach(func() {
				namespace = "remote-data-nodes-test-host-change"
			})

			It("should update remote nodes configs accordingly", func(ctx context.Context) {

				remoteYtsaurus := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
				testutil.DeployObject(h, &remoteYtsaurus)
				waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
				nodes := buildRemoteDataNodes(h, remoteYtsaurusName, remoteDataNodesName)
				testutil.DeployObject(h, &nodes)
				waitRemoteDataNodesDeployed(h, remoteDataNodesName)

				ysonNodeConfig := testutil.FetchConfigMapData(h, dataNodeConfigMapName, dataNodeConfigMapYsonKey)
				Expect(ysonNodeConfig).NotTo(BeEmpty())
				Expect(ysonNodeConfig).To(ContainSubstring(remoteYtsaurusHostname))

				hostnameChanged := remoteYtsaurusHostname + "-changed"
				remoteYtsaurus.Spec.MasterConnectionSpec.HostAddresses = []string{hostnameChanged}
				testutil.UpdateObject(h, &ytv1.RemoteYtsaurus{}, &remoteYtsaurus)
				waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)

				testutil.FetchAndCheckEventually(
					h,
					dataNodeConfigMapName,
					&corev1.ConfigMap{},
					"config map exists and contains changed hostname",
					func(obj client.Object) bool {
						data := obj.(*corev1.ConfigMap).Data
						ysonNodeConfig = data[dataNodeConfigMapYsonKey]
						return strings.Contains(ysonNodeConfig, hostnameChanged)
					},
				)
			})
		})

		Context("When updating image", func() {
			BeforeEach(func() {
				namespace = "remote-data-nodes-test-image-update"
			})

			It("should set new image for nodes' stateful set", func(ctx context.Context) {

				remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
				testutil.DeployObject(h, &remoteYtsaurusSpec)
				waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
				nodes := buildRemoteDataNodes(h, remoteYtsaurusName, remoteDataNodesName)
				testutil.DeployObject(h, &nodes)
				waitRemoteDataNodesDeployed(h, remoteDataNodesName)

				testutil.FetchEventually(h, statefulSetNameDataNodes, &appsv1.StatefulSet{})

				updatedImage := testYtsaurusImage + "-changed"
				nodes.Spec.Image = &updatedImage
				testutil.UpdateObject(h, &ytv1.RemoteDataNodes{}, &nodes)

				testutil.FetchAndCheckEventually(
					h,
					statefulSetNameDataNodes,
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
				namespace = "remote-data-nodes-test-change-instance-count"
			})

			It("should reflect change in stateful set spec", func(ctx context.Context) {

				remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
				testutil.DeployObject(h, &remoteYtsaurusSpec)
				waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
				nodes := buildRemoteDataNodes(h, remoteYtsaurusName, remoteDataNodesName)
				testutil.DeployObject(h, &nodes)
				waitRemoteDataNodesDeployed(h, remoteDataNodesName)

				testutil.FetchEventually(h, statefulSetNameDataNodes, &appsv1.StatefulSet{})

				newInstanceCount := int32(3)
				nodes.Spec.InstanceCount = newInstanceCount
				testutil.UpdateObject(h, &ytv1.RemoteDataNodes{}, &nodes)

				testutil.FetchAndCheckEventually(
					h,
					statefulSetNameDataNodes,
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
				namespace = "remote-data-nodes-test-status-running-zero-pods"
			})

			It("should reach correct running status", func(ctx context.Context) {

				remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
				testutil.DeployObject(h, &remoteYtsaurusSpec)
				waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
				nodes := buildRemoteDataNodes(h, remoteYtsaurusName, remoteDataNodesName)
				testutil.DeployObject(h, &nodes)
				waitRemoteDataNodesDeployed(h, remoteDataNodesName)

				testutil.FetchAndCheckEventually(
					h,
					remoteDataNodesName,
					&ytv1.RemoteDataNodes{},
					"remote nodes status running",
					func(obj client.Object) bool {
						remoteNodes := obj.(*ytv1.RemoteDataNodes)
						return remoteNodes.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusRunning
					},
				)
			})
		})

		Context("When checking status with pods", func() {
			BeforeEach(func() {
				namespace = "remote-data-nodes-test-status-running-with-pods"
			})

			It("should reach correct running status after pods are ready", func(ctx context.Context) {

				remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
				testutil.DeployObject(h, &remoteYtsaurusSpec)
				waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)

				// For some reason ArePodsReady check is ok with having zero pods while MinReadyInstanceCount = 1,
				// so here we are creating pending pods before remote data nodes deploy to obtain Pending status for test purposes.
				// Will investigate and possibly fix ArePodsReady behaviour later.
				pod := buildDataNodePod(h)
				testutil.DeployObject(h, &pod)

				nodes := buildRemoteDataNodes(h, remoteYtsaurusName, remoteDataNodesName)
				nodes.Spec.InstanceSpec.InstanceCount = 1
				nodes.Spec.InstanceSpec.MinReadyInstanceCount = ptr.To(1)
				testutil.DeployObject(h, &nodes)
				waitRemoteDataNodesDeployed(h, remoteDataNodesName)

				testutil.FetchAndCheckEventually(
					h,
					remoteDataNodesName,
					&ytv1.RemoteDataNodes{},
					"remote data nodes status pending",
					func(obj client.Object) bool {
						remoteNodes := obj.(*ytv1.RemoteDataNodes)
						return remoteNodes.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusPending
					},
				)

				pod.Status.Phase = corev1.PodRunning
				err := h.GetK8sClient().Status().Update(ctx, &pod)
				Expect(err).NotTo(HaveOccurred())

				testutil.FetchAndCheckEventually(
					h,
					remoteDataNodesName,
					&ytv1.RemoteDataNodes{},
					"remote nodes status running",
					func(obj client.Object) bool {
						remoteNodes := obj.(*ytv1.RemoteDataNodes)
						return remoteNodes.Status.ReleaseStatus == ytv1.RemoteNodeReleaseStatusRunning
					},
				)
			})
		})
	})
})

func buildRemoteDataNodes(h *testutil.TestHelper, remoteYtsaurusName, remoteDataNodesName string) ytv1.RemoteDataNodes {
	return ytv1.RemoteDataNodes{
		ObjectMeta: h.GetObjectMeta(remoteDataNodesName),
		Spec: ytv1.RemoteDataNodesSpec{
			RemoteClusterSpec: &(corev1.LocalObjectReference{
				Name: remoteYtsaurusName,
			}),
			DataNodesSpec: ytv1.DataNodesSpec{
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

func buildDataNodePod(h *testutil.TestHelper) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dn-0",
			Namespace: h.Namespace,
			Labels: map[string]string{
				consts.YTComponentLabelName: remoteDataNodesName + "-" + consts.ComponentLabel(consts.DataNodeType),
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

func waitRemoteDataNodesDeployed(h *testutil.TestHelper, remoteDataNodesName string) {
	testutil.FetchEventually(h, remoteDataNodesName, &ytv1.RemoteDataNodes{})
}
