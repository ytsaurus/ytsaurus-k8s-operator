package resources

import (
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	labeller2 "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
)

type ConfigMap struct {
	BaseManagedResource[*corev1.ConfigMap]
}

func NewConfigMap(name string, labeller *labeller2.Labeller, apiProxy apiproxy.APIProxy) *ConfigMap {
	return &ConfigMap{
		BaseManagedResource: BaseManagedResource[*corev1.ConfigMap]{
			proxy:     apiProxy,
			labeller:  labeller,
			name:      name,
			oldObject: &corev1.ConfigMap{},
			newObject: &corev1.ConfigMap{},
		},
	}
}

func (s *ConfigMap) Build() *corev1.ConfigMap {
	s.newObject = &corev1.ConfigMap{
		ObjectMeta: s.labeller.GetObjectMeta(s.name),
		Data:       map[string]string{},
	}
	return s.newObject
}
