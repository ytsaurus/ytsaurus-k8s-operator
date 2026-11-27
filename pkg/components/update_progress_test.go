package components

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

// Only observations are stubbed. Any attempt to sync Kubernetes resources
// during the status pass would have to use the unconfigured Kubernetes client.
type updateProgressServer struct {
	server
	rolling      stsRollingStatus
	targetStatus ComponentStatus
	onDeleteDone bool
}

func (s *updateProgressServer) needUpdate() ComponentStatus { return s.targetStatus }
func (s *updateProgressServer) getRollingUpdateStatus(context.Context) (*stsRollingStatus, error) {
	return &s.rolling, nil
}
func (s *updateProgressServer) arePodsUpdatedToNewRevision(context.Context) bool {
	return s.onDeleteDone
}

func newUpdateProgressProxy(strategy *ytv1.ComponentUpdateStrategy) (*HttpProxy, *updateProgressServer) {
	resource := testutil.BuildMinimalYtsaurus("test", "test")
	resource.Spec.HTTPProxies[0].InstanceCount = 3
	owner := apiproxy.NewYtsaurus(&resource, nil, nil, nil)
	hp := NewHTTPProxy(ytconfig.NewGenerator(&resource, "cluster.local"), owner, nil, resource.Spec.HTTPProxies[0])
	resource.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{Component: hp.GetComponent(), Strategy: strategy}}
	resource.Status.State = ytv1.ClusterStateUpdating
	resource.Status.UpdateStatus = ytv1.UpdateStatus{
		State:              ytv1.UpdateStateWaitingForPodsRemoval,
		UpdatingComponents: []ytv1.Component{hp.GetComponent()},
	}
	observed := &updateProgressServer{
		server:       hp.server,
		targetStatus: ComponentStatusReady(),
		rolling:      stsRollingStatus{totalCount: 3, updatedReplicas: 3, availableReplicas: 3},
	}
	hp.server = observed
	return hp, observed
}

func checkUpdateProgress(t *testing.T, hp *HttpProxy, want SyncStatus) ComponentStatus {
	t.Helper()
	resource := hp.ytsaurus.GetResource()
	before := resource.Status.UpdateStatus.DeepCopy()
	status, err := hp.Sync(t.Context(), true)
	require.NoError(t, err)
	require.Equal(t, want, status.SyncStatus)
	require.Equal(t, *before, resource.Status.UpdateStatus, "observation must not record progress")
	return status
}

// The status check used to report Updating even after the rolling update finished
// and the operator recorded PodsUpdated for the current target.
func TestRollingUpdateProgress(t *testing.T) {
	hp, observed := newUpdateProgressProxy(&ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}})
	update := &hp.ytsaurus.GetResource().Status.UpdateStatus
	unrecorded := update.DeepCopy()
	for range 2 {
		// A failed status write leaves the next reconcile with the unrecorded CR.
		*update = *unrecorded.DeepCopy()
		checkUpdateProgress(t, hp, SyncStatusUpdating)
		status, err := hp.Sync(t.Context(), false)
		require.NoError(t, err)
		require.True(t, status.IsReady())
		require.True(t, hp.ytsaurus.IsUpdateStatusConditionTrue(hp.GetLabeller().GetPodsUpdatedCondition()))
		checkUpdateProgress(t, hp, SyncStatusReady)
	}

	// An old completion condition cannot mark a changed target or unfinished roll ready.
	observed.targetStatus = ComponentStatusNeedUpdate("image update")
	checkUpdateProgress(t, hp, SyncStatusUpdating)
	observed.targetStatus = ComponentStatusReady()
	observed.rolling.partition = 1
	observed.rolling.updatedReplicas = 2
	status := checkUpdateProgress(t, hp, SyncStatusUpdating)
	require.Contains(t, status.Message, "2 of 3")
}

