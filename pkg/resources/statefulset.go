package resources

import (
	"context"

	"github.com/YTsaurus/yt-k8s-operator/pkg/apiproxy"
	labeller2 "github.com/YTsaurus/yt-k8s-operator/pkg/labeller"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type StatefulSet struct {
	name     string
	labeller *labeller2.Labeller
	apiProxy *apiproxy.APIProxy

	oldObject appsv1.StatefulSet
	newObject appsv1.StatefulSet
	built     bool
}

func NewStatefulSet(
	name string,
	labeller *labeller2.Labeller,
	apiProxy *apiproxy.APIProxy) *StatefulSet {
	return &StatefulSet{
		name:     name,
		labeller: labeller,
		apiProxy: apiProxy,
	}
}

func (s *StatefulSet) OldObject() client.Object {
	return &s.oldObject
}

func (s *StatefulSet) Name() string {
	return s.name
}

func (s *StatefulSet) Sync(ctx context.Context) error {
	return s.apiProxy.SyncObject(ctx, &s.oldObject, &s.newObject)
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
					Annotations: s.apiProxy.Ytsaurus().Spec.ExtraPodAnnotations,
				},
			},
		}
	}

	s.built = true
	return &s.newObject
}

func (s *StatefulSet) CheckPodsReady(ctx context.Context) bool {
	logger := log.FromContext(ctx)
	podList := &corev1.PodList{}
	err := s.apiProxy.ListObjects(ctx, podList, s.labeller.GetListOptions()...)
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

func (s *StatefulSet) NeedSync(replicas int32, image string) bool {
	if s.oldObject.Spec.Replicas == nil {
		return false
	}

	if *s.oldObject.Spec.Replicas != replicas {
		return false
	}

	if len(s.oldObject.Spec.Template.Spec.Containers) != 1 {
		return false
	}

	if s.oldObject.Spec.Template.Spec.Containers[0].Image != image {
		return false
	}

	return true
}

func (s *StatefulSet) Fetch(ctx context.Context) error {
	return s.apiProxy.FetchObject(ctx, s.name, &s.oldObject)
}
