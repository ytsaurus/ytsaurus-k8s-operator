package resources

import (
	"context"

	"k8s.io/utils/ptr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	labeller2 "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type Deployment struct {
	BaseManagedResource[*appsv1.Deployment]

	ytsaurus     apiproxy.Ytsaurus
	tolerations  []corev1.Toleration
	nodeSelector map[string]string
	built        bool
}

func NewDeployment(
	name string,
	labeller *labeller2.Labeller,
	ytsaurus *apiproxy.Ytsaurus,
	tolerations []corev1.Toleration,
	nodeSelector map[string]string,
) *Deployment {
	return &Deployment{
		BaseManagedResource: BaseManagedResource[*appsv1.Deployment]{
			proxy:     ytsaurus,
			labeller:  labeller,
			name:      name,
			oldObject: &appsv1.Deployment{},
			newObject: &appsv1.Deployment{},
		},
		ytsaurus:     *ytsaurus,
		tolerations:  tolerations,
		nodeSelector: nodeSelector,
	}
}

func (d *Deployment) Build() *appsv1.Deployment {
	if !d.built {
		d.newObject.ObjectMeta = d.labeller.GetObjectMeta(d.name)
		d.newObject.Spec = appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: d.labeller.GetSelectorLabelMap(),
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

func (d *Deployment) GetReplicas() int32 {
	return ptr.Deref(d.oldObject.Spec.Replicas, 1)
}

func (d *Deployment) ListPods(ctx context.Context) ([]corev1.Pod, error) {
	var podList corev1.PodList
	if err := d.proxy.ListObjects(ctx, &podList, d.labeller.GetListOptions()...); err != nil {
		return nil, err
	}
	return podList.Items, nil
}
