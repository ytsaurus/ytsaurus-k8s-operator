package resources

import (
	"context"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	labeller "github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HTTPService struct {
	name          string
	transport     *ytv1.HTTPTransportSpec
	labeller      *labeller.Labeller
	apiProxy      apiproxy.APIProxy
	httpNodePort  *int32
	httpsNodePort *int32

	oldObject corev1.Service
	newObject corev1.Service
}

func NewHTTPService(name string, transport *ytv1.HTTPTransportSpec, labeller *labeller.Labeller, apiProxy apiproxy.APIProxy) *HTTPService {
	if transport == nil {
		transport = &ytv1.HTTPTransportSpec{}
	}
	return &HTTPService{
		name:      name,
		transport: transport,
		labeller:  labeller,
		apiProxy:  apiProxy,
	}
}

func (s *HTTPService) OldObject() client.Object {
	return &s.oldObject
}

func (s *HTTPService) Name() string {
	return s.name
}

func (s *HTTPService) SetHttpNodePort(port *int32) {
	s.httpNodePort = port
}

func (s *HTTPService) SetHttpsNodePort(port *int32) {
	s.httpsNodePort = port
}

func (s *HTTPService) Sync(ctx context.Context) error {
	return s.apiProxy.SyncObject(ctx, &s.oldObject, &s.newObject)
}

func (s *HTTPService) Build() *corev1.Service {
	s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)
	s.newObject.Spec = corev1.ServiceSpec{
		Selector: s.labeller.GetSelectorLabelMap(),
	}

	s.newObject.Spec.Ports = make([]corev1.ServicePort, 0, 2)
	if !s.transport.DisableHTTP {
		port := corev1.ServicePort{
			Name:       "http",
			Port:       consts.HTTPProxyHTTPPort,
			TargetPort: intstr.IntOrString{IntVal: consts.HTTPProxyHTTPPort},
		}
		if s.httpNodePort != nil {
			port.NodePort = *s.httpNodePort
		}
		s.newObject.Spec.Ports = append(s.newObject.Spec.Ports, port)
	}

	if s.transport.HTTPSSecret != nil {
		port := corev1.ServicePort{
			Name:       "https",
			Port:       consts.HTTPProxyHTTPSPort,
			TargetPort: intstr.IntOrString{IntVal: consts.HTTPProxyHTTPSPort},
		}
		if s.httpsNodePort != nil {
			port.NodePort = *s.httpsNodePort
		}
		s.newObject.Spec.Ports = append(s.newObject.Spec.Ports, port)
	}

	return &s.newObject
}

func (s *HTTPService) Fetch(ctx context.Context) error {
	return s.apiProxy.FetchObject(ctx, s.name, &s.oldObject)
}
