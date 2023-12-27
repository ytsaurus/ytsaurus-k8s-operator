package ytconfig

import (
	"k8s.io/apimachinery/pkg/types"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

type NodeGenerator struct {
	// TODO: flavour
	BaseGenerator
}

func NewNodeGenerator(
	key types.NamespacedName,
	clusterDomain string,
	configSpec *ytv1.ConfigurationSpec,
	masterConnectionSpec *ytv1.MasterConnectionSpec,
	masterInstanceCount int32,
) (*NodeGenerator, error) {
	baseGenerator, err := NewBaseGenerator(
		key,
		clusterDomain,
		configSpec,
		masterConnectionSpec,
		masterInstanceCount,
		0,
	)

	if err != nil {
		return nil, err
	}

	return &NodeGenerator{
		BaseGenerator: *baseGenerator,
	}, nil
}

func NewNodeGeneratorFromYtsaurus(
	ytsaurus *ytv1.Ytsaurus,
	clusterDomain string,
) (*NodeGenerator, error) {
	baseGenerator, err := NewBaseGeneratorFromYtsaurus(ytsaurus, clusterDomain)
	if err != nil {
		return nil, err
	}
	return &NodeGenerator{
		BaseGenerator: *baseGenerator,
	}, nil
}
