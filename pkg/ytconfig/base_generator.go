package ytconfig

import (
	"k8s.io/apimachinery/pkg/types"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

type BaseGenerator struct {
	key           types.NamespacedName
	clusterDomain string

	commonSpec             ytv1.CommonSpec
	masterConnectionSpec   ytv1.MasterConnectionSpec
	masterInstanceCount    int32
	discoveryInstanceCount int32
	dataNodesInstanceCount int32
	masterCachesSpec       *ytv1.MasterCachesSpec
}

func NewRemoteBaseGenerator(
	key types.NamespacedName,
	clusterDomain string,
	commonSpec ytv1.CommonSpec,
	masterConnectionSpec ytv1.MasterConnectionSpec,
	masterCachesSpec *ytv1.MasterCachesSpec,
) *BaseGenerator {
	return &BaseGenerator{
		key:                  key,
		clusterDomain:        clusterDomain,
		commonSpec:           commonSpec,
		masterConnectionSpec: masterConnectionSpec,
		masterCachesSpec:     masterCachesSpec,
	}
}

func NewLocalBaseGenerator(
	ytsaurus *ytv1.Ytsaurus,
	clusterDomain string,
) *BaseGenerator {
	var dataNodesInstanceCount int32
	for _, dataNodes := range ytsaurus.Spec.DataNodes {
		dataNodesInstanceCount += dataNodes.InstanceCount
	}

	return &BaseGenerator{
		key: types.NamespacedName{
			Namespace: ytsaurus.Namespace,
			Name:      ytsaurus.Name,
		},
		clusterDomain:          clusterDomain,
		commonSpec:             ytsaurus.Spec.CommonSpec,
		masterConnectionSpec:   ytsaurus.Spec.PrimaryMasters.MasterConnectionSpec,
		masterInstanceCount:    ytsaurus.Spec.PrimaryMasters.InstanceCount,
		discoveryInstanceCount: ytsaurus.Spec.Discovery.InstanceCount,
		masterCachesSpec:       ytsaurus.Spec.MasterCaches,
		dataNodesInstanceCount: dataNodesInstanceCount,
	}
}

func (g *BaseGenerator) GetMaxReplicationFactor() int32 {
	return g.dataNodesInstanceCount
}