// OnDelete used to keep waiting for manual replacement even after all pods
// reached the target revision and the operator recorded PodsUpdated.
func TestOnDeleteUpdateProgress(t *testing.T) {
	hp, observed := newUpdateProgressProxy(&ytv1.ComponentUpdateStrategy{OnDelete: &ytv1.ComponentOnDeleteUpdateMode{}})
	checkUpdateProgress(t, hp, SyncStatusPending)
	hp.ytsaurus.SetUpdateStatusCondition(t.Context(), metav1.Condition{
		Type: hp.GetLabeller().GetWaitingOnDeleteUpdateCondition(), Status: metav1.ConditionTrue,
	})
	observed.onDeleteDone = true
	checkUpdateProgress(t, hp, SyncStatusPending)
	setPodsUpdatedCondition(t.Context(), hp.ytsaurus, hp)
	checkUpdateProgress(t, hp, SyncStatusReady)
	observed.targetStatus = ComponentStatusNeedUpdate("image update")
	checkUpdateProgress(t, hp, SyncStatusPending)
	observed.targetStatus = ComponentStatusReady()
	observed.onDeleteDone = false
	checkUpdateProgress(t, hp, SyncStatusPending)
}

// A query or queue agent can use a tablet group that is waiting for manual
// replacement, provided enough replicas for that group are still serving.
func TestServingDependencyDuringGroupReplacement(t *testing.T) {
	resource := testutil.BuildMinimalYtsaurus("test", "test")
	owner := apiproxy.NewYtsaurus(&resource, nil, nil, nil)
	cfgen := ytconfig.NewGenerator(&resource, "cluster.local")
	tablets := NewTabletNode(&cfgen.NodeGenerator, owner, nil, nil, ytv1.TabletNodesSpec{
		Name:         "hot",
		InstanceSpec: ytv1.InstanceSpec{InstanceCount: 3, MinReadyInstanceCount: ptr.To(int32(2))},
	}, false)
	tablets.SetStatus(ComponentStatusPending("waiting for manual pod replacement"))
	resource.Status.UpdateStatus.State = ytv1.UpdateStateWaitingForPodsRemoval
	resource.Status.UpdateStatus.GroupProgress = []ytv1.ComponentUpdateProgress{{
		Component: tablets.GetComponent(), Phase: ytv1.ComponentUpdatePhaseReplacing,
		Strategy: &ytv1.ComponentUpdateStrategy{OnDelete: &ytv1.ComponentOnDeleteUpdateMode{}},
	}}
	sts := tablets.server.(*serverImpl).statefulSet.OldObject()
	sts.ResourceVersion = "1"
	sts.Generation = 2
	sts.Spec.Replicas = ptr.To(int32(3))
	sts.Status.ObservedGeneration = sts.Generation
	sts.Status.AvailableReplicas = 2
	require.True(t, servingDependencyStatus(owner, tablets).IsReady())
	sts.Status.AvailableReplicas = 1
	require.False(t, servingDependencyStatus(owner, tablets).IsRunning())
	sts.Status.AvailableReplicas = 3
	sts.Status.ObservedGeneration--
	require.False(t, servingDependencyStatus(owner, tablets).IsRunning())
}

func TestGroupWorkloadRevisionAndMode(t *testing.T) {
	resource := testutil.BuildMinimalYtsaurus("test", "test")
	recorder := record.NewFakeRecorder(10)
	owner := apiproxy.NewYtsaurus(&resource, nil, recorder, nil)
	cfgen := ytconfig.NewGenerator(&resource, "cluster.local")
	hp := NewHTTPProxy(cfgen, owner, nil, resource.Spec.HTTPProxies[0])
	resource.Spec.UI = &ytv1.UISpec{}
	ui := NewUI(cfgen, owner, nil)
	revision := func(c Component) string {
		t.Helper()
		hash, err := DesiredWorkloadRevision(c)
		require.NoError(t, err)
		require.NotEmpty(t, hash)
		return hash
	}
	initialProxy, initialUI := revision(hp), revision(ui)
	fresh := ytconfig.NewGenerator(&resource, "cluster.local")
	require.Equal(t, initialProxy, revision(NewHTTPProxy(fresh, owner, nil, resource.Spec.HTTPProxies[0])))
	require.Equal(t, initialUI, revision(NewUI(fresh, owner, nil)))
	hp.server.(*serverImpl).image = "updated-proxy"
	require.NotEqual(t, initialProxy, revision(hp))
	require.Equal(t, initialUI, revision(ui), "a change in another group preserves this group's target")
	configs := ui.microservice.(*microserviceImpl).configs
	configs.AddGenerator("extra", ConfigFormatText, func() ([]byte, error) {
		return []byte("extra UI config"), nil
	})
	extended := revision(ui)
	require.NotEqual(t, initialUI, extended)
	configs.overridesMap.ResourceVersion = "1"
	configs.overridesMap.Data = map[string]string{"header-extra": "an override"}
	require.NotEqual(t, extended, revision(ui))
	require.Empty(t, recorder.Events, "observing the rollout target must not emit config-apply events")
	rolling := &ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}}
	require.True(t, WorkloadReplacementMode(hp, rolling))
	require.False(t, WorkloadReplacementMode(ui, rolling), "UI's deployment is always updated in bulk")
}

