package controllers

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

// A selector at its concurrency limit can yield to a later selector for the
// same component type. Each admitted group must keep the policy it actually got.
func TestGroupAdmissionKeepsSelectedPolicy(t *testing.T) {
	first := ytv1.Component{Type: ytv1.HttpProxyType}
	second := ytv1.Component{Type: ytv1.HttpProxyType, Name: "internal"}
	rolling := &ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}}
	onDelete := &ytv1.ComponentUpdateStrategy{OnDelete: &ytv1.ComponentOnDeleteUpdateMode{}, RunPreChecks: ptr.To(false)}
	cm := &ComponentManager{status: ComponentManagerStatus{needUpdate: []ytv1.Component{first, second}}}
	cm.applyUpdatePlan([]ytv1.ComponentUpdateSelector{
		{Component: first, Strategy: rolling, Concurrency: ptr.To(int32(1))},
		{Component: second, Strategy: onDelete},
	})
	require.Equal(t, []ytv1.Component{first, second}, cm.status.canUpdate)
	require.Equal(t, []ytv1.ComponentUpdateProgress{
		{Component: first, Strategy: rolling, Phase: ytv1.ComponentUpdatePhasePending},
		{Component: second, Strategy: onDelete, Phase: ytv1.ComponentUpdatePhasePending},
	}, cm.status.admitted)
	*onDelete.RunPreChecks = true
	require.False(t, *cm.status.admitted[1].Strategy.RunPreChecks)
}

// An ordinary explicit master plan selects one cell; Everything and cell maintenance retain their wider selection.
func TestMasterPlanAdmission(t *testing.T) {
	primary := ytv1.Component{Type: consts.MasterType}
	secondary := ytv1.Component{Type: consts.MasterType, Name: "12"}
	otherSecondary := ytv1.Component{Type: consts.MasterType, Name: "13"}
	proxy := ytv1.Component{Type: consts.HttpProxyType}
	all := []ytv1.Component{primary, secondary, otherSecondary, proxy}

	check := func(plan []ytv1.ComponentUpdateSelector, maintenance bool, want []ytv1.Component) {
		t.Helper()
		cm := &ComponentManager{status: ComponentManagerStatus{needUpdate: all, mastersMaintenance: maintenance}}
		cm.applyUpdatePlan(plan)
		require.Equal(t, want, cm.status.canUpdate)
	}
	for _, cell := range []ytv1.Component{primary, secondary} {
		check([]ytv1.ComponentUpdateSelector{{Component: cell}}, false, []ytv1.Component{cell})
	}
	check([]ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassEverything}}, false, all)
	check([]ytv1.ComponentUpdateSelector{{Component: primary}}, true, []ytv1.Component{primary, secondary, otherSecondary})
	check([]ytv1.ComponentUpdateSelector{{Component: primary}, {Component: proxy}}, false, []ytv1.Component{proxy})
}

// Everything keeps a stopped data node at zero through master recovery, even
// when its own pods are already gone and the master pods appear ready.
func TestBulkGroupRecreatesAfterSharedMasterRecovery(t *testing.T) {
	ctx := t.Context()
	resource := testutil.BuildMinimalYtsaurus("test", "test")
	master := ytv1.Component{Type: consts.MasterType}
	node := ytv1.Component{Type: consts.DataNodeType, Name: resource.Spec.DataNodes[0].Name}
	resource.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{Class: consts.ComponentClassEverything}}
	resource.Status.State = ytv1.ClusterStateUpdating
	resource.Status.UpdateStatus = ytv1.UpdateStatus{
		UpdatingComponents: []ytv1.Component{master, node},
		GroupProgress: []ytv1.ComponentUpdateProgress{
			{Component: master, Phase: ytv1.ComponentUpdatePhaseCreating},
			{Component: node, Phase: ytv1.ComponentUpdatePhaseCreating},
		},
	}
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, ytv1.AddToScheme(scheme))
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: resource.Namespace, Name: "dnd-" + node.Name},
		Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To(int32(0))},
	}
	api := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&resource, sts).Build()
	owner := apiproxy.NewYtsaurus(&resource, api, record.NewFakeRecorder(100), scheme)
	cm, err := NewComponentManager(ctx, owner, "cluster.local")
	require.NoError(t, err)
	var data components.Component
	for _, c := range cm.allComponents {
		if c.GetComponent() == master {
			c.SetStatus(components.ComponentStatusReady())
		}
		if c.GetComponent() == node {
			data = c
		}
	}
	require.NotNil(t, data)
	for _, name := range []string{data.GetLabeller().GetPodsRemovingStartedCondition(), data.GetLabeller().GetPodsRemovedCondition()} {
		owner.SetUpdateStatusCondition(ctx, metav1.Condition{Type: name, Status: metav1.ConditionTrue})
	}
	for _, state := range []ytv1.UpdateState{
		ytv1.UpdateStateWaitingForPodsRemoval,
		ytv1.UpdateStateWaitingForMasterReady,
		ytv1.UpdateStateWaitingForMasterExitReadOnly,
		ytv1.UpdateStateWaitingForPodsCreation,
	} {
		owner.SetUpdateState(state)
		require.NoError(t, data.Fetch(ctx))
		status, err := cm.observeComponent(ctx, data)
		require.NoError(t, err)
		if status.SyncStatus == components.SyncStatusPending || status.SyncStatus == components.SyncStatusUpdating {
			_, err = data.Sync(ctx, false)
			require.NoError(t, err)
		}
		require.NoError(t, api.Get(ctx, client.ObjectKeyFromObject(sts), sts))
		if state == ytv1.UpdateStateWaitingForPodsCreation {
			require.Equal(t, resource.Spec.DataNodes[0].InstanceCount, *sts.Spec.Replicas)
		} else {
			require.Zero(t, *sts.Spec.Replicas, state)
		}
	}
}

