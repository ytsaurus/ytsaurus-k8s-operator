package resources

import (
	"context"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	labeller2 "github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HTTPService struct {
	name     string
	labeller *labeller2.Labeller
	apiProxy *apiproxy.APIProxy

	oldObject corev1.Service
	newObject corev1.Service
}

func NewHTTPService(name string, labeller *labeller2.Labeller, apiProxy *apiproxy.APIProxy) *HTTPService {
	return &HTTPService{
		name:     name,
		labeller: labeller,
		apiProxy: apiProxy,
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
		Ports: []corev1.ServicePort{
			{
				Name:       "http",
				Port:       consts.HTTPProxyHTTPPort,
				TargetPort: intstr.IntOrString{IntVal: consts.HTTPProxyHTTPPort},
			},
		},
	}

	return &s.newObject
}

func (s *HTTPService) Fetch(ctx context.Context) error {
	return s.apiProxy.FetchObject(ctx, s.name, &s.oldObject)
}
