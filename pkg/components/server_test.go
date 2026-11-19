package components

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

func TestServerInitContainerCPUResources(t *testing.T) {
	tests := []struct {
		name           string
		requests       corev1.ResourceList
		cpuLimit       string
		wantCPURequest string
		wantCPULimit   string
	}{
		{
			name:         "limits only",
			cpuLimit:     "2",
			wantCPULimit: "1999m",
		},
		{
			name:         "empty requests",
			requests:     corev1.ResourceList{},
			cpuLimit:     "2",
			wantCPULimit: "1999m",
		},
		{
			name:         "only memory requested",
			requests:     corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			cpuLimit:     "2",
			wantCPULimit: "1999m",
		},
		{
			name:           "equal integer CPU request and limit",
			requests:       corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
			cpuLimit:       "2",
			wantCPURequest: "1999m",
			wantCPULimit:   "1999m",
		},
		{
			name:           "explicit zero CPU request",
			requests:       corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0")},
			cpuLimit:       "2",
			wantCPURequest: "0",
			wantCPULimit:   "2",
		},
		{
			name:           "unequal CPU request and limit",
			requests:       corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			cpuLimit:       "2",
			wantCPURequest: "1",
			wantCPULimit:   "2",
		},
		{
			name:           "fractional CPU request and limit",
			requests:       corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1500m")},
			cpuLimit:       "1500m",
			wantCPURequest: "1500m",
			wantCPULimit:   "1500m",
		},
		{
			name:         "fractional CPU limit only",
			cpuLimit:     "1500m",
			wantCPULimit: "1500m",
		},
		{
			name:         "zero CPU limit",
			cpuLimit:     "0",
			wantCPULimit: "0",
		},
		{
			name:           "CPU request without limit",
			requests:       corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")},
			wantCPURequest: "1",
		},
		{
			name: "no CPU resources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := &ytv1.InstanceSpec{
				InstanceCount: 1,
				Resources: corev1.ResourceRequirements{
					Requests: tt.requests.DeepCopy(),
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
				},
			}
			if tt.cpuLimit != "" {
				spec.Resources.Limits[corev1.ResourceCPU] = resource.MustParse(tt.cpuLimit)
			}
			originalResources := spec.Resources.DeepCopy()
			wantResources := spec.Resources.DeepCopy()
			if tt.wantCPURequest != "" {
				wantResources.Requests[corev1.ResourceCPU] = resource.MustParse(tt.wantCPURequest)
			}
			if tt.wantCPULimit != "" {
				wantResources.Limits[corev1.ResourceCPU] = resource.MustParse(tt.wantCPULimit)
			}

			l := &labeller.Labeller{
				Namespace:     "test",
				ClusterName:   "test",
				ResourceName:  "test",
				ComponentType: ytv1.DataNodeType,
			}
			s := newServerConfigured(
				l, nil, &ytv1.CommonSpec{CoreImage: "test:latest"}, &ytv1.PodSpec{}, spec,
				"ytserver", []ConfigGenerator{{FileName: "config.yson"}},
				nil, nil, nil, newServerOptions(spec, 10000),
			)
			podSpec := s.rebuildStatefulSet().Spec.Template.Spec

			require.Len(t, podSpec.InitContainers, 2)
			for _, container := range podSpec.InitContainers {
				require.True(t, equality.Semantic.DeepEqual(*wantResources, container.Resources),
					"%s resources = %v, want %v", container.Name, container.Resources, *wantResources)
			}
			require.Equal(t, *originalResources, podSpec.Containers[0].Resources)
			require.Equal(t, *originalResources, spec.Resources)
		})
	}
}

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
	for i := range total {
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
			// A draining pod (terminating, so kubelet has flipped it to not-ready) is not
			// counted toward readyCount, so completion still needs minReady other ready pods.
			name: "draining pod not counted, below minReady blocks",
			pods: func() []corev1.Pod {
				pods := makeOnDeletePods(10, 2)
				now := metav1.Now()
				pods[0].DeletionTimestamp = &now
				pods[0].Status.Conditions = nil // draining: no longer scheduled/ready
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
