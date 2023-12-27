package ytconfig

import (
	"errors"

	"k8s.io/apimachinery/pkg/types"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

type BaseGenerator struct {
	key           types.NamespacedName
	clusterDomain string

	configSpec *ytv1.ConfigurationSpec
	// masterConnectionSpec.hostAddresses and masterInstanceCount can't be empty at the same time.
	masterConnectionSpec   *ytv1.MasterConnectionSpec
	masterInstanceCount    int32
	discoveryInstanceCount int32
}

func NewBaseGenerator(
	key types.NamespacedName,
	clusterDomain string,
	configSpec *ytv1.ConfigurationSpec,
	masterConnectionSpec *ytv1.MasterConnectionSpec,
	masterInstanceCount int32,
	discoveryInstanceCount int32,
) (*BaseGenerator, error) {
	generator := &BaseGenerator{
		key:                    key,
		clusterDomain:          clusterDomain,
		configSpec:             configSpec,
		masterConnectionSpec:   masterConnectionSpec,
		masterInstanceCount:    masterInstanceCount,
		discoveryInstanceCount: discoveryInstanceCount,
	}
	if err := generator.validate(); err != nil {
		return nil, err
	}
	return generator, nil
}

func NewBaseGeneratorFromYtsaurus(
	ytsaurus *ytv1.Ytsaurus,
	clusterDomain string,
) (*BaseGenerator, error) {
	return NewBaseGenerator(
		types.NamespacedName{
			Namespace: ytsaurus.Namespace,
			Name:      ytsaurus.Name,
		},
		clusterDomain,
		&ytsaurus.Spec.ConfigurationSpec,
		&ytsaurus.Spec.PrimaryMasters.MasterConnectionSpec,
		ytsaurus.Spec.PrimaryMasters.InstanceCount,
		ytsaurus.Spec.Discovery.InstanceCount,
	)
}

func (g *BaseGenerator) validate() error {
	if g.masterInstanceCount == 0 && len(g.masterConnectionSpec.HostAddresses) == 0 {
		return errors.New("masterInstanceCount and masterConnectionSpec.HostAddresses can't be empty at the same time")
	}
	return nil
}
