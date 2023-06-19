package controllers

import (
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	YtsaurusName = "test-ytsaurus"
)

func CreateBaseYtsaurusResource(namespace string) *ytv1.Ytsaurus {
	masterVolumeSize, _ := resource.ParseQuantity("1Gi")
	execNodeVolumeSize, _ := resource.ParseQuantity("1Gi")
	execNodeCPU, _ := resource.ParseQuantity("1")
	execNodeMemory, _ := resource.ParseQuantity("1Gi")

	return &ytv1.Ytsaurus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      YtsaurusName,
			Namespace: namespace,
		},
		Spec: ytv1.YtsaurusSpec{
			CoreImage: "ytsaurus/ytsaurus:unstable-0.0.4",
			Discovery: ytv1.DiscoverySpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount: 1,
				},
			},
			PrimaryMasters: ytv1.MastersSpec{
				CellTag: 1,
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
					VolumeClaimTemplates: []ytv1.EmbeddedPersistentVolumeClaim{
						{
							EmbeddedObjectMetadata: ytv1.EmbeddedObjectMetadata{
								Name: "master-data",
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{"ReadWriteOnce"},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: masterVolumeSize,
									},
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
			ExecNodes: []ytv1.ExecNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 1,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    execNodeCPU,
								corev1.ResourceMemory: execNodeMemory,
							},
						},
						Locations: []ytv1.LocationSpec{
							{
								LocationType: "ChunkCache",
								Path:         "/yt/node-data/chunk-cache",
							},
							{
								LocationType: "Slots",
								Path:         "/yt/node-data/slots",
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "node-data",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{
										SizeLimit: &execNodeVolumeSize,
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
}
