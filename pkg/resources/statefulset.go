package resources

import (
	"context"

	"k8s.io/utils/ptr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type StatefulSet struct {
	BaseManagedResource[*appsv1.StatefulSet]

	commonSpec *ytv1.CommonSpec
	built      bool
}

func NewStatefulSet(
	name string,
	l *labeller.Labeller,
	proxy apiproxy.APIProxy,
	commonSpec *ytv1.CommonSpec,
) *StatefulSet {
	return &StatefulSet{
		BaseManagedResource: BaseManagedResource[*appsv1.StatefulSet]{
			proxy:     proxy,
			labeller:  l,
			name:      name,
			oldObject: &appsv1.StatefulSet{},
			newObject: &appsv1.StatefulSet{},
		},
		commonSpec: commonSpec,
	}
}

func (s *StatefulSet) Build() *appsv1.StatefulSet {
	if !s.built {
		s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)
		s.newObject.Spec = appsv1.StatefulSetSpec{
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Selector: &metav1.LabelSelector{
				MatchLabels: s.labeller.GetSelectorLabelMap(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      s.labeller.GetMetaLabelMap(false),
					Annotations: s.labeller.GetAnnotations(),
				},
			},
		}
	}

	s.built = true
	return s.newObject
}

func (s *StatefulSet) GetReplicas() int32 {
	return ptr.Deref(s.oldObject.Spec.Replicas, 1)
}

func (s *StatefulSet) ListPods(ctx context.Context) ([]corev1.Pod, error) {
	var podList corev1.PodList
	if err := s.proxy.ListObjects(ctx, &podList, s.labeller.GetListOptions()...); err != nil {
		return nil, err
	}
	return podList.Items, nil
}
