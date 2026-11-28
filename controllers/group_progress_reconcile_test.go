package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

// These are the Kubernetes objects visible to the restarted operator. No
// component, reconciler, or in-memory progress is transferred to the subprocess.
type bulkRestartSnapshot struct {
	Cluster      ytv1.Ytsaurus          `json:"cluster"`
	StatefulSets appsv1.StatefulSetList `json:"statefulSets"`
	Pods         corev1.PodList         `json:"pods"`
	ConfigMaps   corev1.ConfigMapList   `json:"configMaps"`
	Services     corev1.ServiceList     `json:"services"`
	Secrets      corev1.SecretList      `json:"secrets"`
	Jobs         batchv1.JobList        `json:"jobs"`
}

func (s *bulkRestartSnapshot) lists() []client.ObjectList {
	return []client.ObjectList{&s.StatefulSets, &s.Pods, &s.ConfigMaps, &s.Services, &s.Secrets, &s.Jobs}
}

// A fresh operator process must recreate a bulk group's pods after removal
// even if another group's rolling update has made no further progress.
func TestBulkGroupRestartsWhileProxyRolloutIsPending(t *testing.T) {
	ctx := t.Context()
	resource := testutil.BuildMinimalYtsaurus("test", "test")
	resource.UID = "cluster-uid"
	resource.Spec.HTTPProxies[0].MinReadyInstanceCount = ptr.To(int32(2))
	resource.Spec.YQLAgents = &ytv1.YQLAgentSpec{InstanceSpec: ytv1.InstanceSpec{
		Image: ptr.To("yql:new"), InstanceCount: 1,
	}}
	hp := ytv1.Component{Type: consts.HttpProxyType}
	yql := ytv1.Component{Type: consts.YqlAgentType}
	rolling := &ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}}
	bulk := &ytv1.ComponentUpdateStrategy{RunPreChecks: ptr.To(false)}
	resource.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{Component: hp, Strategy: rolling}, {Component: yql, Strategy: bulk}}
	resource.Status.State = ytv1.ClusterStateUpdating
	resource.Status.UpdateStatus = ytv1.UpdateStatus{
		State:              ytv1.UpdateStateWaitingForPodsRemoval,
		UpdatingComponents: []ytv1.Component{hp, yql},
		GroupProgress: []ytv1.ComponentUpdateProgress{
			{Component: hp, Strategy: rolling, Phase: ytv1.ComponentUpdatePhaseReplacing},
			{Component: yql, Strategy: bulk, Phase: ytv1.ComponentUpdatePhaseRemoving},
		},
	}
	generator := ytconfig.NewGenerator(&resource, "cluster.local")
	yqlLabels := generator.GetComponentLabeller(yql.Type, yql.Name)
	oldPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "yqla-0", Namespace: resource.Namespace, Labels: yqlLabels.GetSelectorLabelMap(),
	}}
	sts := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "yqla", Namespace: resource.Namespace},
		Spec: appsv1.StatefulSetSpec{Replicas: ptr.To(int32(1))},
	}
	// One new replica is unavailable, exhausting the rolling update budget.
	proxy := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "hp", Namespace: resource.Namespace},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To(int32(3)),
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type:          appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateStatefulSetStrategy{Partition: ptr.To(int32(2))},
			},
		},
		Status: appsv1.StatefulSetStatus{
			CurrentRevision: "old", UpdateRevision: "new",
			Replicas: 3, CurrentReplicas: 2, UpdatedReplicas: 1, ReadyReplicas: 2, AvailableReplicas: 2,
		},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, ytv1.AddToScheme(scheme))
	const restartEnv = "YTOP_TEST_RESTART_BULK_GROUP"
	resumed := os.Getenv(restartEnv) == "1"
	builder := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&resource)
	var snapshot bulkRestartSnapshot
	if resumed {
		require.NoError(t, json.NewDecoder(os.Stdin).Decode(&snapshot))
		builder.WithObjects(&snapshot.Cluster).WithLists(snapshot.lists()...)
	} else {
		builder.WithObjects(&resource, oldPod, sts, proxy)
	}
	api := builder.Build()
	key := client.ObjectKeyFromObject(&resource)
	reconcile := func() ytv1.UpdateStatus {
		t.Helper()
		// Each pass reads the CR back from storage and reconstructs all components.
		r := &YtsaurusReconciler{BaseReconciler: BaseReconciler{
			ClusterDomain: "cluster.local", Client: api, Scheme: scheme, Recorder: record.NewFakeRecorder(100),
		}}
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
		require.NoError(t, err)
		var current ytv1.Ytsaurus
		require.NoError(t, api.Get(ctx, key, &current))
		require.Equal(t, ytv1.ClusterStateUpdating, current.Status.State)
		require.NoError(t, api.Get(ctx, client.ObjectKeyFromObject(sts), sts))
		return current.Status.UpdateStatus
	}

	groupProgress := func(status ytv1.UpdateStatus, component ytv1.Component) ytv1.ComponentUpdateProgress {
		t.Helper()
		for _, group := range status.GroupProgress {
			if group.Component == component {
				return group
			}
		}
		t.Fatalf("missing progress for %v", component)
		return ytv1.ComponentUpdateProgress{}
	}
	if !resumed {
		var target string
		for range 10 {
			status := reconcile()
			if *sts.Spec.Replicas == 0 {
				target = groupProgress(status, yql).TargetRevision
				break
			}
		}
		require.Zero(t, *sts.Spec.Replicas, "the bulk rollout must first scale YQL to zero")
		require.NotEmpty(t, target, "the target must be durable before the replacement starts")
		require.NoError(t, api.Delete(ctx, oldPod))
		for range 10 {
			group := groupProgress(reconcile(), yql)
			require.Equal(t, target, group.TargetRevision)
			require.Zero(t, *sts.Spec.Replicas, "persist creation before applying it")
			if group.Phase == ytv1.ComponentUpdatePhaseCreating {
				break
			}
		}
		require.NoError(t, api.Get(ctx, key, &snapshot.Cluster))
		require.Equal(t, ytv1.ComponentUpdatePhaseCreating, groupProgress(snapshot.Cluster.Status.UpdateStatus, yql).Phase)
		for _, list := range snapshot.lists() {
			require.NoError(t, api.List(ctx, list))
		}
		var data bytes.Buffer
		require.NoError(t, json.NewEncoder(&data).Encode(snapshot))
		executable, err := os.Executable()
		require.NoError(t, err)
		childCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(childCtx, executable, "-test.run=^"+t.Name()+"$")
		cmd.Env = append(os.Environ(), restartEnv+"=1")
		cmd.Stdin = &data
		output, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "restarted operator: %s", output)
		return
	}

	target := groupProgress(snapshot.Cluster.Status.UpdateStatus, yql).TargetRevision
	require.NotEmpty(t, target)
	for range 10 {
		status := reconcile()
		require.Equal(t, target, groupProgress(status, yql).TargetRevision)
		if *sts.Spec.Replicas == 1 {
			require.True(t, meta.IsStatusConditionTrue(status.Conditions, yqlLabels.GetPodsRemovedCondition()))
			require.Equal(t, ytv1.ComponentUpdatePhaseReplacing, groupProgress(status, hp).Phase)
			require.Equal(t, "yql:new", sts.Spec.Template.Spec.Containers[0].Image)
			// Simulate the StatefulSet controller bringing the replacement pod up.
			replacement := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: oldPod.Name, Namespace: resource.Namespace, Labels: oldPod.Labels},
				Spec:       sts.Spec.Template.Spec,
				Status: corev1.PodStatus{Conditions: []corev1.PodCondition{
					{Type: corev1.PodScheduled, Status: corev1.ConditionTrue},
				}},
			}
			for _, container := range replacement.Spec.Containers {
				replacement.Status.ContainerStatuses = append(replacement.Status.ContainerStatuses, corev1.ContainerStatus{
					Name: container.Name, Started: ptr.To(true), Ready: true,
					State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
				})
			}
			require.NoError(t, api.Create(ctx, replacement))
			status = reconcile()
			require.Contains(t, status.GroupProgress, ytv1.ComponentUpdateProgress{
				Component: yql, Strategy: bulk, Phase: ytv1.ComponentUpdatePhaseWorkloadStarted,
				TargetRevision: target,
			})
			reconcile() // A subsequent restart must leave the restored workload in place.
			require.EqualValues(t, 1, *sts.Spec.Replicas)
			return
		}
	}
	t.Fatalf("YQL remained at zero while the proxy was updating")
}
