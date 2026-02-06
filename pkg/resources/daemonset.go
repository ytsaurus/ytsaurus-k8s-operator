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
			proxy:     ytsaurus,
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

func (d *DaemonSet) ListPods(ctx context.Context) ([]corev1.Pod, error) {
	podList := &corev1.PodList{}
	if err := d.proxy.ListObjects(ctx, podList, d.labeller.GetListOptions()...); err != nil {
		return nil, err
	}
	return podList.Items, nil
}

func (d *DaemonSet) ArePodsReady(ctx context.Context) bool {
	logger := log.FromContext(ctx)
	ds := d.OldObject()
	if ds == nil {
		logger.Info("Image heater daemonset is nil", "daemonset", d.name)
		return false
	}

	if ds.Generation != ds.Status.ObservedGeneration {
		logger.Info("Image heater daemonset status not observed yet",
			"daemonset", d.name,
			"generation", ds.Generation,
			"observedGeneration", ds.Status.ObservedGeneration,
		)
		return false
	}

	desired := ds.Status.DesiredNumberScheduled
	updated := ds.Status.UpdatedNumberScheduled
	available := ds.Status.NumberAvailable
	ready := ds.Status.NumberReady

	if desired == 0 {
		logger.Info("Image heater daemonset has no desired pods", "daemonset", d.name)
		return true
	}

	if updated != desired {
		logger.Info("Image heater daemonset pods not updated to latest spec",
			"daemonset", d.name,
			"desiredNumberScheduled", desired,
			"updatedNumberScheduled", updated,
		)
		return false
	}

	if available != desired || ready != desired {
		logger.Info("Image heater daemonset pods not yet available",
			"daemonset", d.name,
			"desiredNumberScheduled", desired,
			"numberAvailable", available,
			"numberReady", ready,
		)
		return false
	}

	return true
}
