package resources

import (
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	labeller "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type RPCService struct {
	BaseManagedResource[*corev1.Service]

	port     *int32
	nodePort *int32
}

func NewRPCService(name string, labeller *labeller.Labeller, apiProxy apiproxy.APIProxy) *RPCService {
	return &RPCService{
		BaseManagedResource: BaseManagedResource[*corev1.Service]{
			proxy:     apiProxy,
			labeller:  labeller,
			name:      name,
			oldObject: &corev1.Service{},
			newObject: &corev1.Service{},
		},
	}
}

func (s *RPCService) SetNodePort(port *int32) {
	s.nodePort = port
}

func (s *RPCService) SetPort(port *int32) {
	s.port = port
}

func (s *RPCService) Build() *corev1.Service {
	var port int32 = consts.RPCProxyRPCPort
	if s.port != nil {
		port = *s.port
	}
	servicePort := corev1.ServicePort{
		Name:       "rpc",
		Port:       port,
		TargetPort: intstr.IntOrString{IntVal: port},
	}
	if s.nodePort != nil {
		servicePort.NodePort = *s.nodePort
	}

	s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)
	s.newObject.Spec = corev1.ServiceSpec{
		Selector: s.labeller.GetSelectorLabelMap(),
		Ports:    []corev1.ServicePort{servicePort},
	}

	return s.newObject
}
