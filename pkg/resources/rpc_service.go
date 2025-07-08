package resources

import (
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	labeller "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
)

type RPCService struct {
	BaseManagedResource[*corev1.Service]

	Ports []corev1.ServicePort
}

func NewRPCService(name string, ports []corev1.ServicePort, labeller *labeller.Labeller, apiProxy apiproxy.APIProxy) *RPCService {
	return &RPCService{
		BaseManagedResource: BaseManagedResource[*corev1.Service]{
			proxy:     apiProxy,
			labeller:  labeller,
			name:      name,
			oldObject: &corev1.Service{},
			newObject: &corev1.Service{},
		},
		Ports: ports,
	}
}

func (s *RPCService) Build() *corev1.Service {
	s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)
	s.newObject.Spec = corev1.ServiceSpec{
		Selector: s.labeller.GetSelectorLabelMap(),
		Ports:    s.Ports,
	}
	return s.newObject
}
