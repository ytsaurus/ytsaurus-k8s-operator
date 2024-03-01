package ytconfig

import (
	"k8s.io/apimachinery/pkg/types"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

type BaseGenerator struct {
	key           types.NamespacedName
	clusterDomain string

	commonSpec               ytv1.CommonSpec
	masterConnectionSpec     ytv1.MasterConnectionSpec
	masterInstanceCount      int32
	discoveryInstanceCount   int32
	masterCachesSpec         *ytv1.MasterCachesSpec
	masterCacheInstanceCount int32
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
	return &BaseGenerator{
		key: types.NamespacedName{
			Namespace: ytsaurus.Namespace,
			Name:      ytsaurus.Name,
		},
		clusterDomain:            clusterDomain,
		commonSpec:               ytsaurus.Spec.CommonSpec,
		masterConnectionSpec:     ytsaurus.Spec.PrimaryMasters.MasterConnectionSpec,
		masterInstanceCount:      ytsaurus.Spec.PrimaryMasters.InstanceCount,
		discoveryInstanceCount:   ytsaurus.Spec.Discovery.InstanceCount,
		masterCacheInstanceCount: ytsaurus.Spec.MasterCaches.InstanceCount,
		masterCachesSpec:         ytsaurus.Spec.MasterCaches,
	}
}
