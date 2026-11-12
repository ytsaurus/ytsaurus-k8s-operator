package components

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

func newGarbageCollectorTest(t *testing.T, shortNames bool, hooks interceptor.Funcs) (*GarbageCollector, client.Client, string) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, ytv1.AddToScheme(scheme))
	resource := testutil.BuildMinimalYtsaurus("namespace", "cluster")
	resource.UID = "cluster-uid"
	resource.Spec.UseShortNames = shortNames
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(hooks).Build()
	proxy := apiproxy.NewYtsaurus(&resource, k8sClient, record.NewFakeRecorder(100), scheme)
	cfgen := ytconfig.NewGenerator(proxy.Cluster, "cluster.local")
	active := NewRPCProxy(cfgen, proxy, nil, &ytv1.RPCProxiesSpec{Role: "active"})
	gc := NewGarbageCollector(cfgen, proxy, func() []Component { return []Component{active} })
	return gc, k8sClient, active.GetLabeller().GetInstanceLabelValue(false)
}

func createGarbageCollectorObject(t *testing.T, gc *GarbageCollector, c client.Client, object client.Object, name, label string) {
	t.Helper()
	object.SetName(name)
	object.SetNamespace(gc.labeller.Namespace)
	object.SetUID(types.UID(fmt.Sprintf("%T-%s-uid", object, name)))
	if label != "" {
		object.SetLabels(map[string]string{consts.YTComponentLabelName: label})
	}
	require.NoError(t, ctrl.SetControllerReference(gc.ytsaurus.GetResource(), object, c.Scheme()))
	require.NoError(t, c.Create(t.Context(), object))
}

func createGarbageCollectorStatefulSet(t *testing.T, gc *GarbageCollector, c client.Client, name string) *appsv1.StatefulSet {
	t.Helper()
	sts := &appsv1.StatefulSet{}
	label := gc.labeller.ForComponent(ytv1.RpcProxyType, "removed").GetInstanceLabelValue(false)
	createGarbageCollectorObject(t, gc, c, sts, name, label)
	return sts
}

func TestGarbageCollectorDetectsOnlyOwnedOrphans(t *testing.T) {
	for _, shortNames := range []bool{false, true} {
		name := "full-names"
		if shortNames {
			name = "short-names"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			gc, c, activeLabel := newGarbageCollectorTest(t, shortNames, interceptor.Funcs{})
			active := createGarbageCollectorStatefulSet(t, gc, c, "arbitrary-name")
			active.Labels[consts.YTComponentLabelName] = activeLabel
			require.NoError(t, c.Update(ctx, active))
			orphan := createGarbageCollectorStatefulSet(t, gc, c, "orphan")
			otherOwner := createGarbageCollectorStatefulSet(t, gc, c, "other-owner")
			otherOwner.OwnerReferences[0].UID = "other-uid"
			require.NoError(t, c.Update(ctx, otherOwner))
			nonController := createGarbageCollectorStatefulSet(t, gc, c, "non-controller")
			nonController.OwnerReferences[0].Controller = ptr.To(false)
			require.NoError(t, c.Update(ctx, nonController))
			unowned := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "unowned", Namespace: gc.labeller.Namespace}}
			require.NoError(t, c.Create(ctx, unowned))
			otherNamespace := orphan.DeepCopy()
			otherNamespace.Namespace = "other"
			otherNamespace.ResourceVersion = ""
			require.NoError(t, c.Create(ctx, otherNamespace))
			config := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "unused-config", Namespace: gc.labeller.Namespace}}
			require.NoError(t, ctrl.SetControllerReference(gc.ytsaurus.GetResource(), config, c.Scheme()))
			require.NoError(t, c.Create(ctx, config))

			require.NoError(t, gc.Fetch(ctx))
			require.Equal(t, SyncStatusNeedUpdate, gc.NeedUpdate().SyncStatus)
			require.Contains(t, gc.NeedUpdate().Message, orphan.Name)
			require.Len(t, gc.orphans, 1)
			gc.ytsaurus.Status.State = ytv1.ClusterStateUpdating
			gc.ytsaurus.SetUpdatingComponents([]ytv1.Component{{Type: ytv1.GarbageCollectorType}})
			gc.ytsaurus.SetUpdateState(ytv1.UpdateStateWaitingForPodsRemoval)
			_, err := gc.Sync(ctx, false)
			require.NoError(t, err)
			require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(orphan), orphan)))
			for _, obj := range []client.Object{active, otherOwner, nonController, unowned, otherNamespace, config} {
				require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(obj), obj))
			}
		})
	}
}

