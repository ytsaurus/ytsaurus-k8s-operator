package resources

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *BaseManagedResource[T]) listPods(ctx context.Context) *corev1.PodList {
	logger := log.FromContext(ctx)
	podList := &corev1.PodList{}
	if err := r.proxy.ListObjects(ctx, podList, r.labeller.GetListOptions()...); err != nil {
		logger.Error(err, "unable to list pods for component", "component", r.labeller.GetFullComponentName())
		return nil
	}
	return podList
}
