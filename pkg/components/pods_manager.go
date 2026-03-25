package components

import (
	"context"
	"slices"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

// TODO: move to Updatable
type podsManager interface {
	getInstanceCount() int32
	setInstanceCount(instanceCount int32)
	getMinReadyInstanceCount(margin int32) int32
	listPods(ctx context.Context) ([]corev1.Pod, error)

	removePods(ctx context.Context) error
	podsImageCorrespondsToSpec() bool
	arePodsUpdatedToNewRevision(ctx context.Context) bool
}

func podConditionIsTrue(pod *corev1.Pod, conditionType corev1.PodConditionType) bool {
	return slices.ContainsFunc(pod.Status.Conditions, func(c corev1.PodCondition) bool {
		return c.Type == conditionType && c.Status == corev1.ConditionTrue
	})
}

// Pod is scheduled to node and not deleted yet
func podIsScheduled(pod *corev1.Pod) bool {
	return podConditionIsTrue(pod, corev1.PodScheduled) && pod.DeletionTimestamp.IsZero()
}

func podContainerIsRunning(pod *corev1.Pod, name string) bool {
	// started = true - startup finished and passed startup probe
	// state.running != nil - container is running right now
	return slices.ContainsFunc(pod.Status.ContainerStatuses, func(s corev1.ContainerStatus) bool {
		return s.Name == name && ptr.Deref(s.Started, false) && s.State.Running != nil
	})
}

func podContainerIsReady(pod *corev1.Pod, name string) bool {
	return slices.ContainsFunc(pod.Status.ContainerStatuses, func(s corev1.ContainerStatus) bool {
		return s.Name == name && s.Ready
	})
}

func podContainerNames(pod *corev1.Pod, names []string) []string {
	if len(names) == 0 {
		for _, ct := range pod.Spec.Containers {
			names = append(names, ct.Name)
		}
	}
	return names
}

// Counts pods where all containers or containers with given names are scheduled, running and ready.
func countPods(pods []corev1.Pod, containerNames []string) (scheduled, running, ready int32) {
	for _, pod := range pods {
		if podIsScheduled(&pod) {
			scheduled += 1
		} else {
			continue
		}

		containersAreRunning, containersAreReady := true, true
		for _, name := range podContainerNames(&pod, containerNames) {
			containersAreRunning = containersAreRunning && podContainerIsRunning(&pod, name)
			containersAreReady = containersAreReady && podContainerIsReady(&pod, name)
		}
		if containersAreRunning {
			running += 1
		}
		if containersAreReady {
			ready += 1
		}
	}
	return scheduled, running, ready
}

func arePodsReady(
	ctx context.Context,
	manager podsManager,
	l *labeller.Labeller,
	containerNames []string,
) (status ComponentStatus, err error) {
	logger := log.FromContext(ctx)

	pods, err := manager.listPods(ctx)
	if err != nil {
		logger.Error(err, "Cannot list pods",
			"component", l.GetFullComponentName(),
			"workload", l.GetComponentShortName(),
		)
		return ComponentStatusBlocked("Cannot list pods: %v", err), err
	}

	instanceCount := manager.getInstanceCount()
	minReadyCount := manager.getMinReadyInstanceCount(0)
	scheduledPods, runningPods, readyPods := countPods(pods, containerNames)

	switch {
	case len(pods) > int(instanceCount):
		status = ComponentStatusBlocked("Too many pods: %v of %v", len(pods), instanceCount)
	case scheduledPods < minReadyCount:
		status = ComponentStatusBlocked("Not enough scheduled pods: %v of %v", scheduledPods, instanceCount)
	case runningPods < minReadyCount:
		status = ComponentStatusBlocked("Not enough running pods: %v of %v", runningPods, instanceCount)
	case readyPods < minReadyCount:
		status = ComponentStatusStarted("Not enough ready pods: %v of %v", readyPods, instanceCount)
	default:
		return ComponentStatusReadyAfter("Pods are ready: %v of %v", readyPods, instanceCount), nil
	}

	logger.Info(status.Message,
		"component", l.GetFullComponentName(),
		"workload", l.GetComponentShortName(),
		"podsCount", len(pods),
		"runningPods", runningPods,
		"instanceCount", instanceCount,
		"readyPods", readyPods,
		"minReadyCount", minReadyCount,
	)

	return status, nil
}

// TODO: Removing should become simply scaling to zero replicas using common readiness check.
func removePods(ctx context.Context, manager podsManager, c *component) error {
	removingStarted := c.labeller.GetPodsRemovingStartedCondition()
	if !c.ytsaurus.IsUpdateStatusConditionTrue(removingStarted) {
		if err := manager.removePods(ctx); err != nil {
			return err
		}
		c.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    removingStarted,
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: "Pods removing was started",
		})
		return nil
	}
	if pods, err := manager.listPods(ctx); err != nil {
		return err
	} else if len(pods) == 0 {
		c.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    c.labeller.GetPodsRemovedCondition(),
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: "Pods removed",
		})
	}
	return nil
}