func TestGarbageCollectorClassifiesResourcesByComponentLabel(t *testing.T) {
	for _, object := range []client.Object{
		&appsv1.StatefulSet{}, &appsv1.Deployment{}, &corev1.Service{}, &corev1.ConfigMap{}, &corev1.Secret{},
	} {
		t.Run(fmt.Sprintf("%T", object), func(t *testing.T) {
			ctx := t.Context()
			gc, c, activeLabel := newGarbageCollectorTest(t, true, interceptor.Funcs{})
			orphanLabel := gc.labeller.ForComponent(ytv1.RpcProxyType, "removed").GetInstanceLabelValue(false)
			create := func(name, label string) client.Object {
				t.Helper()
				resource := object.DeepCopyObject().(client.Object)
				createGarbageCollectorObject(t, gc, c, resource, name, label)
				return resource
			}
			active := create("arbitrary-name", activeLabel)
			initJob := create("init-config", activeLabel+"-init-job")
			unlabeled := create("unlabeled", "")
			emptyLabel := create("empty-label", "")
			emptyLabel.SetLabels(map[string]string{consts.YTComponentLabelName: ""})
			require.NoError(t, c.Update(ctx, emptyLabel))
			// Even the expected workload name is orphaned when its label selects a removed component.
			orphan := create("rp-active", orphanLabel)
			otherOwner := create("other-owner", orphanLabel)
			owners := otherOwner.GetOwnerReferences()
			owners[0].UID = "other-cluster-uid"
			otherOwner.SetOwnerReferences(owners)
			require.NoError(t, c.Update(ctx, otherOwner))
			nonController := create("non-controller", orphanLabel)
			owners = nonController.GetOwnerReferences()
			owners[0].Controller = ptr.To(false)
			nonController.SetOwnerReferences(owners)
			require.NoError(t, c.Update(ctx, nonController))
			unowned := create("unowned", orphanLabel)
			unowned.SetOwnerReferences(nil)
			require.NoError(t, c.Update(ctx, unowned))
			otherNamespace := orphan.DeepCopyObject().(client.Object)
			otherNamespace.SetNamespace("other")
			otherNamespace.SetResourceVersion("")
			require.NoError(t, c.Create(ctx, otherNamespace))

			require.NoError(t, gc.Fetch(ctx))
			require.Len(t, gc.orphans, 1)
			gvk, err := c.GroupVersionKindFor(orphan)
			require.NoError(t, err)
			require.Equal(t, "Orphan resources: "+gvk.Kind+"/"+orphan.GetName(), gc.NeedUpdate().Message)
			gc.ytsaurus.Status.State = ytv1.ClusterStateUpdating
			gc.ytsaurus.SetUpdatingComponents([]ytv1.Component{{Type: ytv1.GarbageCollectorType}})
			gc.ytsaurus.SetUpdateState(ytv1.UpdateStateWaitingForPodsRemoval)
			_, err = gc.Sync(ctx, false)
			require.NoError(t, err)
			require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(orphan), orphan)))
			for _, preserved := range []client.Object{active, initJob, unlabeled, emptyLabel, otherOwner, nonController, unowned, otherNamespace} {
				require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(preserved), preserved))
			}
		})
	}
}

