package resources

import (
	"context"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	labeller2 "github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HTTPService struct {
	name      string
	transport *ytv1.HTTPTransportSpec
	labeller  *labeller2.Labeller
	apiProxy  apiproxy.APIProxy

	oldObject corev1.Service
	newObject corev1.Service
}

func NewHTTPService(name string, transport *ytv1.HTTPTransportSpec, labeller *labeller2.Labeller, apiProxy apiproxy.APIProxy) *HTTPService {
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

func (s *HTTPService) Sync(ctx context.Context) error {
	return s.apiProxy.SyncObject(ctx, &s.oldObject, &s.newObject)
}

func (s *HTTPService) Build() *corev1.Service {
	s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)
	s.newObject.Spec = corev1.ServiceSpec{
		Selector: s.labeller.GetSelectorLabelMap(),
	}

	if !s.transport.DisableHTTP {
		s.newObject.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "http",
				Port:       consts.HTTPProxyHTTPPort,
				TargetPort: intstr.IntOrString{IntVal: consts.HTTPProxyHTTPPort},
			},
		}
	}

	if s.transport.HTTPSSecret != nil {
		s.newObject.Spec.Ports = append(s.newObject.Spec.Ports, corev1.ServicePort{
			Name:       "https",
			Port:       consts.HTTPProxyHTTPSPort,
			TargetPort: intstr.IntOrString{IntVal: consts.HTTPProxyHTTPSPort},
		})
	}

	return &s.newObject
}

func (s *HTTPService) Fetch(ctx context.Context) error {
	return s.apiProxy.FetchObject(ctx, s.name, &s.oldObject)
}
