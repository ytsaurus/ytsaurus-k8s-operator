package resources

import (
	"context"
	"fmt"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	labeller2 "github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TCPService struct {
	name        string
	serviceType corev1.ServiceType
	portCount   int32
	minPort     int32

	labeller *labeller2.Labeller
	apiProxy apiproxy.APIProxy

	oldObject corev1.Service
	newObject corev1.Service
}

func NewTCPService(name string,
	serviceType corev1.ServiceType,
	portCount int32,
	minPort int32,
	labeller *labeller2.Labeller,
	apiProxy apiproxy.APIProxy) *TCPService {
	return &TCPService{
		name:        name,
		serviceType: serviceType,
		portCount:   portCount,
		minPort:     minPort,
		labeller:    labeller,
		apiProxy:    apiProxy,
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

	return &s.newObject
}

func (s *TCPService) Fetch(ctx context.Context) error {
	return s.apiProxy.FetchObject(ctx, s.name, &s.oldObject)
}
