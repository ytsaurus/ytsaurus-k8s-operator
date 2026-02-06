package resources

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	labeller2 "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type DaemonSet struct {
	BaseManagedResource[*appsv1.DaemonSet]

	ytsaurus     apiproxy.Ytsaurus
	tolerations  []corev1.Toleration
	nodeSelector map[string]string
	built        bool
}

func NewDaemonSet(
	name string,
	labeller *labeller2.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	tolerations []corev1.Toleration,
	nodeSelector map[string]string,
) *DaemonSet {
	return &DaemonSet{
		BaseManagedResource: BaseManagedResource[*appsv1.DaemonSet]{
			proxy:     ytsaurus.APIProxy(),
			labeller:  labeller,
			name:      name,
			oldObject: &appsv1.DaemonSet{},
			newObject: &appsv1.DaemonSet{},
		},
		ytsaurus:     *ytsaurus,
		tolerations:  tolerations,
		nodeSelector: nodeSelector,
	}
}

func (d *DaemonSet) Build() *appsv1.DaemonSet {
	if !d.built {
		maxUnavailable := intstr.FromString("100%")
		d.newObject.ObjectMeta = d.labeller.GetObjectMeta(d.name)
		d.newObject.Spec = appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: d.labeller.GetSelectorLabelMap(),
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      d.labeller.GetMetaLabelMap(false),
					Annotations: d.labeller.GetAnnotations(),
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: d.ytsaurus.GetResource().Spec.ImagePullSecrets,
					Tolerations:      d.tolerations,
					NodeSelector:     d.nodeSelector,
				},
			},
		}
	}

	d.built = true
	return d.newObject
}

func (d *DaemonSet) ArePodsReady(ctx context.Context) bool {
	logger := log.FromContext(ctx)
	logger.Info("Image heater checking daemonset pods", "daemonset", d.name)
	podList := d.listPods(ctx)
	if podList == nil {
		logger.Info("Image heater podList is nil", "daemonset", d.name)
		return false
	}

	if len(podList.Items) == 0 {
		logger.Info("Image heater daemonset has no pods", "daemonset", d.name)
		return false
	}

	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			logger.Info("Image heater pod is not yet running", "podName", pod.Name, "phase", pod.Status.Phase)
			return false
		}
	}

	return true
}
