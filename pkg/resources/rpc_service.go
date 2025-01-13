package resources

import (
	"context"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	labeller "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RPCService struct {
	name     string
	labeller *labeller.Labeller
	apiProxy apiproxy.APIProxy
	port     *int32
	nodePort *int32

	oldObject corev1.Service
	newObject corev1.Service
}

func NewRPCService(name string, labeller *labeller.Labeller, apiProxy apiproxy.APIProxy) *RPCService {
	return &RPCService{
		name:     name,
		labeller: labeller,
		apiProxy: apiProxy,
	}
}

func (s *RPCService) Service() corev1.Service {
	return s.oldObject
}

func (s *RPCService) OldObject() client.Object {
	return &s.oldObject
}

func (s *RPCService) Name() string {
	return s.name
}

func (s *RPCService) SetNodePort(port *int32) {
	s.nodePort = port
}

func (s *RPCService) SetPort(port *int32) {
	s.port = port
}

func (s *RPCService) Sync(ctx context.Context) error {
	return s.apiProxy.SyncObject(ctx, &s.oldObject, &s.newObject)
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

	return &s.newObject
}

func (s *RPCService) Fetch(ctx context.Context) error {
	return s.apiProxy.FetchObject(ctx, s.name, &s.oldObject)
}