func TestGarbageCollectorPreservesUnsupportedResourceKinds(t *testing.T) {
	ctx := t.Context()
	gc, c, _ := newGarbageCollectorTest(t, true, interceptor.Funcs{})
	objects := []client.Object{&corev1.Pod{}, &corev1.PersistentVolumeClaim{}, &batchv1.Job{}, &appsv1.DaemonSet{}}
	label := gc.labeller.ForComponent(ytv1.RpcProxyType, "removed").GetInstanceLabelValue(false)
	for _, object := range objects {
		createGarbageCollectorObject(t, gc, c, object, "orphan", label)
	}
	require.NoError(t, gc.Fetch(ctx))
	require.True(t, gc.NeedUpdate().IsReady())
	gc.ytsaurus.Status.State = ytv1.ClusterStateUpdating
	gc.ytsaurus.SetUpdatingComponents([]ytv1.Component{{Type: ytv1.GarbageCollectorType}})
	gc.ytsaurus.SetUpdateState(ytv1.UpdateStateWaitingForPodsRemoval)
	_, err := gc.Sync(ctx, false)
	require.NoError(t, err)
	for _, object := range objects {
		require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(object), object))
	}
}

func TestGarbageCollectorSerializesDeletionAcrossResourceKinds(t *testing.T) {
	ctx := t.Context()
	var deletions []string
	gc, c, _ := newGarbageCollectorTest(t, true, interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, object client.Object, opts ...client.DeleteOption) error {
			gvk, err := c.GroupVersionKindFor(object)
			require.NoError(t, err)
			deletions = append(deletions, gvk.Kind+"/"+object.GetName())
			options := (&client.DeleteOptions{}).ApplyOptions(opts)
			require.Equal(t, metav1.DeletePropagationBackground, *options.PropagationPolicy)
			require.Equal(t, object.GetUID(), *options.Preconditions.UID)
			require.Equal(t, object.GetResourceVersion(), *options.Preconditions.ResourceVersion)
			return c.Delete(ctx, object, opts...)
		},
	})
	objects := []client.Object{&appsv1.StatefulSet{}, &appsv1.Deployment{}, &corev1.Service{}, &corev1.ConfigMap{}, &corev1.Secret{}}
	label := gc.labeller.ForComponent(ytv1.RpcProxyType, "removed").GetInstanceLabelValue(false)
	want := []string{"StatefulSet/orphan", "Deployment/orphan", "Service/orphan", "ConfigMap/orphan", "Secret/orphan"}
	// Same names across kinds must remain distinct in status and deletion tracking.
	for _, object := range objects {
		object.SetFinalizers([]string{"test/finalizer"})
		createGarbageCollectorObject(t, gc, c, object, "orphan", label)
	}
	gc.ytsaurus.Status.State = ytv1.ClusterStateUpdating
	gc.ytsaurus.SetUpdatingComponents([]ytv1.Component{{Type: ytv1.GarbageCollectorType}})
	gc.ytsaurus.SetUpdateState(ytv1.UpdateStateWaitingForPodsRemoval)
	condition := gc.labeller.GetPodsRemovedCondition()
	for i, object := range objects {
		require.NoError(t, gc.Fetch(ctx))
		require.Contains(t, gc.NeedUpdate().Message, want[i])
		_, err := gc.Sync(ctx, false)
		require.NoError(t, err)
		require.Equal(t, want[:i+1], deletions)
		for range 2 {
			require.NoError(t, gc.Fetch(ctx))
			_, err = gc.Sync(ctx, false)
			require.NoError(t, err)
			require.Equal(t, want[:i+1], deletions, "a terminating resource must block all subsequent kinds")
			require.False(t, gc.ytsaurus.IsUpdateStatusConditionTrue(condition))
		}
		require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(object), object))
		require.NotNil(t, object.GetDeletionTimestamp())
		object.SetFinalizers(nil)
		require.NoError(t, c.Update(ctx, object))
	}
	require.NoError(t, gc.Fetch(ctx))
	_, err := gc.Sync(ctx, false)
	require.NoError(t, err)
	require.True(t, gc.ytsaurus.IsUpdateStatusConditionTrue(condition))
	require.True(t, gc.NeedUpdate().IsReady())
}

