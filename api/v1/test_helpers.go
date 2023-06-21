package v1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	YtsaurusName = "test-ytsaurus"
)

func CreateBaseYtsaurusResource(namespace string) *Ytsaurus {
	masterVolumeSize, _ := resource.ParseQuantity("5Gi")
	execNodeVolumeSize, _ := resource.ParseQuantity("3Gi")
	execNodeCPU, _ := resource.ParseQuantity("1")
	execNodeMemory, _ := resource.ParseQuantity("2Gi")

	return &Ytsaurus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      YtsaurusName,
			Namespace: namespace,
		},
		Spec: YtsaurusSpec{
			CoreImage: "ytsaurus/ytsaurus:23.1-latest",
			Discovery: DiscoverySpec{
				InstanceSpec: InstanceSpec{
					InstanceCount: 1,
				},
			},
			PrimaryMasters: MastersSpec{
				CellTag: 1,
				InstanceSpec: InstanceSpec{
					InstanceCount: 1,
					Locations: []LocationSpec{
						{
							LocationType: "MasterChangelogs",
							Path:         "/yt/master-data/master-changelogs",
						},
						{
							LocationType: "MasterSnapshots",
							Path:         "/yt/master-data/master-snapshots",
						},
					},
					VolumeClaimTemplates: []EmbeddedPersistentVolumeClaim{
						{
							EmbeddedObjectMetadata: EmbeddedObjectMetadata{
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
					Loggers: []LoggerSpec{
						{
							Name:        "debug",
							WriterType:  "file",
							MinLogLevel: "debug",
						},
					},
				},
			},
			HTTPProxies: []HTTPProxiesSpec{
				{
					ServiceType: "NodePort",
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
					},
				},
			},
			DataNodes: []DataNodesSpec{
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
						Locations: []LocationSpec{
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
			ExecNodes: []ExecNodesSpec{
				{
					InstanceSpec: InstanceSpec{
						InstanceCount: 1,
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    execNodeCPU,
								corev1.ResourceMemory: execNodeMemory,
							},
						},
						Locations: []LocationSpec{
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
