package components

import (
	"context"
	"fmt"
	"net"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.ytsaurus.tech/yt/go/guid"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	mock_yt "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/mock"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

var _ = Describe("Tablet node test", func() {
	var ytsaurusSpec *ytv1.Ytsaurus
	ytsaurusName := "ytsaurus"
	namespace := "default"
	var mockYtClient *mock_yt.MockClient

	var client client.WithWatch

	BeforeEach(func() {
		mockYtClient = mock_yt.NewMockClient(mockCtrl)

		masterVolumeSize, _ := resource.ParseQuantity("1Gi")

		ytsaurusSpec = &ytv1.Ytsaurus{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Ytsaurus",
				APIVersion: "cluster.ytsaurus.tech/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      ytsaurusName,
				Namespace: namespace,
			},
			Spec: ytv1.YtsaurusSpec{
				CommonSpec: ytv1.CommonSpec{
					CoreImage: "ytsaurus/ytsaurus:latest",
				},
				Discovery: ytv1.DiscoverySpec{
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 1,
					},
				},
				PrimaryMasters: ytv1.MastersSpec{
					MasterConnectionSpec: ytv1.MasterConnectionSpec{
						CellTag: 1,
					},
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 1,
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
						Volumes: []corev1.Volume{
							{
								Name: "master-data",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{
										SizeLimit: &masterVolumeSize,
									},
								},
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "master-data",
								MountPath: "/yt/master-data",
							},
						},
					},
				},
				HTTPProxies: []ytv1.HTTPProxiesSpec{
					{
						ServiceType: "NodePort",
						InstanceSpec: ytv1.InstanceSpec{
							InstanceCount: 1,
						},
					},
				},
				UI: &ytv1.UISpec{
					InstanceCount: 1,
				},
				DataNodes: []ytv1.DataNodesSpec{
					{
						InstanceSpec: ytv1.InstanceSpec{
							InstanceCount: 1,
							Locations: []ytv1.LocationSpec{
								{
									LocationType: "ChunkStore",
									Path:         "/yt/node-data/chunk-store",
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "node-data",
									VolumeSource: corev1.VolumeSource{
										EmptyDir: &corev1.EmptyDirVolumeSource{
											SizeLimit: &masterVolumeSize,
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "node-data",
									MountPath: "/yt/node-data",
								},
							},
						},
					},
				},
				TabletNodes: []ytv1.TabletNodesSpec{
					{
						InstanceSpec: ytv1.InstanceSpec{
							InstanceCount: 1,
						},
					},
				},
			},
		}
	})

	Context("Default context", func() {
		var scheme *runtime.Scheme

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(ytv1.AddToScheme(scheme)).To(Succeed())
			Expect(corev1.AddToScheme(scheme)).To(Succeed())

			client = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(ytsaurusSpec).
				Build()
		})

		It("Check Ytsaurus spec", func() {
			ytsaurusSpecCopy := &ytv1.Ytsaurus{}
			ytsaurusLookupKey := types.NamespacedName{Name: ytsaurusName, Namespace: namespace}
			Expect(client.Get(context.Background(), ytsaurusLookupKey, ytsaurusSpecCopy)).Should(Succeed())
			Expect(ytsaurusSpecCopy).Should(Equal(ytsaurusSpec))
		})

		It("Tablet node Sync; ytclient not ready", func() {
			cfgen := ytconfig.NewLocalNodeGenerator(ytsaurusSpec, "cluster_domain")
			ytsaurus := apiproxy.NewYtsaurus(ytsaurusSpec, client, record.NewFakeRecorder(1), scheme)

			ytsaurusClient := NewFakeYtsaurusClient(mockYtClient)

			ytsaurusClient.SetStatus(SimpleStatus(SyncStatusPending))

			tabletNode := NewTabletNode(cfgen, ytsaurus, ytsaurusClient, ytsaurusSpec.Spec.TabletNodes[0], true)
			tabletNode.server = NewFakeServer()
			status, err := tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusBlocked))

			ytsaurusClient.SetStatus(SimpleStatus(SyncStatusReady))

			status, err = tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusPending))
		})

		It("Tablet node Sync; pods are not ready", func() {
			cfgen := ytconfig.NewLocalNodeGenerator(ytsaurusSpec, "cluster_domain")
			ytsaurus := apiproxy.NewYtsaurus(ytsaurusSpec, client, record.NewFakeRecorder(1), scheme)

			ytsaurusClient := NewFakeYtsaurusClient(mockYtClient)
			tabletNode := NewTabletNode(cfgen, ytsaurus, ytsaurusClient, ytsaurusSpec.Spec.TabletNodes[0], true)
			fakeServer := NewFakeServer()
			fakeServer.podsReady = false
			tabletNode.server = fakeServer

			status, err := tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusBlocked))

			fakeServer.podsReady = true

			status, err = tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusPending))
		})

		It("Tablet node Sync; yt errors", func() {
			ytsaurus := apiproxy.NewYtsaurus(ytsaurusSpec, client, record.NewFakeRecorder(1), scheme)

			ytsaurusClient := NewFakeYtsaurusClient(mockYtClient)

			existsNetError := net.UnknownNetworkError("exists: some net error")
			createBundleNetError := net.UnknownNetworkError("create bundle: some net error")
			getNetError := net.UnknownNetworkError("get: some net error")
			createCellNetError := net.UnknownNetworkError("create cell: some net error")

			nodeCfgen := ytconfig.NewLocalNodeGenerator(ytsaurusSpec, "cluster_domain")
			tabletNode := NewTabletNode(nodeCfgen, ytsaurus, ytsaurusClient, ytsaurusSpec.Spec.TabletNodes[0], true)
			tabletNode.server = NewFakeServer()

			By("Failed to check if there is //sys/tablet_cell_bundles/sys.")
			gomock.InOrder(
				mockYtClient.EXPECT().
					NodeExists(
						gomock.Any(),
						gomock.Any(),
						gomock.Nil()).
					Return(false, existsNetError),
			)
			_, err := tabletNode.Sync(context.Background(), false)
			Expect(err).Should(Equal(existsNetError))
			status, err := tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusPending))

			By("Failed to create `sys` bundle.")
			gomock.InOrder(
				mockYtClient.EXPECT().
					NodeExists(
						gomock.Any(),
						gomock.Any(),
						gomock.Nil()).
					Return(false, nil).Times(1),
				mockYtClient.EXPECT().
					CreateObject(
						gomock.Any(),
						gomock.Eq(yt.NodeTabletCellBundle),
						gomock.Any()).
					Return(yt.NodeID(guid.New()), createBundleNetError),
			)
			_, err = tabletNode.Sync(context.Background(), false)
			Expect(err).Should(Equal(createBundleNetError))
			status, err = tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusPending))

			By("Failed to get @tablet_cell_count of the `sys` bundle.")
			gomock.InOrder(
				mockYtClient.EXPECT().
					NodeExists(
						gomock.Any(),
						gomock.Any(),
						gomock.Nil()).
					Return(false, nil).Times(1),
				mockYtClient.EXPECT().
					CreateObject(
						gomock.Any(),
						gomock.Eq(yt.NodeTabletCellBundle),
						gomock.Any()).
					Return(yt.NodeID(guid.New()), nil).Times(1),
				mockYtClient.EXPECT().
					GetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path("//sys/tablet_cell_bundles/default/@options")),
						gomock.Any(),
						gomock.Nil()).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					SetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path("//sys/tablet_cell_bundles/default/@options")),
						gomock.Any(),
						gomock.Nil()).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					GetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s/@tablet_cell_count", "default"))),
						gomock.Any(),
						nil).
					Return(getNetError),
			)
			_, err = tabletNode.Sync(context.Background(), false)
			Expect(err).Should(Equal(getNetError))
			status, err = tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusPending))

			By("Failed to create tablet_cell in the `sys` bundle.")
			gomock.InOrder(
				mockYtClient.EXPECT().
					NodeExists(
						gomock.Any(),
						gomock.Any(),
						gomock.Nil()).
					Return(true, nil).Times(1),
				mockYtClient.EXPECT().
					GetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path("//sys/tablet_cell_bundles/default/@options")),
						gomock.Any(),
						gomock.Nil()).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					SetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path("//sys/tablet_cell_bundles/default/@options")),
						gomock.Any(),
						gomock.Nil()).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					GetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s/@tablet_cell_count", "default"))),
						gomock.Any(),
						nil).
					SetArg(2, 0).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					CreateObject(
						gomock.Any(),
						gomock.Eq(yt.NodeType("tablet_cell")),
						gomock.Any()).
					Return(yt.NodeID(guid.New()), createCellNetError),
			)
			_, err = tabletNode.Sync(context.Background(), false)
			Expect(err).Should(Equal(createCellNetError))
			status, err = tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusPending))
			gomock.InOrder()

			By("Failed to get @tablet_cell_count of the `default` bundle.")
			gomock.InOrder(
				mockYtClient.EXPECT().
					NodeExists(
						gomock.Any(),
						gomock.Any(),
						gomock.Nil()).
					Return(true, nil).Times(1),
				mockYtClient.EXPECT().
					GetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path("//sys/tablet_cell_bundles/default/@options")),
						gomock.Any(),
						gomock.Nil()).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					SetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path("//sys/tablet_cell_bundles/default/@options")),
						gomock.Any(),
						gomock.Nil()).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					GetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s/@tablet_cell_count", "default"))),
						gomock.Any(),
						nil).
					SetArg(2, 0).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					CreateObject(
						gomock.Any(),
						gomock.Eq(yt.NodeType("tablet_cell")),
						gomock.Any()).
					Return(yt.NodeID(guid.New()), nil).Times(1),
				mockYtClient.EXPECT().
					GetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s/@tablet_cell_count", "sys"))),
						gomock.Any(),
						nil).
					Return(getNetError).Times(1),
			)
			_, err = tabletNode.Sync(context.Background(), false)
			Expect(err).Should(Equal(getNetError))
			status, err = tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusPending))

			By("Then everything was successfully.")
			gomock.InOrder(
				mockYtClient.EXPECT().
					NodeExists(
						gomock.Any(),
						gomock.Any(),
						gomock.Nil()).
					Return(true, nil).Times(1),
				mockYtClient.EXPECT().
					GetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path("//sys/tablet_cell_bundles/default/@options")),
						gomock.Any(),
						gomock.Nil()).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					SetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path("//sys/tablet_cell_bundles/default/@options")),
						gomock.Any(),
						gomock.Nil()).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					GetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s/@tablet_cell_count", "default"))),
						gomock.Any(),
						nil).
					SetArg(2, 1).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					GetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s/@tablet_cell_count", "sys"))),
						gomock.Any(),
						nil).
					SetArg(2, 0).
					Return(nil).Times(1),
				mockYtClient.EXPECT().
					CreateObject(
						gomock.Any(),
						gomock.Eq(yt.NodeType("tablet_cell")),
						gomock.Any()).
					Return(yt.NodeID(guid.New()), nil).Times(1),
			)
			_, err = tabletNode.Sync(context.Background(), false)
			Expect(err).Should(Succeed())
			status, err = tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusReady))
		})

		It("Tablet node Sync; success", func() {
			ytsaurus := apiproxy.NewYtsaurus(ytsaurusSpec, client, record.NewFakeRecorder(1), scheme)

			ytsaurusClient := NewFakeYtsaurusClient(mockYtClient)

			mockYtClient.EXPECT().
				NodeExists(
					gomock.Any(),
					gomock.Eq(ypath.Path("//sys/tablet_cell_bundles/sys")),
					gomock.Nil()).
				Return(false, nil)

			mockYtClient.EXPECT().
				CreateObject(
					gomock.Any(),
					gomock.Eq(yt.NodeTabletCellBundle),
					gomock.Eq(&yt.CreateObjectOptions{
						Attributes: map[string]interface{}{
							"name": "sys",
							"options": map[string]any{
								"changelog_account":            "sys",
								"snapshot_account":             "sys",
								"changelog_replication_factor": 1,
								"changelog_read_quorum":        1,
								"changelog_write_quorum":       1,
								"snapshot_replication_factor":  1,
							},
						}})).
				Return(yt.NodeID(guid.New()), nil)

			mockYtClient.EXPECT().
				GetNode(
					gomock.Any(),
					gomock.Eq(ypath.Path("//sys/tablet_cell_bundles/default/@options")),
					gomock.Any(),
					gomock.Nil()).
				Return(nil)

			mockYtClient.EXPECT().
				SetNode(
					gomock.Any(),
					gomock.Eq(ypath.Path("//sys/tablet_cell_bundles/default/@options")),
					gomock.Any(),
					gomock.Nil()).
				Return(nil)

			for _, bundle := range []string{"default", "sys"} {
				mockYtClient.EXPECT().
					GetNode(
						gomock.Any(),
						gomock.Eq(ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s/@tablet_cell_count", bundle))),
						gomock.Any(),
						nil).
					SetArg(2, 0).
					Return(nil)

				mockYtClient.EXPECT().
					CreateObject(
						gomock.Any(),
						gomock.Eq(yt.NodeType("tablet_cell")),
						gomock.Eq(&yt.CreateObjectOptions{
							Attributes: map[string]interface{}{
								"tablet_cell_bundle": bundle,
							}})).Return(yt.NodeID(guid.New()), nil)
			}

			nodeCfgen := ytconfig.NewLocalNodeGenerator(ytsaurusSpec, "cluster_domain")
			tabletNode := NewTabletNode(nodeCfgen, ytsaurus, ytsaurusClient, ytsaurusSpec.Spec.TabletNodes[0], true)
			tabletNode.server = NewFakeServer()
			_, err := tabletNode.Sync(context.Background(), false)
			Expect(err).Should(Succeed())

			status, err := tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusReady))
		})

		It("Tablet node Sync; no initialization", func() {
			ytsaurus := apiproxy.NewYtsaurus(ytsaurusSpec, client, record.NewFakeRecorder(1), scheme)

			ytsaurusClient := NewFakeYtsaurusClient(mockYtClient)

			cfgen := ytconfig.NewLocalNodeGenerator(ytsaurusSpec, "cluster_domain")
			tabletNode := NewTabletNode(cfgen, ytsaurus, ytsaurusClient, ytsaurusSpec.Spec.TabletNodes[0], false)
			tabletNode.server = NewFakeServer()
			_, err := tabletNode.Sync(context.Background(), false)
			Expect(err).Should(Succeed())

			status, err := tabletNode.Sync(context.Background(), true)
			Expect(err).Should(Succeed())
			Expect(status.SyncStatus).Should(Equal(SyncStatusReady))
		})
	})
})
