package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Basic test for YTsaurus controller", func() {

	const (
		ytsaurusName = "test-ytsaurus"
		namespace    = "default"

		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When setting up the test environment", func() {
		It("Should create YTsaurus custom resources", func() {
			By("Creating a first YTsaurus resource")
			ctx := context.Background()

			masterVolumeSize, _ := resource.ParseQuantity("1Gi")

			ytsaurus := v1.Ytsaurus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ytsaurusName,
					Namespace: namespace,
				},
				Spec: v1.YtsaurusSpec{
					CoreImage: "ytsaurus/ytsaurus:latest",
					Discovery: v1.DiscoverySpec{
						InstanceSpec: v1.InstanceSpec{
							InstanceCount: 1,
						},
					},
					PrimaryMasters: v1.MastersSpec{
						CellTag: 1,
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
				},
			}

			Expect(k8sClient.Create(ctx, &ytsaurus)).Should(Succeed())

			ytsaurusLookupKey := types.NamespacedName{Name: ytsaurusName, Namespace: namespace}
			createdYtsaurus := &v1.Ytsaurus{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, ytsaurusLookupKey, createdYtsaurus)
				if err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

})