func TestGarbageCollectorOnlyDeletesDuringSelectedUpdate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		state    ytv1.ClusterState
		phase    ytv1.UpdateState
		selected ytv1.ComponentType
		dry      bool
	}{
		{name: "initializing", state: ytv1.ClusterStateInitializing},
		{name: "running", state: ytv1.ClusterStateRunning},
		{name: "blocked", state: ytv1.ClusterStateUpdateBlocked},
		{name: "unselected", state: ytv1.ClusterStateUpdating, phase: ytv1.UpdateStateWaitingForPodsRemoval, selected: ytv1.DiscoveryType},
		{name: "before removal", state: ytv1.ClusterStateUpdating, selected: ytv1.GarbageCollectorType},
		{name: "after removal", state: ytv1.ClusterStateUpdating, phase: ytv1.UpdateStateWaitingForPodsCreation, selected: ytv1.GarbageCollectorType},
		{name: "dry run", state: ytv1.ClusterStateUpdating, phase: ytv1.UpdateStateWaitingForPodsRemoval, selected: ytv1.GarbageCollectorType, dry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gc, c, _ := newGarbageCollectorTest(t, true, interceptor.Funcs{})
			orphan := createGarbageCollectorStatefulSet(t, gc, c, "orphan")
			require.NoError(t, gc.Fetch(t.Context()))
			gc.ytsaurus.Status.State = tc.state
			gc.ytsaurus.SetUpdateState(tc.phase)
			gc.ytsaurus.SetUpdatingComponents([]ytv1.Component{{Type: tc.selected}})
			_, err := gc.Sync(t.Context(), tc.dry)
			require.NoError(t, err)
			require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(orphan), orphan))
			require.Empty(t, gc.ytsaurus.Status.UpdateStatus.Conditions)
		})
	}
}

func TestGarbageCollectorWaitsForBackgroundDeletionBeforeNextObject(t *testing.T) {
	ctx := t.Context()
	var deletions []string
	gc, c, _ := newGarbageCollectorTest(t, true, interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			options := (&client.DeleteOptions{}).ApplyOptions(opts)
			require.Equal(t, metav1.DeletePropagationBackground, *options.PropagationPolicy)
			require.Equal(t, obj.GetUID(), *options.Preconditions.UID)
			require.Equal(t, obj.GetResourceVersion(), *options.Preconditions.ResourceVersion)
			deletions = append(deletions, obj.GetName())
			return c.Delete(ctx, obj, opts...)
		},
	})
	orphan := createGarbageCollectorStatefulSet(t, gc, c, "orphan-b")
	orphan.Finalizers = []string{"test/finalizer"}
	require.NoError(t, c.Update(ctx, orphan))
	next := createGarbageCollectorStatefulSet(t, gc, c, "orphan-c")
	gc.ytsaurus.Status.State = ytv1.ClusterStateUpdating
	gc.ytsaurus.SetUpdatingComponents([]ytv1.Component{{Type: ytv1.GarbageCollectorType}})
	gc.ytsaurus.SetUpdateState(ytv1.UpdateStateWaitingForPodsRemoval)
	condition := gc.labeller.GetPodsRemovedCondition()
	require.NoError(t, gc.Fetch(ctx))
	_, err := gc.Sync(ctx, false)
	require.NoError(t, err)
	require.Equal(t, []string{orphan.Name}, deletions, "a sync must delete at most one StatefulSet")
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(orphan), orphan))
	require.NotNil(t, orphan.DeletionTimestamp)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(next), next))

	// A new orphan sorting before the terminating object must not bypass the wait.
	earlier := createGarbageCollectorStatefulSet(t, gc, c, "orphan-a")
	for range 2 {
		require.NoError(t, gc.Fetch(ctx))
		status, err := gc.Sync(ctx, false)
		require.NoError(t, err)
		require.Equal(t, SyncStatusUpdating, status.SyncStatus)
		require.False(t, gc.ytsaurus.IsUpdateStatusConditionTrue(condition))
		require.Equal(t, []string{orphan.Name}, deletions, "collection must wait until the terminating object disappears")
		require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(earlier), earlier))
		require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(next), next))
	}
	orphan.Finalizers = nil
	require.NoError(t, c.Update(ctx, orphan))
	require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(orphan), orphan)))
	require.NoError(t, gc.Fetch(ctx))
	_, err = gc.Sync(ctx, false)
	require.NoError(t, err)
	require.Equal(t, []string{orphan.Name, earlier.Name}, deletions, "immediate deletion must still return before deleting the next object")
	require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(earlier), earlier)))
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(next), next))
	require.False(t, gc.ytsaurus.IsUpdateStatusConditionTrue(condition))

	require.NoError(t, gc.Fetch(ctx))
	_, err = gc.Sync(ctx, false)
	require.NoError(t, err)
	require.Equal(t, []string{orphan.Name, earlier.Name, next.Name}, deletions)
	require.True(t, apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(next), next)))
	require.False(t, gc.ytsaurus.IsUpdateStatusConditionTrue(condition), "completion requires observing an empty orphan list")
	require.NoError(t, gc.Fetch(ctx))
	_, err = gc.Sync(ctx, false)
	require.NoError(t, err)
	require.True(t, gc.ytsaurus.IsUpdateStatusConditionTrue(condition))
	require.Equal(t, SyncStatusReady, gc.NeedUpdate().SyncStatus)
}

