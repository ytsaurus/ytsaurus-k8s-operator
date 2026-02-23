package resources

import (
	"fmt"

	"k8s.io/utils/ptr"

	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type DaemonSet struct {
	BaseManagedResource[*appsv1.DaemonSet]
}

func NewDaemonSet(
	name string,
	labeller *labeller.Labeller,
	proxy apiproxy.APIProxy,
) *DaemonSet {
	return &DaemonSet{
		BaseManagedResource: BaseManagedResource[*appsv1.DaemonSet]{
			proxy:     proxy,
			labeller:  labeller,
			name:      name,
			oldObject: &appsv1.DaemonSet{},
			newObject: &appsv1.DaemonSet{},
		},
	}
}

func (d *DaemonSet) Build() *appsv1.DaemonSet {
	d.newObject = &appsv1.DaemonSet{
		ObjectMeta: d.labeller.GetObjectMeta(d.name),
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: d.labeller.GetSelectorLabelMap(),
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: ptr.To(intstr.FromString("100%")),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      d.labeller.GetMetaLabelMap(false),
					Annotations: d.labeller.GetAnnotations(),
				},
			},
		},
	}
	return d.newObject
}

func (d *DaemonSet) Status() (ready bool, message string) {
	return GetDaemonSetStatus(d.oldObject)
}

// DaemonSet is ready when:
// - exists, not deleted, observed
// - pods for all desired nodes are updated, ready and available.
func GetDaemonSetStatus(ds *appsv1.DaemonSet) (ready bool, message string) {
	st := &ds.Status
	desired := st.DesiredNumberScheduled
	switch {
	case ds.ResourceVersion == "":
		return false, fmt.Sprintf("daemon set %v does not exist", ds.Name)
	case !ds.DeletionTimestamp.IsZero():
		return false, fmt.Sprintf("daemon set %v is deleted", ds.Name)
	case st.ObservedGeneration != ds.Generation:
		return false, fmt.Sprintf("daemon set %v is not observed", ds.Name)
	case desired == 0:
		return true, fmt.Sprintf("daemon set %v has no desired nodes", ds.Name)
	case st.CurrentNumberScheduled < desired:
		return false, fmt.Sprintf("daemon set %v scheduled pods: %v of %v", ds.Name, st.CurrentNumberScheduled, desired)
	case st.UpdatedNumberScheduled < desired:
		return false, fmt.Sprintf("daemon set %v updated pods: %v of %v", ds.Name, st.UpdatedNumberScheduled, desired)
	case st.NumberReady < desired:
		return false, fmt.Sprintf("daemon set %v ready pods: %v of %v", ds.Name, st.NumberReady, desired)
	case st.NumberAvailable < desired:
		return false, fmt.Sprintf("daemon set %v available pods: %v of %v", ds.Name, st.NumberAvailable, desired)
	}
	return true, fmt.Sprintf("daemon set %v is ready at %v nodes", ds.Name, desired)
}
