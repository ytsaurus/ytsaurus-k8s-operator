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
	offshoreDataGatewaysName            = "test-offshore-data-gateways"
	statefulSetNameOffshoreDataGateways = "odg-test-offshore-data-gateways"
	offshoreDataGatewayConfigMapName    = "yt-offshore-data-gateway-config"
	offshoreDataGatewayConfigMapYsonKey = "ytserver-offshore-data-gateway.yson"
)

var _ = Describe("OffshoreDataGateways Controller", func() {
	var h *testutil.TestHelper
	var namespace string
	var reconcilerSetupFunc func(mgr ctrl.Manager) error

	JustBeforeEach(func() {
		h = testutil.NewTestHelper(GinkgoTB(), namespace, "..")
		h.Start(reconcilerSetupFunc)
	})

	BeforeEach(func() {
		reconcilerSetupFunc = func(mgr ctrl.Manager) error {
			return (&controllers.OffshoreDataGatewaysReconciler{
				BaseReconciler: controllers.BaseReconciler{
					ClusterDomain: "cluster.local",
					Client:        mgr.GetClient(),
					Scheme:        mgr.GetScheme(),
					Recorder:      mgr.GetEventRecorderFor("offshoredatagateways-controller"),
				},
			}).SetupWithManager(mgr)
		}
	})

	Describe("OffshoreDataGateways operations", func() {
		Context("When creating offshore data gateways from scratch", func() {
			BeforeEach(func() {
				namespace = "offshore-data-gateways-test-from-scratch"
			})

			It("should create resources with correct connection to remote Ytsaurus", func(ctx context.Context) {

				remoteYtsaurusSpec := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
				testutil.DeployObject(h, &remoteYtsaurusSpec)
				waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
				offshoreDataGateways := buildOffshoreDataGateways(h, remoteYtsaurusName, offshoreDataGatewaysName)
				testutil.DeployObject(h, &offshoreDataGateways)
				waitOffshoreDataGatewaysDeployed(h, offshoreDataGatewaysName)

				testutil.FetchEventually(h, statefulSetNameOffshoreDataGateways, &appsv1.StatefulSet{})

				ysonNodeConfig := testutil.FetchConfigMapData(h, offshoreDataGatewayConfigMapName, offshoreDataGatewayConfigMapYsonKey)
				Expect(ysonNodeConfig).NotTo(BeEmpty())
				Expect(ysonNodeConfig).To(ContainSubstring(remoteYtsaurusHostname))
			})
		})

		Context("When remote Ytsaurus changes hostnames", func() {
			BeforeEach(func() {
				namespace = "offshore-data-gateways-test-host-change"
			})

			It("should update remote nodes configs accordingly", func(ctx context.Context) {

				remoteYtsaurus := buildRemoteYtsaurus(h, remoteYtsaurusName, remoteYtsaurusHostname)
				testutil.DeployObject(h, &remoteYtsaurus)
				waitRemoteYtsaurusDeployed(h, remoteYtsaurusName)
				offshoreDataGateways := buildOffshoreDataGateways(h, remoteYtsaurusName, offshoreDataGatewaysName)
				testutil.DeployObject(h, &offshoreDataGateways)
				waitOffshoreDataGatewaysDeployed(h, offshoreDataGatewaysName)

				ysonNodeConfig := testutil.FetchConfigMapData(h, offshoreDataGatewayConfigMapName, offshoreDataGatewayConfigMapYsonKey)
				Expect(ysonNodeConfig).NotTo(BeEmpty())
				Expect(ysonNodeConfig).To(ContainSubstring(remoteYtsaurusHostname))

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
			})
		})

		Context("When updating image", func() {
			BeforeEach(func() {
				namespace = "offshore-data-gateways-test-image-update"
			})

			It("should set new image for nodes' stateful set", func(ctx context.Context) {

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
			})
		})

		Context("When changing instance count", func() {
			BeforeEach(func() {
				namespace = "offshore-data-gateways-test-change-instance-count"
			})

			It("should reflect change in stateful set spec", func(ctx context.Context) {

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
			})
		})

		Context("When checking status with zero pods", func() {
			BeforeEach(func() {
				namespace = "offshore-data-gateways-test-status-running-zero-pods"
			})

			It("should reach correct running status", func(ctx context.Context) {

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
			})
		})

		Context("When checking status with pods", func() {
			BeforeEach(func() {
				namespace = "offshore-data-gateways-test-status-running-with-pods"
			})

			It("should reach correct running status after pods are ready", func(ctx context.Context) {

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
				err := h.GetK8sClient().Status().Update(ctx, &pod)
				Expect(err).NotTo(HaveOccurred())

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
			})
		})
	})
})

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
