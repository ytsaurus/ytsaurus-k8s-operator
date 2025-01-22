package resources

import (
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	labeller2 "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
)

type HeadlessService struct {
	BaseManagedResource[*corev1.Service]
}

func NewHeadlessService(name string, labeller *labeller2.Labeller, apiProxy apiproxy.APIProxy) *HeadlessService {
	return &HeadlessService{
		BaseManagedResource: BaseManagedResource[*corev1.Service]{
			proxy:     apiProxy,
			labeller:  labeller,
			name:      name,
			oldObject: &corev1.Service{},
			newObject: &corev1.Service{},
		},
	}
}

func (s *HeadlessService) Build() *corev1.Service {
	s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)
	s.newObject.Spec = corev1.ServiceSpec{
		ClusterIP: "None",
		Selector:  s.labeller.GetSelectorLabelMap(),
	}

	return s.newObject
}
