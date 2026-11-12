package controllers_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func TestOneShotPlanWaitsForUpdatePreparation(t *testing.T) {
	ctx := t.Context()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, ytv1.AddToScheme(scheme))

	resource := testutil.BuildMinimalYtsaurus("test", "cluster")
	resource.Generation = 1
	resource.Spec.EphemeralCluster = true
	resource.Spec.PrimaryMasters.MinReadyInstanceCount = ptr.To(int32(0))
	resource.Spec.Discovery.MinReadyInstanceCount = ptr.To(int32(0))
	resource.Spec.HTTPProxies[0].MinReadyInstanceCount = ptr.To(int32(0))
	resource.Spec.DataNodes[0].MinReadyInstanceCount = ptr.To(int32(0))
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&ytv1.Ytsaurus{}, &ytv1.YtsaurusUpdatePlan{}).
		WithObjects(&resource).Build()
	reconciler := controllers.YtsaurusReconciler{BaseReconciler: controllers.BaseReconciler{
		Client: k8sClient, Scheme: scheme, Recorder: record.NewFakeRecorder(1000), ClusterDomain: "cluster.local",
	}}
	key := client.ObjectKeyFromObject(&resource)
	syncCluster := func() {
		t.Helper()
		require.NoError(t, k8sClient.Get(ctx, key, &resource))
		_, err := reconciler.Sync(ctx, &resource)
		require.NoError(t, err)

		var jobs batchv1.JobList
		require.NoError(t, k8sClient.List(ctx, &jobs))
		for i := range jobs.Items {
			job := &jobs.Items[i]
			if job.Status.Succeeded == 0 {
				job.Status.Succeeded = 1
				job.Status.Conditions = append(job.Status.Conditions, batchv1.JobCondition{
					Type: batchv1.JobComplete, Status: corev1.ConditionTrue,
				})
				require.NoError(t, k8sClient.Status().Update(ctx, job))
			}
		}
	}
	for range 100 {
		syncCluster()
		if resource.Status.State == ytv1.ClusterStateRunning {
			break
		}
	}
	require.Equal(t, ytv1.ClusterStateRunning, resource.Status.State)

	// Submit the spec change and its one-shot plan before either is reconciled.
	image := testYtsaurusImage + "-planned"
	resource.Spec.Discovery.Image = &image
	resource.Generation++
	require.NoError(t, k8sClient.Update(ctx, &resource))
	plan := &ytv1.YtsaurusUpdatePlan{
		ObjectMeta: metav1.ObjectMeta{Name: "plan", Namespace: resource.Namespace, Generation: 1},
		Spec: ytv1.YtsaurusUpdatePlanSpec{
			Ytsaurus:   ytv1.YtsaurusReference{Name: resource.Name},
			OneShot:    true,
			UpdatePlan: ytv1.UpdatePlan{{Class: ytv1.ComponentClassEverything}},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, plan))
	syncCluster()
	require.NoError(t, k8sClient.Get(ctx, key, &resource))
	require.Equal(t, ytv1.ClusterStatePreparing, resource.Status.State)
	require.NoError(t, k8sClient.Get(ctx, client.ObjectKeyFromObject(plan), plan))

	for range 100 {
		syncCluster()
		if resource.Status.State == ytv1.ClusterStateRunning {
			break
		}
	}
	require.Equal(t, ytv1.ClusterStateRunning, resource.Status.State)
	var discovery appsv1.StatefulSet
	require.NoError(t, k8sClient.Get(ctx, client.ObjectKey{Namespace: resource.Namespace, Name: "ds"}, &discovery))
	require.Equal(t, image, discovery.Spec.Template.Spec.Containers[0].Image)
	require.True(t, apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(plan), plan)))
}