func TestGarbageCollectorErrors(t *testing.T) {
	for _, operation := range []string{"list", "delete"} {
		t.Run(operation, func(t *testing.T) {
			failure := errors.New(operation + " failed")
			hooks := interceptor.Funcs{}
			if operation == "list" {
				hooks.List = func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error { return failure }
			} else {
				hooks.Delete = func(context.Context, client.WithWatch, client.Object, ...client.DeleteOption) error { return failure }
			}
			gc, c, _ := newGarbageCollectorTest(t, true, hooks)
			orphan := createGarbageCollectorStatefulSet(t, gc, c, "orphan")
			gc.ytsaurus.Status.State = ytv1.ClusterStateUpdating
			gc.ytsaurus.SetUpdatingComponents([]ytv1.Component{{Type: ytv1.GarbageCollectorType}})
			gc.ytsaurus.SetUpdateState(ytv1.UpdateStateWaitingForPodsRemoval)
			err := gc.Fetch(t.Context())
			if operation == "delete" {
				require.NoError(t, err)
				_, err = gc.Sync(t.Context(), false)
			}
			require.ErrorIs(t, err, failure)
			require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(orphan), orphan))
		})
	}
}

func TestGarbageCollectorRechecksCompletedRemoval(t *testing.T) {
	ctx := t.Context()
	gc, c, _ := newGarbageCollectorTest(t, true, interceptor.Funcs{})
	gc.ytsaurus.Status.State = ytv1.ClusterStateUpdating
	gc.ytsaurus.SetUpdatingComponents([]ytv1.Component{{Type: ytv1.GarbageCollectorType}})
	gc.ytsaurus.SetUpdateState(ytv1.UpdateStateWaitingForPodsRemoval)
	condition := gc.labeller.GetPodsRemovedCondition()
	require.NoError(t, gc.Fetch(ctx))
	_, err := gc.Sync(ctx, false)
	require.NoError(t, err)
	require.True(t, gc.ytsaurus.IsUpdateStatusConditionTrue(condition))

	orphan := createGarbageCollectorStatefulSet(t, gc, c, "new-orphan")
	require.NoError(t, gc.Fetch(ctx))
	_, err = gc.Sync(ctx, true)
	require.NoError(t, err)
	require.False(t, gc.ytsaurus.IsUpdateStatusConditionTrue(condition), "new orphans must invalidate the previous completion before the update flow advances")
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(orphan), orphan), "dry sync must preserve the orphan")

	_, err = gc.Sync(ctx, false)
	require.NoError(t, err)
	require.NoError(t, gc.Fetch(ctx))
	_, err = gc.Sync(ctx, false)
	require.NoError(t, err)
	require.True(t, gc.ytsaurus.IsUpdateStatusConditionTrue(condition))
}

