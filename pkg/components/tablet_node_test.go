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

	v1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	mock_yt "github.com/ytsaurus/yt-k8s-operator/pkg/mock"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

var _ = Describe("Tablet node test", func() {
	var ytsaurusSpec *v1.Ytsaurus
	ytsaurusName := "ytsaurus"
	namespace := "default"
	var mockYtClient *mock_yt.MockClient

	var client client.WithWatch

	BeforeEach(func() {
		mockYtClient = mock_yt.NewMockClient(ctrl)

		masterVolumeSize, _ := resource.ParseQuantity("1Gi")

		ytsaurusSpec = &v1.Ytsaurus{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Ytsaurus",
				APIVersion: "cluster.ytsaurus.tech/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      ytsaurusName,
				Namespace: namespace,
			},
			Spec: v1.YtsaurusSpec{
				CommonSpec: v1.CommonSpec{
					CoreImage: "ytsaurus/ytsaurus:latest",
				},
				Discovery: v1.DiscoverySpec{
					InstanceSpec: v1.InstanceSpec{
						InstanceCount: 1,
					},
				},
				PrimaryMasters: v1.MastersSpec{
					MasterConnectionSpec: v1.MasterConnectionSpec{
						CellTag: 1,
					},
					InstanceSpec: v1.InstanceSpec{
						InstanceCount: 1,
						Locations: []v1.LocationSpec{
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
				HTTPProxies: []v1.HTTPProxiesSpec{
					{
						ServiceType: "NodePort",
						InstanceSpec: v1.InstanceSpec{
							InstanceCount: 1,
						},
					},
				},
				UI: &v1.UISpec{
					InstanceCount: 1,
				},
				DataNodes: []v1.DataNodesSpec{
					{
						InstanceSpec: v1.InstanceSpec{
							InstanceCount: 1,
							Locations: []v1.LocationSpec{
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
				TabletNodes: []v1.TabletNodesSpec{
					{
						InstanceSpec: v1.InstanceSpec{
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
			Expect(v1.AddToScheme(scheme)).To(Succeed())
			Expect(corev1.AddToScheme(scheme)).To(Succeed())

			client = fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(ytsaurusSpec).
				Build()
		})

		It("Check Ytsaurus spec", func() {
			ytsaurusSpecCopy := &v1.Ytsaurus{}
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
			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusBlocked))

			ytsaurusClient.SetStatus(SimpleStatus(SyncStatusReady))

			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusPending))
		})

		It("Tablet node Sync; pods are not ready", func() {
			cfgen := ytconfig.NewLocalNodeGenerator(ytsaurusSpec, "cluster_domain")
			ytsaurus := apiproxy.NewYtsaurus(ytsaurusSpec, client, record.NewFakeRecorder(1), scheme)

			ytsaurusClient := NewFakeYtsaurusClient(mockYtClient)
			tabletNode := NewTabletNode(cfgen, ytsaurus, ytsaurusClient, ytsaurusSpec.Spec.TabletNodes[0], true)
			fakeServer := NewFakeServer()
			fakeServer.podsReady = false
			tabletNode.server = fakeServer

			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusBlocked))

			fakeServer.podsReady = true

			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusPending))
		})

		It("Tablet node Sync; yt errors", func() {
			ytsaurus := apiproxy.NewYtsaurus(ytsaurusSpec, client, record.NewFakeRecorder(1), scheme)

			ytsaurusClient := NewFakeYtsaurusClient(mockYtClient)

			existsNetError := net.UnknownNetworkError("exists: some net error")
			createBundleNetError := net.UnknownNetworkError("create bundle: some net error")
			getNetError := net.UnknownNetworkError("get: some net error")
			createCellNetError := net.UnknownNetworkError("create cell: some net error")
			gomock.InOrder(
				mockYtClient.EXPECT().
					NodeExists(
						gomock.Any(),
						gomock.Any(),
						gomock.Nil()).
					Return(false, existsNetError),
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
						gomock.Eq(ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s/@tablet_cell_count", "default"))),
						gomock.Any(),
						nil).
					Return(getNetError),
				mockYtClient.EXPECT().
					NodeExists(
						gomock.Any(),
						gomock.Any(),
						gomock.Nil()).
					Return(true, nil).Times(1),
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
				mockYtClient.EXPECT().
					NodeExists(
						gomock.Any(),
						gomock.Any(),
						gomock.Nil()).
					Return(true, nil).Times(1),
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
				mockYtClient.EXPECT().
					NodeExists(
						gomock.Any(),
						gomock.Any(),
						gomock.Nil()).
					Return(true, nil).Times(1),
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

			nodeCfgen := ytconfig.NewLocalNodeGenerator(ytsaurusSpec, "cluster_domain")
			tabletNode := NewTabletNode(nodeCfgen, ytsaurus, ytsaurusClient, ytsaurusSpec.Spec.TabletNodes[0], true)
			tabletNode.server = NewFakeServer()

			// Failed to check if there is //sys/tablet_cell_bundles/sys.
			err := tabletNode.Sync(context.Background())
			Expect(err).Should(MatchError(existsNetError))
			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusPending))

			// Failed to create `sys` bundle.
			err = tabletNode.Sync(context.Background())
			Expect(err).Should(MatchError(createBundleNetError))
			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusPending))

			// Failed to get @tablet_cell_count of the `sys` bundle.
			err = tabletNode.Sync(context.Background())
			Expect(err).Should(MatchError(getNetError))
			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusPending))

			// Failed to create tablet_cell in the `sys` bundle.
			err = tabletNode.Sync(context.Background())
			Expect(err).Should(MatchError(createCellNetError))
			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusPending))

			// Failed to get @tablet_cell_count of the `default` bundle.
			err = tabletNode.Sync(context.Background())
			Expect(err).Should(Equal(getNetError))
			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusPending))

			// Then everything was successfully.
			err = tabletNode.Sync(context.Background())
			Expect(err).Should(Succeed())
			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusReady))
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
							"options": map[string]string{
								"changelog_account": "sys",
								"snapshot_account":  "sys",
							},
						}})).
				Return(yt.NodeID(guid.New()), nil)

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
			err := tabletNode.Sync(context.Background())
			Expect(err).Should(Succeed())

			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusReady))
		})

		It("Tablet node Sync; no initialization", func() {
			ytsaurus := apiproxy.NewYtsaurus(ytsaurusSpec, client, record.NewFakeRecorder(1), scheme)

			ytsaurusClient := NewFakeYtsaurusClient(mockYtClient)

			cfgen := ytconfig.NewLocalNodeGenerator(ytsaurusSpec, "cluster_domain")
			tabletNode := NewTabletNode(cfgen, ytsaurus, ytsaurusClient, ytsaurusSpec.Spec.TabletNodes[0], false)
			tabletNode.server = NewFakeServer()
			err := tabletNode.Sync(context.Background())
			Expect(err).Should(Succeed())

			Expect(tabletNode.Status(context.Background()).SyncStatus).Should(Equal(SyncStatusReady))
		})
	})
})
