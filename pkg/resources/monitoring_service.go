package resources

import (
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	labeller2 "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type MonitoringService struct {
	BaseManagedResource[*corev1.Service]

	ports []corev1.ServicePort
}

func NewMonitoringService(monitoringTargetPort int32, labeller *labeller2.Labeller, apiProxy apiproxy.APIProxy) *MonitoringService {
	return &MonitoringService{
		BaseManagedResource: BaseManagedResource[*corev1.Service]{
			proxy:     apiProxy,
			labeller:  labeller,
			name:      labeller.GetMonitoringServiceName(),
			oldObject: &corev1.Service{},
			newObject: &corev1.Service{},
		},
		ports: []corev1.ServicePort{
			{
				Name:       consts.YTMonitoringServicePortName,
				Port:       consts.YTMonitoringPort,
				TargetPort: intstr.FromInt32(monitoringTargetPort),
			},
		},
	}
}

func (s *MonitoringService) GetServiceMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: s.labeller.GetNamespace(),
		Labels:    s.labeller.GetMonitoringMetaLabelMap(),
	}
}

func (s *MonitoringService) Build() *corev1.Service {
	s.newObject.ObjectMeta = s.GetServiceMeta(s.name)

	// FIXME(khlebnikov): Must be headless.
	s.newObject.Spec = corev1.ServiceSpec{
		Selector: s.labeller.GetSelectorLabelMap(),
		Ports:    s.ports,
	}

	return s.newObject
}

func (s *MonitoringService) AddPort(port corev1.ServicePort) {
	s.ports = append(s.ports, port)
}