func TestGarbageCollectorSkipsUnsafeOwners(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		name := "missing UID"
		if deleting {
			name = "deleting cluster"
		}
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			gc, c, _ := newGarbageCollectorTest(t, true, interceptor.Funcs{})
			orphan := createGarbageCollectorStatefulSet(t, gc, c, "orphan")
			require.NoError(t, gc.Fetch(ctx))
			require.True(t, gc.NeedUpdate().IsNeedUpdate())
			if deleting {
				now := metav1.Now()
				gc.ytsaurus.DeletionTimestamp = &now
			} else {
				gc.ytsaurus.UID = ""
			}
			require.NoError(t, gc.Fetch(ctx))
			require.True(t, gc.NeedUpdate().IsReady(), "unsafe owners must clear previously fetched orphans")
			gc.ytsaurus.Status.State = ytv1.ClusterStateUpdating
			gc.ytsaurus.SetUpdatingComponents([]ytv1.Component{{Type: ytv1.GarbageCollectorType}})
			gc.ytsaurus.SetUpdateState(ytv1.UpdateStateWaitingForPodsRemoval)
			_, err := gc.Sync(ctx, false)
			require.NoError(t, err)
			require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(orphan), orphan))
		})
	}
}

func TestGarbageCollectorRechecksOwnershipAfterConflict(t *testing.T) {
	ctx := t.Context()
	gc, c, _ := newGarbageCollectorTest(t, true, interceptor.Funcs{})
	orphan := createGarbageCollectorStatefulSet(t, gc, c, "orphan")
	require.NoError(t, gc.Fetch(ctx))
	gc.ytsaurus.Status.State = ytv1.ClusterStateUpdating
	gc.ytsaurus.SetUpdatingComponents([]ytv1.Component{{Type: ytv1.GarbageCollectorType}})
	gc.ytsaurus.SetUpdateState(ytv1.UpdateStateWaitingForPodsRemoval)

	// Ownership changes after the list but before deletion. The version
	// precondition must reject deletion, and the next fetch must preserve it.
	orphan.OwnerReferences[0].UID = "other-cluster-uid"
	require.NoError(t, c.Update(ctx, orphan))
	_, err := gc.Sync(ctx, false)
	require.True(t, apierrors.IsConflict(err), "expected a deletion conflict, got %v", err)
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(orphan), orphan))
	require.False(t, gc.ytsaurus.IsUpdateStatusConditionTrue(gc.labeller.GetPodsRemovedCondition()))
	require.NoError(t, gc.Fetch(ctx))
	_, err = gc.Sync(ctx, false)
	require.NoError(t, err)
	require.True(t, gc.ytsaurus.IsUpdateStatusConditionTrue(gc.labeller.GetPodsRemovedCondition()))
	require.NoError(t, c.Get(ctx, client.ObjectKeyFromObject(orphan), orphan))
}

func TestGarbageCollectorToleratesAlreadyDeletedOrphans(t *testing.T) {
	ctx := t.Context()
	gc, c, _ := newGarbageCollectorTest(t, true, interceptor.Funcs{})
	orphan := createGarbageCollectorStatefulSet(t, gc, c, "orphan")
	require.NoError(t, gc.Fetch(ctx))
	require.NoError(t, c.Delete(ctx, orphan))
	gc.ytsaurus.Status.State = ytv1.ClusterStateUpdating
	gc.ytsaurus.SetUpdatingComponents([]ytv1.Component{{Type: ytv1.GarbageCollectorType}})
	gc.ytsaurus.SetUpdateState(ytv1.UpdateStateWaitingForPodsRemoval)
	_, err := gc.Sync(ctx, false)
	require.NoError(t, err, "another actor deleting the orphan must not fail collection")
	require.NoError(t, gc.Fetch(ctx))
	_, err = gc.Sync(ctx, false)
	require.NoError(t, err)
	require.True(t, gc.ytsaurus.IsUpdateStatusConditionTrue(gc.labeller.GetPodsRemovedCondition()))
}
