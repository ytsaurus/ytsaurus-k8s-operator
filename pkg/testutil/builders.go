package testutil

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ptr "k8s.io/utils/pointer"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
)

const (
	testYtsaurusImage = "test-ytsaurus-image"
	dndsNameOne       = "dn-1"
)

func BuildMinimalYtsaurus(namespace, name string) ytv1.Ytsaurus {
	return ytv1.Ytsaurus{
		ObjectMeta: v1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: ytv1.YtsaurusSpec{
			CommonSpec: ytv1.CommonSpec{
				CoreImage:     testYtsaurusImage,
				UseShortNames: true,
			},
			IsManaged:        true,
			EnableFullUpdate: false,

			Discovery: ytv1.DiscoverySpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount:  3,
					MonitoringPort: ptr.Int32(consts.DiscoveryMonitoringPort),
				},
			},
			PrimaryMasters: ytv1.MastersSpec{
				InstanceSpec: ytv1.InstanceSpec{
					InstanceCount:  3,
					MonitoringPort: ptr.Int32(consts.MasterMonitoringPort),
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
				},
				MasterConnectionSpec: ytv1.MasterConnectionSpec{
					CellTag: 1,
				},
			},
			HTTPProxies: []ytv1.HTTPProxiesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{InstanceCount: 3},
					ServiceType:  corev1.ServiceTypeNodePort,
				},
			},
			DataNodes: []ytv1.DataNodesSpec{
				{
					InstanceSpec: ytv1.InstanceSpec{
						InstanceCount: 5,
						Locations: []ytv1.LocationSpec{
							{
								LocationType: "ChunkStore",
								Path:         "/yt/node-data/chunk-store",
							},
						},
					},
					ClusterNodesSpec: ytv1.ClusterNodesSpec{},
					Name:             dndsNameOne,
				},
			},
		},
	}
}