// A status conflict after scaling to zero can lose both the successful YQL
// precheck and the removal marker. Counting the departed agents again would stall.
func TestBulkResumesAfterUnrecordedScaleDown(t *testing.T) {
	resource := testutil.BuildMinimalYtsaurus("test", "test")
	resource.Spec.YQLAgents = &ytv1.YQLAgentSpec{InstanceSpec: ytv1.InstanceSpec{InstanceCount: 3}}
	owner := apiproxy.NewYtsaurus(&resource, nil, nil, nil)
	yql := NewYQLAgent(ytconfig.NewGenerator(&resource, "cluster.local"), owner, nil, nil)
	resource.Status.State = ytv1.ClusterStateUpdating
	resource.Status.UpdateStatus = ytv1.UpdateStatus{
		State: ytv1.UpdateStateWaitingForPodsRemoval, UpdatingComponents: []ytv1.Component{yql.GetComponent()},
		GroupProgress: []ytv1.ComponentUpdateProgress{{
			Component: yql.GetComponent(), Strategy: &ytv1.ComponentUpdateStrategy{}, Phase: ytv1.ComponentUpdatePhaseRemoving,
		}},
	}
	sts := yql.server.(*serverImpl).statefulSet.OldObject()
	sts.ResourceVersion = "1"
	sts.Spec.Replicas = ptr.To(int32(3))
	status, err := yql.Sync(t.Context(), true)
	require.NoError(t, err)
	require.Equal(t, SyncStatusBlocked, status.SyncStatus, "prechecks still run before scale-down")
	sts.Spec.Replicas = ptr.To(int32(0))
	status, err = yql.Sync(t.Context(), true)
	require.NoError(t, err)
	require.Equal(t, SyncStatusUpdating, status.SyncStatus)
}

// A changed pod template must be submitted even when its image and config
// still match. A pre-upgrade workload without an annotation can be adopted.
func TestGroupTemplateDriftDuringReplacement(t *testing.T) {
	resource := testutil.BuildMinimalYtsaurus("test", "test")
	owner := apiproxy.NewYtsaurus(&resource, nil, nil, nil)
	hp := NewHTTPProxy(ytconfig.NewGenerator(&resource, "cluster.local"), owner, nil, resource.Spec.HTTPProxies[0])
	s := hp.server.(*serverImpl)
	sts := s.statefulSet.OldObject()
	for _, obj := range []metav1.Object{sts, s.headlessService.OldObject(), s.monitoringService.OldObject(), s.configs.configMap.OldObject()} {
		obj.SetResourceVersion("1")
	}
	initial, err := DesiredWorkloadRevision(hp)
	require.NoError(t, err)
	sts.Annotations = map[string]string{consts.GroupUpdateTargetAnnotationName: initial}
	s.instanceSpec.VolumeMounts = append(s.instanceSpec.VolumeMounts, corev1.VolumeMount{Name: "scratch", MountPath: "/scratch"})
	updated, err := DesiredWorkloadRevision(hp)
	require.NoError(t, err)
	require.NotEqual(t, initial, updated)
	resource.Status.State = ytv1.ClusterStateUpdating
	resource.Status.UpdateStatus = ytv1.UpdateStatus{
		State: ytv1.UpdateStateWaitingForPodsRemoval,
		GroupProgress: []ytv1.ComponentUpdateProgress{{
			Component: hp.GetComponent(), TargetRevision: updated, Phase: ytv1.ComponentUpdatePhaseReplacing,
		}},
	}
	require.Equal(t, "workload target changed", s.needUpdate().Message)
	require.False(t, needsWorkloadTargetSync(updated, nil), "legacy workloads use observed rollout state")
	resource.Status.State = ytv1.ClusterStateRunning
	require.Empty(t, activeWorkloadTarget(owner, hp.GetLabeller()))
}
