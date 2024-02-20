package ytconfig

import (
	"k8s.io/apimachinery/pkg/types"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

type BaseGenerator struct {
	key           types.NamespacedName
	clusterDomain string

	configSpec             ytv1.ConfigurationSpec
	masterConnectionSpec   ytv1.MasterConnectionSpec
	masterInstanceCount    int32
	discoveryInstanceCount int32
}

func NewRemoteBaseGenerator(
	key types.NamespacedName,
	clusterDomain string,
	configSpec ytv1.ConfigurationSpec,
	masterConnectionSpec ytv1.MasterConnectionSpec,
) *BaseGenerator {
	return &BaseGenerator{
		key:                  key,
		clusterDomain:        clusterDomain,
		configSpec:           configSpec,
		masterConnectionSpec: masterConnectionSpec,
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
		clusterDomain:          clusterDomain,
		configSpec:             ytsaurus.Spec.ConfigurationSpec,
		masterConnectionSpec:   ytsaurus.Spec.PrimaryMasters.MasterConnectionSpec,
		masterInstanceCount:    ytsaurus.Spec.PrimaryMasters.InstanceCount,
		discoveryInstanceCount: ytsaurus.Spec.Discovery.InstanceCount,
	}
}
