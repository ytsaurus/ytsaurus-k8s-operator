package resources

import (
	"context"
	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/YTsaurus/yt-k8s-operator/pkg/consts"
	labeller2 "github.com/YTsaurus/yt-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type HttpService struct {
	name     string
	labeller *labeller2.Labeller
	apiProxy *apiproxy.ApiProxy

	oldObject corev1.Service
	newObject corev1.Service
}

func NewHttpService(name string, labeller *labeller2.Labeller, apiProxy *apiproxy.ApiProxy) *HttpService {
	return &HttpService{
		name:     name,
		labeller: labeller,
		apiProxy: apiProxy,
	}
}

func (s *HttpService) OldObject() client.Object {
	return &s.oldObject
}

func (s *HttpService) Name() string {
	return s.name
}

func (s *HttpService) Sync(ctx context.Context) error {
	return s.apiProxy.SyncObject(ctx, &s.oldObject, &s.newObject)
}

func (s *HttpService) Build() *corev1.Service {
	s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)
	s.newObject.Spec = corev1.ServiceSpec{
		Selector: s.labeller.GetSelectorLabelMap(),
		Ports: []corev1.ServicePort{
			{
				Name:       "http",
				Port:       consts.HttpProxyHttpPort,
				TargetPort: intstr.IntOrString{IntVal: consts.HttpProxyHttpPort},
			},
		},
	}

	return &s.newObject
}

func (s *HttpService) Fetch(ctx context.Context) error {
	return s.apiProxy.FetchObject(ctx, s.name, &s.oldObject)
}
