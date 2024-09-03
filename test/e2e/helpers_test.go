package controllers_test

import (
	"context"

	mapset "github.com/deckarep/golang-set/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	ctrlcli "sigs.k8s.io/controller-runtime/pkg/client"
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

type StringSet mapset.Set[string]

func NewStringSet() StringSet {
	return mapset.NewSet[string]()
}

func NewStringSetFromItems(slice ...string) StringSet {
	set := NewStringSet()
	for _, item := range slice {
		set.Add(item)
	}
	return set
}

func NewStringSetFromMap[T any](data map[string]T) StringSet {
	set := NewStringSet()
	for key := range data {
		set.Add(key)
	}
	return set
}

type podsCreationDiff struct {
	created   StringSet
	deleted   StringSet
	recreated StringSet
}

func newPodsCreationDiff() podsCreationDiff {
	return podsCreationDiff{
		created:   NewStringSet(),
		deleted:   NewStringSet(),
		recreated: NewStringSet(),
	}
}

func diffPodsCreation(before, after map[string]corev1.Pod) podsCreationDiff {
	diff := newPodsCreationDiff()
	for podName := range before {
		if _, existNow := after[podName]; !existNow {
			diff.deleted.Add(podName)
		}
	}
	for podName, podAfter := range after {
		podBefore, wasBefore := before[podName]
		if !wasBefore {
			diff.created.Add(podName)
			continue
		}
		if podBefore.CreationTimestamp.Before(&podAfter.CreationTimestamp) {
			diff.recreated.Add(podName)
		}
	}
	return diff
}
