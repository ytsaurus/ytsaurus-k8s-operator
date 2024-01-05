package ytconfig

import (
	"k8s.io/apimachinery/pkg/types"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

type NodeGenerator struct {
	BaseGenerator
}

func NewNodeGenerator(
	key types.NamespacedName,
	clusterDomain string,
	configSpec ytv1.ConfigurationSpec,
	masterConnectionSpec ytv1.MasterConnectionSpec,
) *NodeGenerator {
	baseGenerator := NewBaseGenerator(
		key,
		clusterDomain,
		configSpec,
		masterConnectionSpec,
	)
	return &NodeGenerator{
		BaseGenerator: *baseGenerator,
	}
}

func NewNodeGeneratorFromYtsaurus(
	ytsaurus *ytv1.Ytsaurus,
	clusterDomain string,
) *NodeGenerator {
	baseGenerator := NewBaseGeneratorFromYtsaurus(ytsaurus, clusterDomain)
	return &NodeGenerator{
		BaseGenerator: *baseGenerator,
	}
}
