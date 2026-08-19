package components

import (
	"fmt"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testUpdateRevision = "rev-new"
	testOldRevision    = "rev-old"
)

type podOpts struct {
	revision    string
	scheduled   bool
	ready       bool
	terminating bool
}

func makeOnDeletePod(name string, o podOpts) corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{appsv1.StatefulSetRevisionLabel: o.revision},
		},
	}
	if o.terminating {
		now := metav1.Now()
		pod.DeletionTimestamp = &now
	}
	if o.scheduled {
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:   corev1.PodScheduled,
			Status: corev1.ConditionTrue,
		})
	}
	if o.ready {
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
		})
	}
	return pod
}

// makeOnDeletePods builds `total` pods at updateRevision; the first `ready` of them are
// scheduled and ready, the rest are neither (e.g. unschedulable/pending).
func makeOnDeletePods(total, ready int) []corev1.Pod {
	pods := make([]corev1.Pod, 0, total)
	for i := 0; i < total; i++ {
		isReady := i < ready
		pods = append(pods, makeOnDeletePod(fmt.Sprintf("tnd-%d", i), podOpts{
			revision:  testUpdateRevision,
			scheduled: isReady,
			ready:     isReady,
		}))
	}
	return pods
}

func TestEvaluateOnDeleteCompletion(t *testing.T) {
	tests := []struct {
		name     string
		pods     []corev1.Pod
		replicas int32
		minReady int32
		wantDone bool
	}{
		{
			name:     "all updated and ready, strict default (minReady == replicas)",
			pods:     makeOnDeletePods(10, 10),
			replicas: 10,
			minReady: 10,
			wantDone: true,
		},
		{
			name:     "one unschedulable but minReady satisfied",
			pods:     makeOnDeletePods(10, 9),
			replicas: 10,
			minReady: 2,
			wantDone: true,
		},
		{
			name:     "not enough ready pods below minReady",
			pods:     makeOnDeletePods(10, 8),
			replicas: 10,
			minReady: 9,
			wantDone: false,
		},
		{
			name:     "one unschedulable with strict default blocks",
			pods:     makeOnDeletePods(10, 9),
			replicas: 10,
			minReady: 10,
			wantDone: false,
		},
		{
			name: "pod still at old revision blocks even if enough are ready",
			pods: func() []corev1.Pod {
				pods := makeOnDeletePods(10, 10)
				pods[3].Labels[appsv1.StatefulSetRevisionLabel] = testOldRevision
				return pods
			}(),
			replicas: 10,
			minReady: 2,
			wantDone: false,
		},
		{
			name: "terminating pod blocks",
			pods: func() []corev1.Pod {
				pods := makeOnDeletePods(10, 10)
				now := metav1.Now()
				pods[0].DeletionTimestamp = &now
				return pods
			}(),
			replicas: 10,
			minReady: 2,
			wantDone: false,
		},
		{
			name:     "pod count mismatch blocks",
			pods:     makeOnDeletePods(9, 9),
			replicas: 10,
			minReady: 2,
			wantDone: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done, waitReason := evaluateOnDeleteCompletion(tt.pods, testUpdateRevision, tt.replicas, tt.minReady)
			if done != tt.wantDone {
				t.Fatalf("evaluateOnDeleteCompletion() done = %v, want %v (reason: %q)", done, tt.wantDone, waitReason)
			}
			if done && waitReason != "" {
				t.Fatalf("evaluateOnDeleteCompletion() returned done=true but non-empty waitReason %q", waitReason)
			}
			if !done && waitReason == "" {
				t.Fatalf("evaluateOnDeleteCompletion() returned done=false but empty waitReason")
			}
		})
	}
}
