package resources

import (
	"context"
	"fmt"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	labeller2 "github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MonitoringService struct {
	name     string
	labeller *labeller2.Labeller
	apiProxy apiproxy.APIProxy

	monitoringTargetPort int32

	oldObject corev1.Service
	newObject corev1.Service
}

func NewMonitoringService(monitoringTargetPort int32, labeller *labeller2.Labeller, apiProxy apiproxy.APIProxy) *MonitoringService {
	return &MonitoringService{
		name:                 fmt.Sprintf("%s-monitoring", labeller.ComponentObjectsNamePrefix),
		labeller:             labeller,
		apiProxy:             apiProxy,
		monitoringTargetPort: monitoringTargetPort,
	}
}

func (s *MonitoringService) Service() corev1.Service {
	return s.oldObject
}

func (s *MonitoringService) OldObject() client.Object {
	return &s.oldObject
}

func (s *MonitoringService) Name() string {
	return s.name
}

func (s *MonitoringService) Sync(ctx context.Context) error {
	return s.apiProxy.SyncObject(ctx, &s.oldObject, &s.newObject)
}

func (s *MonitoringService) GetServiceMeta(name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: s.labeller.ObjectMeta.Namespace,
		Labels:    s.labeller.GetMonitoringMetaLabelMap(),
	}
}

func (s *MonitoringService) Build() *corev1.Service {
	s.newObject.ObjectMeta = s.GetServiceMeta(s.name)
	s.newObject.Spec = corev1.ServiceSpec{
		Selector: s.labeller.GetSelectorLabelMap(),
		Ports: []corev1.ServicePort{
			{
				Name:       consts.YTMonitoringPortName,
				Port:       consts.YTMonitoringPort,
				TargetPort: intstr.IntOrString{IntVal: s.monitoringTargetPort},
			},
		},
	}

	return &s.newObject
}

func (s *MonitoringService) Fetch(ctx context.Context) error {
	return s.apiProxy.FetchObject(ctx, s.name, &s.oldObject)
}
