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

// Pod is terminating.
func podIsTerminating(pod *corev1.Pod) bool {
	return !pod.DeletionTimestamp.IsZero()
}

// Pod is scheduled to node.
func podIsScheduled(pod *corev1.Pod) bool {
	return podConditionIsTrue(pod, corev1.PodScheduled)
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

// Counts pods where all containers or containers with given names have passed certain startup phase.
func countPods(pods []corev1.Pod, containerNames []string) (pending, scheduled, running, ready, terminating int32) {
	for _, pod := range pods {
		if podIsTerminating(&pod) {
			terminating += 1
			continue
		}
		if podIsScheduled(&pod) {
			scheduled += 1
		} else {
			pending += 1
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
	return pending, scheduled, running, ready, terminating
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
	pendingPods, scheduledPods, runningPods, readyPods, terminatingPods := countPods(pods, containerNames)

	switch {
	case len(pods) > int(instanceCount):
		status = ComponentStatusBlocked("Too many pods: %v of %v, running %v, terminating %v", len(pods), instanceCount, runningPods, terminatingPods)
	case runningPods < minReadyCount:
		status = ComponentStatusBlocked("Not enough running pods: %v of %v, pending %v, scheduled %v, need %v", runningPods, instanceCount, pendingPods, scheduledPods, minReadyCount)
	case readyPods < minReadyCount:
		status = ComponentStatusStarted("Not enough ready pods: %v of %v, running %v, need %v", readyPods, instanceCount, runningPods, minReadyCount)
	default:
		return ComponentStatusReadyAfter("Pods are ready: %v of %v, running %v, need %v", readyPods, instanceCount, runningPods, minReadyCount), nil
	}

	logger.Info(status.Message,
		"component", l.GetFullComponentName(),
		"workload", l.GetComponentShortName(),
		"podsCount", len(pods),
		"scheduledPods", scheduledPods,
		"runningPods", runningPods,
		"terminatingPods", terminatingPods,
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
