package controllers_test

import (
	"context"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/utils/ptr"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

func getComponentPods(ctx context.Context, namespace string) map[string]corev1.Pod {
	podlist := corev1.PodList{}
	noJobPodsReq, err := labels.NewRequirement("job-name", selection.DoesNotExist, []string{})
	Expect(err).Should(Succeed())
	selector := labels.NewSelector()
	selector = selector.Add(*noJobPodsReq)
	err = k8sClient.List(
		ctx,
		&podlist,
		ctrlcli.InNamespace(namespace),
		ctrlcli.MatchingLabelsSelector{Selector: selector},
	)
	Expect(err).Should(Succeed())

	result := make(map[string]corev1.Pod)
	for _, pod := range podlist.Items {
		result[pod.Name] = pod
	}
	return result
}

func getAllPods(ctx context.Context, namespace string) []corev1.Pod {
	podList := corev1.PodList{}
	err := k8sClient.List(ctx, &podList, ctrlcli.InNamespace(namespace))
	Expect(err).Should(Succeed())
	return podList.Items
}

type changedObjects struct {
	Deleted, Updated, Created []string
}

func getChangedPods(before, after map[string]corev1.Pod) changedObjects {
	var ret changedObjects
	for name := range before {
		if _, found := after[name]; !found {
			ret.Deleted = append(ret.Deleted, name)
		}
	}
	for name, podAfter := range after {
		if !podAfter.DeletionTimestamp.IsZero() {
			ret.Deleted = append(ret.Deleted, name)
		} else if podBefore, found := before[name]; !found {
			ret.Created = append(ret.Created, name)
		} else if !podAfter.CreationTimestamp.Equal(&podBefore.CreationTimestamp) {
			ret.Updated = append(ret.Updated, name)
		}
	}
	return ret
}

// updateSpecToTriggerAllComponentUpdate is a helper
// that introduce spec change which should trigger change in all component static configs
// and thus trigger all components update.
func updateSpecToTriggerAllComponentUpdate(ytsaurus *ytv1.Ytsaurus) {
	// ForceTCP is a field that is used in all components static configs
	// so changing it should trigger all components update.
	// Any value should be ok for test purposes.
	if ytsaurus.Spec.ForceTCP == nil {
		ytsaurus.Spec.ForceTCP = ptr.To(true)
	} else {
		ytsaurus.Spec.ForceTCP = ptr.To(!*ytsaurus.Spec.ForceTCP)
	}
}
