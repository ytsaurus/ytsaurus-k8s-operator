package apiproxy

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

// A bulk group can recreate during the global removal phase. Batches with masters
// and the coordinated master-cell maintenance flow follow the cluster phases.
func TestComponentUpdateState(t *testing.T) {
	proxy := ytv1.Component{Type: ytv1.HttpProxyType}
	master := ytv1.Component{Type: ytv1.MasterType}
	resource := &ytv1.Ytsaurus{}
	update := &resource.Status.UpdateStatus
	update.State = ytv1.UpdateStateWaitingForPodsRemoval
	update.UpdatingComponents = []ytv1.Component{proxy}
	update.GroupProgress = []ytv1.ComponentUpdateProgress{{Component: proxy, Phase: ytv1.ComponentUpdatePhaseCreating}}
	owner := NewYtsaurus(resource, nil, nil, nil)

	require.Equal(t, ytv1.UpdateStateWaitingForPodsCreation, owner.GetComponentUpdateState(proxy))
	resource.Spec.ClusterMaintenance = &ytv1.ClusterMaintenance{Shutdown: ytv1.ClusterShutdownExceptMasters}
	require.Equal(t, ytv1.UpdateStateWaitingForPodsRemoval, owner.GetComponentUpdateState(proxy))

	resource.Spec.ClusterMaintenance = nil
	update.UpdatingComponents = append(update.UpdatingComponents, master)
	update.GroupProgress = append(update.GroupProgress, ytv1.ComponentUpdateProgress{Component: master, Phase: ytv1.ComponentUpdatePhaseCreating})
	require.Equal(t, ytv1.UpdateStateWaitingForPodsRemoval, owner.GetComponentUpdateState(proxy))
	require.Equal(t, ytv1.UpdateStateWaitingForPodsRemoval, owner.GetComponentUpdateState(master))
	update.State = ytv1.UpdateStateWaitingForMasterReady
	require.Equal(t, ytv1.UpdateStateWaitingForMasterReady, owner.GetComponentUpdateState(master))
	update.State = ytv1.UpdateStateWaitingForPodsCreation
	require.Equal(t, ytv1.UpdateStateWaitingForPodsCreation, owner.GetComponentUpdateState(proxy))
}

func TestGroupPreChecksUseAdmittedPolicy(t *testing.T) {
	group := ytv1.Component{Type: ytv1.HttpProxyType, Name: "internal"}
	resource := &ytv1.Ytsaurus{}
	resource.Status.UpdateStatus.GroupProgress = []ytv1.ComponentUpdateProgress{{
		Component: group,
		Strategy:  &ytv1.ComponentUpdateStrategy{RunPreChecks: ptr.To(false)},
	}}
	owner := NewYtsaurus(resource, nil, nil, nil)
	name := group.String()
	require.False(t, owner.ShouldRunGroupPreChecks(group, name))

	resource.Status.UpdateStatus.GroupProgress[0].Strategy.RunPreChecks = ptr.To(true)
	resource.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{
		Component: ytv1.Component{Type: group.Type},
		Strategy:  &ytv1.ComponentUpdateStrategy{RunPreChecks: ptr.To(false)},
	}}
	require.True(t, owner.ShouldRunGroupPreChecks(group, name))
	resource.Status.UpdateStatus.Conditions = []metav1.Condition{{Type: name + "Ready", Status: metav1.ConditionTrue}}
	require.False(t, owner.ShouldRunGroupPreChecks(group, name))
}

func TestPreChecksBeforeNextReplacement(t *testing.T) {
	exec := ytv1.Component{Type: ytv1.ExecNodeType}
	resource := &ytv1.Ytsaurus{}
	update := &resource.Status.UpdateStatus
	update.Conditions = []metav1.Condition{{Type: exec.String() + "Ready", Status: metav1.ConditionTrue}}
	update.GroupProgress = []ytv1.ComponentUpdateProgress{{
		Component: exec, Phase: ytv1.ComponentUpdatePhaseReplacing,
		Strategy: &ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}},
	}}
	owner := NewYtsaurus(resource, nil, nil, nil)
	progress := owner.GetGroupProgress(exec)
	require.True(t, owner.ShouldRunGroupPreChecks(exec, exec.String()), "the next partition needs its own drain check")
	progress.Strategy.RunPreChecks = ptr.To(false)
	require.False(t, owner.ShouldRunGroupPreChecks(exec, exec.String()))
	progress.Strategy = &ytv1.ComponentUpdateStrategy{OnDelete: &ytv1.ComponentOnDeleteUpdateMode{}}
	require.False(t, owner.ShouldRunGroupPreChecks(exec, exec.String()))
	progress.Strategy = &ytv1.ComponentUpdateStrategy{}
	progress.Phase = ytv1.ComponentUpdatePhaseRemoving
	require.False(t, owner.ShouldRunGroupPreChecks(exec, exec.String()))
}
