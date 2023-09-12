package resources

import (
	"context"
	"fmt"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	labeller2 "github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TCPService struct {
	name string

	labeller *labeller2.Labeller
	apiProxy apiproxy.APIProxy

	oldObject corev1.Service
	newObject corev1.Service
}

func NewTCPService(name string, labeller *labeller2.Labeller, apiProxy apiproxy.APIProxy) *TCPService {
	return &TCPService{
		name:     name,
		labeller: labeller,
		apiProxy: apiProxy,
	}
}

func (s *TCPService) Service() corev1.Service {
	return s.oldObject
}

func (s *TCPService) OldObject() client.Object {
	return &s.oldObject
}

func (s *TCPService) Name() string {
	return s.name
}

func (s *TCPService) Sync(ctx context.Context) error {
	return s.apiProxy.SyncObject(ctx, &s.oldObject, &s.newObject)
}

func (s *TCPService) Build() *corev1.Service {
	s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)

	var ports = make([]corev1.ServicePort, 0)
	for port := consts.TCPProxyMinTCPPort; port <= consts.TCPProxyMaxTCPPort; port++ {
		ports = append(ports, corev1.ServicePort{
			Name:       fmt.Sprintf("tcp[%d]", port),
			Port:       int32(port),
			TargetPort: intstr.IntOrString{IntVal: int32(port)},
		})
	}

	s.newObject.Spec = corev1.ServiceSpec{
		Selector: s.labeller.GetSelectorLabelMap(),
		Ports:    ports,
	}

	return &s.newObject
}

func (s *TCPService) Fetch(ctx context.Context) error {
	return s.apiProxy.FetchObject(ctx, s.name, &s.oldObject)
}
