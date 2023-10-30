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
					Labels:      s.labeller.GetMetaLabelMap(false),
					Annotations: s.ytsaurus.GetResource().Spec.ExtraPodAnnotations,
				},
			},
		}
	}

	s.built = true
	return &s.newObject
}

func (s *StatefulSet) getPods(ctx context.Context) *corev1.PodList {
	logger := log.FromContext(ctx)
	podList := &corev1.PodList{}
	err := s.ytsaurus.APIProxy().ListObjects(ctx, podList, s.labeller.GetListOptions()...)
	if err != nil {
		logger.Error(err, "unable to list pods for component", "component", s.labeller.ComponentName)
		return nil
	}

	return podList
}

func (s *StatefulSet) ArePodsRemoved(ctx context.Context) bool {
	logger := log.FromContext(ctx)

	podList := s.getPods(ctx)
	if podList == nil {
		return false
	}
	podsCount := len(podList.Items)
	if podsCount != 0 {
		logger.Info("there are pods", "podsCount", podsCount, "component", s.labeller.ComponentName)
		return false
	}
	return true
}

func (s *StatefulSet) ArePodsReady(ctx context.Context, minReadyInstanceCount *int) bool {
	logger := log.FromContext(ctx)
	podList := s.getPods(ctx)
	if podList == nil {
		return false
	}

	effectiveMinReadyInstanceCount := len(podList.Items)
	if minReadyInstanceCount != nil && *minReadyInstanceCount < len(podList.Items) {
		effectiveMinReadyInstanceCount = *minReadyInstanceCount
	}

	readyInstanceCount := 0
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			logger.Info("pod is not yet running", "podName", pod.Name, "phase", pod.Status.Phase)
		} else {
			readyInstanceCount += 1
		}
	}

	if readyInstanceCount < effectiveMinReadyInstanceCount {
		logger.Info(
			"not enough pods are running",
			"component", s.labeller.ComponentName,
			"readyInstanceCount", readyInstanceCount,
			"minReadyInstanceCount", effectiveMinReadyInstanceCount,
			"totalInstanceCount", len(podList.Items))
		return false
	}

	return true
}

func (s *StatefulSet) NeedSync(replicas int32) bool {
	return s.oldObject.Spec.Replicas == nil ||
		*s.oldObject.Spec.Replicas != replicas
}

func (s *StatefulSet) Fetch(ctx context.Context) error {
	return s.ytsaurus.APIProxy().FetchObject(ctx, s.name, &s.oldObject)
}
