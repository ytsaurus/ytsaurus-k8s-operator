package resources

import (
	"fmt"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	labeller2 "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type TCPService struct {
	BaseManagedResource[*corev1.Service]

	serviceType corev1.ServiceType
	portCount   int32
	minPort     int32
}

func NewTCPService(name string,
	serviceType corev1.ServiceType,
	portCount int32,
	minPort int32,
	labeller *labeller2.Labeller,
	apiProxy apiproxy.APIProxy) *TCPService {
	return &TCPService{
		BaseManagedResource: BaseManagedResource[*corev1.Service]{
			proxy:     apiProxy,
			labeller:  labeller,
			name:      name,
			oldObject: &corev1.Service{},
			newObject: &corev1.Service{},
		},
		serviceType: serviceType,
		portCount:   portCount,
		minPort:     minPort,
	}
}

func (s *TCPService) Build() *corev1.Service {
	s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)

	var ports = make([]corev1.ServicePort, 0)
	for port := s.minPort; port < s.minPort+s.portCount; port++ {
		servicePort := corev1.ServicePort{
			Name:       fmt.Sprintf("tcp-%d", port),
			Port:       port,
			TargetPort: intstr.IntOrString{IntVal: port},
		}
		if s.serviceType == corev1.ServiceTypeNodePort {
			servicePort.NodePort = port
		}
		ports = append(ports, servicePort)
	}

	s.newObject.Spec = corev1.ServiceSpec{
		Selector: s.labeller.GetSelectorLabelMap(),
		Type:     s.serviceType,
		Ports:    ports,
	}

	return s.newObject
}
