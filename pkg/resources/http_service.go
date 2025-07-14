package resources

import (
	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	labeller "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

type HTTPService struct {
	BaseManagedResource[*corev1.Service]

	transport     *ytv1.HTTPTransportSpec
	httpPort      *int32
	httpsPort     *int32
	httpNodePort  *int32
	httpsNodePort *int32
}

func NewHTTPService(name string, transport *ytv1.HTTPTransportSpec, labeller *labeller.Labeller, apiProxy apiproxy.APIProxy) *HTTPService {
	if transport == nil {
		transport = &ytv1.HTTPTransportSpec{}
	}
	return &HTTPService{
		BaseManagedResource: BaseManagedResource[*corev1.Service]{
			proxy:     apiProxy,
			labeller:  labeller,
			name:      name,
			oldObject: &corev1.Service{},
			newObject: &corev1.Service{},
		},
		transport: transport,
	}
}

func (s *HTTPService) SetHttpPort(port *int32) {
	s.httpPort = port
}

func (s *HTTPService) SetHttpsPort(port *int32) {
	s.httpsPort = port
}

func (s *HTTPService) SetHttpNodePort(port *int32) {
	s.httpNodePort = port
}

func (s *HTTPService) SetHttpsNodePort(port *int32) {
	s.httpsNodePort = port
}

func (s *HTTPService) Build() *corev1.Service {
	s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)
	s.newObject.Spec = corev1.ServiceSpec{
		Selector: s.labeller.GetSelectorLabelMap(),
	}

	httpPort := ptr.Deref(s.httpPort, consts.HTTPProxyHTTPPort)
	httpsPort := ptr.Deref(s.httpsPort, consts.HTTPProxyHTTPSPort)

	s.newObject.Spec.Ports = make([]corev1.ServicePort, 0, 2)
	if !s.transport.DisableHTTP {
		port := corev1.ServicePort{
			Name:       consts.HTTPPortName,
			Port:       httpPort,
			TargetPort: intstr.FromInt32(httpPort),
		}
		if s.httpNodePort != nil {
			port.NodePort = *s.httpNodePort
		}
		s.newObject.Spec.Ports = append(s.newObject.Spec.Ports, port)
	}

	if s.transport.HTTPSSecret != nil {
		port := corev1.ServicePort{
			Name:       consts.HTTPSPortName,
			Port:       httpsPort,
			TargetPort: intstr.FromInt32(httpsPort),
		}
		if s.httpsNodePort != nil {
			port.NodePort = *s.httpsNodePort
		}
		s.newObject.Spec.Ports = append(s.newObject.Spec.Ports, port)
	}

	return s.newObject
}
