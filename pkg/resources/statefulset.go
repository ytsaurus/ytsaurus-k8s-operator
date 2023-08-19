package resources

import (
	"context"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	labeller2 "github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type StatefulSet struct {
	name     string
	labeller *labeller2.Labeller
	ytsaurus *apiproxy.Ytsaurus

	oldObject appsv1.StatefulSet
	newObject appsv1.StatefulSet
	built     bool
}

func NewStatefulSet(
	name string,
	labeller *labeller2.Labeller,
	ytsaurus *apiproxy.Ytsaurus) *StatefulSet {
	return &StatefulSet{
		name:     name,
		labeller: labeller,
		ytsaurus: ytsaurus,
	}
}

func (s *StatefulSet) OldObject() client.Object {
	return &s.oldObject
}

func (s *StatefulSet) Name() string {
	return s.name
}

func (s *StatefulSet) Sync(ctx context.Context) error {
	return s.ytsaurus.APIProxy().SyncObject(ctx, &s.oldObject, &s.newObject)
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
					Labels:      s.labeller.GetMetaLabelMap(),
					Annotations: s.ytsaurus.GetResource().Spec.ExtraPodAnnotations,
				},
			},
		}
	}

	s.built = true
	return &s.newObject
}

func (s *StatefulSet) ArePodsReady(ctx context.Context) bool {
	logger := log.FromContext(ctx)
	podList := &corev1.PodList{}
	err := s.ytsaurus.APIProxy().ListObjects(ctx, podList, s.labeller.GetListOptions()...)
	if err != nil {
		logger.Error(err, "unable to list pods for component", "component", s.labeller.ComponentName)
		return false
	}

	if len(podList.Items) == 0 {
		logger.Info("no pods found for component", "component", s.labeller.ComponentName)
		return false
	}

	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			logger.Info("pod is not yet running", "podName", pod.Name, "phase", pod.Status.Phase)
			return false
		}
	}

	return true
}

func (s *StatefulSet) NeedSync(replicas int32) bool {
	return s.oldObject.Spec.Replicas == nil ||
		*s.oldObject.Spec.Replicas != replicas ||
		len(s.oldObject.Spec.Template.Spec.Containers) != 1
}

func (s *StatefulSet) Fetch(ctx context.Context) error {
	return s.ytsaurus.APIProxy().FetchObject(ctx, s.name, &s.oldObject)
}