// Target changes invalidate an in-place group's completion markers without
// replaying a bulk scale-down or restarting a group that already finished.
func TestGroupTargetDriftPreservesProgress(t *testing.T) {
	resource := testutil.BuildMinimalYtsaurus("test", "test")
	builder := testutil.YtsaurusBuilder{Ytsaurus: &resource}
	builder.WithExecNodes()
	exec := ytv1.Component{Type: ytv1.ExecNodeType}
	proxy := ytv1.Component{Type: ytv1.HttpProxyType}
	resource.Status.State = ytv1.ClusterStateUpdating
	resource.Status.UpdateStatus = ytv1.UpdateStatus{
		State: ytv1.UpdateStateWaitingForPodsRemoval, UpdatingComponents: []ytv1.Component{exec, proxy},
		GroupProgress: []ytv1.ComponentUpdateProgress{
			{Component: exec, Phase: ytv1.ComponentUpdatePhaseReplacing, Strategy: &ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}}},
			{Component: proxy, Phase: ytv1.ComponentUpdatePhaseCreating},
		},
	}
	owner := apiproxy.NewYtsaurus(&resource, nil, nil, nil)
	// Construct fresh components after each spec edit, as the reconciler does.
	refresh := func(id ytv1.Component) *ComponentManager {
		t.Helper()
		cm, err := NewComponentManager(t.Context(), owner, "cluster.local")
		require.NoError(t, err)
		for _, c := range cm.allComponents {
			if c.GetComponent() == id {
				require.NoError(t, cm.refreshGroupTarget(c))
				return cm
			}
		}
		t.Fatalf("component %v missing", id)
		return nil
	}
	refresh(exec)
	refresh(proxy)
	e, p := owner.GetGroupProgress(exec), owner.GetGroupProgress(proxy)
	firstExec, firstProxy := e.TargetRevision, p.TargetRevision
	require.NotEmpty(t, firstExec)
	require.NotEmpty(t, firstProxy)
	execUpdated := exec.String() + consts.ConditionPodsUpdated
	execJobs := exec.String() + consts.ConditionSchedulerJobsEnabled
	peer := ytv1.Component{Type: ytv1.ExecNodeType, Name: "gpu"}
	peerUpdated := peer.String() + consts.ConditionPodsUpdated
	peerJobs := peer.String() + consts.ConditionSchedulerJobsEnabled
	proxyRemoved := proxy.String() + consts.ConditionPodsRemoved
	for _, name := range []string{execUpdated, execJobs, peerUpdated, peerJobs, proxyRemoved} {
		owner.SetUpdateStatusCondition(t.Context(), metav1.Condition{Type: name, Status: metav1.ConditionTrue})
	}

	resource.Spec.ExecNodes[0].Image = ptr.To("exec:v2")
	resource.Spec.HTTPProxies[0].Image = ptr.To("proxy:v2")
	refresh(exec)
	require.NotEqual(t, firstExec, e.TargetRevision)
	require.Equal(t, ytv1.ComponentUpdatePhaseReplacing, e.Phase)
	require.Nil(t, owner.GetUpdateStatusCondition(execUpdated))
	require.Nil(t, owner.GetUpdateStatusCondition(execJobs))
	require.True(t, owner.IsUpdateStatusConditionTrue(peerUpdated))
	require.True(t, owner.IsUpdateStatusConditionTrue(peerJobs))
	require.True(t, owner.IsUpdateStatusConditionTrue(proxyRemoved))
	require.Equal(t, firstProxy, p.TargetRevision)
	refresh(proxy)
	require.NotEqual(t, firstProxy, p.TargetRevision)
	require.Equal(t, ytv1.ComponentUpdatePhaseCreating, p.Phase)
	require.True(t, owner.IsUpdateStatusConditionTrue(proxyRemoved))

	e.Phase = ytv1.ComponentUpdatePhaseWorkloadStarted
	finished := e.TargetRevision
	resource.Spec.ExecNodes[0].Image = ptr.To("exec:v3")
	require.True(t, refresh(exec).deferred[exec])
	require.Equal(t, finished, e.TargetRevision)
	require.Equal(t, ytv1.ComponentUpdatePhaseWorkloadStarted, e.Phase)
}
