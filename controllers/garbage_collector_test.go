package controllers_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/controllers"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

func TestGarbageCollectorFollowsUpdatePlan(t *testing.T) {
	for _, separate := range []bool{false, true} {
		name := "inline"
		if separate {
			name = "separate"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			scheme := runtime.NewScheme()
			require.NoError(t, clientgoscheme.AddToScheme(scheme))
			require.NoError(t, ytv1.AddToScheme(scheme))
			resource := testutil.BuildMinimalYtsaurus("test", "cluster")
			resource.UID = "cluster-uid"
			resource.Generation = 1
			resource.Spec.EphemeralCluster = true
			resource.Spec.PrimaryMasters.MinReadyInstanceCount = ptr.To(int32(0))
			resource.Spec.Discovery.MinReadyInstanceCount = ptr.To(int32(0))
			resource.Spec.HTTPProxies[0].MinReadyInstanceCount = ptr.To(int32(0))
			resource.Spec.DataNodes[0].MinReadyInstanceCount = ptr.To(int32(0))
			rpcSpec := ytv1.RPCProxiesSpec{
				Role: "removed", InstanceSpec: ytv1.InstanceSpec{InstanceCount: 1, MinReadyInstanceCount: ptr.To(int32(0))},
			}
			rpcResource := &ytv1.YtsaurusRPCProxies{
				ObjectMeta: metav1.ObjectMeta{Name: "proxies", Namespace: resource.Namespace, UID: "proxies-uid"},
				Spec: ytv1.YtsaurusRPCProxiesSpec{
					Ytsaurus: ytv1.YtsaurusReference{Name: resource.Name}, RPCProxiesSpec: rpcSpec,
				},
			}
			builder := fake.NewClientBuilder().WithScheme(scheme).
				WithStatusSubresource(&ytv1.Ytsaurus{}, rpcResource, &ytv1.YtsaurusUpdatePlan{})
			if separate {
				builder.WithObjects(rpcResource)
			} else {
				resource.Spec.RPCProxies = []ytv1.RPCProxiesSpec{rpcSpec}
			}
			c := builder.WithObjects(&resource).Build()
			reconciler := controllers.YtsaurusReconciler{BaseReconciler: controllers.BaseReconciler{
				Client: c, Scheme: scheme, Recorder: record.NewFakeRecorder(1000), ClusterDomain: "cluster.local",
			}}
			key := client.ObjectKeyFromObject(&resource)
			reconcile := func() {
				t.Helper()
				require.NoError(t, c.Get(ctx, key, &resource))
				_, err := reconciler.Sync(ctx, &resource)
				require.NoError(t, err)
				var jobs batchv1.JobList
				require.NoError(t, c.List(ctx, &jobs))
				for i := range jobs.Items {
					job := &jobs.Items[i]
					if job.Status.Succeeded == 0 {
						job.Status.Succeeded = 1
						job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
						require.NoError(t, c.Status().Update(ctx, job))
					}
				}
			}
			until := func(state ytv1.ClusterState) {
				t.Helper()
				for range 100 {
					reconcile()
					if resource.Status.State == state {
						return
					}
				}
				t.Fatalf("cluster did not reach %s: %+v", state, resource.Status)
			}
			until(ytv1.ClusterStateRunning)
			orphanKey := client.ObjectKey{Namespace: resource.Namespace, Name: "rp-removed"}
			var orphan appsv1.StatefulSet
			require.NoError(t, c.Get(ctx, orphanKey, &orphan))
			if separate {
				// Separate component changes do not change the parent generation.
				require.NoError(t, c.Delete(ctx, rpcResource))
			} else {
				resource.Spec.RPCProxies = nil
				resource.Generation++
				require.NoError(t, c.Update(ctx, &resource))
			}
			until(ytv1.ClusterStateUpdateBlocked)
			require.Contains(t, resource.Status.UpdateStatus.BlockedComponentsSummary, "gc")
			condition := meta.FindStatusCondition(resource.Status.Conditions, "GarbageCollectorReady")
			require.NotNil(t, condition)
			require.Equal(t, string(ytv1.ComponentStateNeedUpdate), condition.Reason)
			require.Contains(t, condition.Message, orphan.Name)
			require.NoError(t, c.Get(ctx, orphanKey, &orphan))

			// Selecting a different component must leave garbage collection blocked.
			resource.Spec.UpdatePlan = ytv1.UpdatePlan{{Component: ytv1.Component{Type: ytv1.DiscoveryType}}}
			resource.Generation++
			require.NoError(t, c.Update(ctx, &resource))
			for range 5 {
				reconcile()
			}
			require.NoError(t, c.Get(ctx, orphanKey, &orphan))
			gcPlan := ytv1.UpdatePlan{{Component: ytv1.Component{Type: ytv1.GarbageCollectorType}}}
			plan := &ytv1.YtsaurusUpdatePlan{
				ObjectMeta: metav1.ObjectMeta{Name: "collect", Namespace: resource.Namespace, Generation: 1},
				Spec: ytv1.YtsaurusUpdatePlanSpec{
					Ytsaurus: ytv1.YtsaurusReference{Name: resource.Name}, UpdatePlan: gcPlan, OneShot: true,
				},
			}
			resource.Spec.UpdatePlan = nil
			if separate {
				require.NoError(t, c.Create(ctx, plan))
			} else {
				resource.Spec.UpdatePlan = gcPlan
			}
			resource.Generation++
			require.NoError(t, c.Update(ctx, &resource))
			until(ytv1.ClusterStateUpdating)
			for range 100 {
				reconcile()
				if meta.IsStatusConditionTrue(resource.Status.UpdateStatus.Conditions, "GarbageCollectorPodsRemoved") {
					break
				}
			}
			require.Equal(t, ytv1.UpdateStateWaitingForPodsRemoval, resource.Status.UpdateStatus.State)
			require.True(t, meta.IsStatusConditionTrue(resource.Status.UpdateStatus.Conditions, "GarbageCollectorPodsRemoved"))

			// A new orphan can appear after collection completed but before the flow
			// advances. It must reopen the removal step rather than reuse its result.
			lateOrphan := orphan.DeepCopy()
			lateOrphan.Name = "late-orphan"
			lateOrphan.UID = "late-orphan-uid"
			lateOrphan.ResourceVersion = ""
			require.NoError(t, c.Create(ctx, lateOrphan))
			remainingOrphan := lateOrphan.DeepCopy()
			remainingOrphan.Name = "remaining-orphan"
			remainingOrphan.UID = "remaining-orphan-uid"
			remainingOrphan.ResourceVersion = ""
			require.NoError(t, c.Create(ctx, remainingOrphan))
			reconcile()
			require.Equal(t, ytv1.UpdateStateWaitingForPodsRemoval, resource.Status.UpdateStatus.State)
			require.False(t, meta.IsStatusConditionTrue(resource.Status.UpdateStatus.Conditions, "GarbageCollectorPodsRemoved"))
			require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(lateOrphan), lateOrphan)))
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(remainingOrphan), remainingOrphan), "each reconciliation must delete at most one orphan")
			reconcile()
			require.Equal(t, ytv1.UpdateStateWaitingForPodsRemoval, resource.Status.UpdateStatus.State)
			require.False(t, meta.IsStatusConditionTrue(resource.Status.UpdateStatus.Conditions, "GarbageCollectorPodsRemoved"))
			require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(remainingOrphan), remainingOrphan)))

			until(ytv1.ClusterStateRunning)
			require.True(t, apierrors.IsNotFound(c.Get(ctx, orphanKey, &orphan)))
			for _, object := range []client.Object{
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "rpc-proxies-removed", Namespace: resource.Namespace}},
				&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "yt-rpc-proxy-removed-monitoring", Namespace: resource.Namespace}},
				&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "yt-rpc-proxy-removed-config", Namespace: resource.Namespace}},
			} {
				require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(object), object)), "removed component resource must be collected: %T %s", object, object.GetName())
			}
			condition = meta.FindStatusCondition(resource.Status.Conditions, "GarbageCollectorReady")
			require.NotNil(t, condition)
			require.Equal(t, metav1.ConditionTrue, condition.Status)
			for _, name := range []string{"ms", "ds", "hp", "dnd-dn-1"} {
				require.NoError(t, c.Get(ctx, client.ObjectKey{Namespace: resource.Namespace, Name: name}, &appsv1.StatefulSet{}))
			}
			if separate {
				require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(plan), plan)))
			}
		})
	}
}
