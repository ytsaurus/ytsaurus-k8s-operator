package components

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

// During RollingUpdate or OnDelete, the YT client can use the proxy as soon as
// its Service exists and the current StatefulSet status shows an available replica.
func TestHTTPProxyClientDependencyDuringReplacement(t *testing.T) {
	resource := testutil.BuildMinimalYtsaurus("test", "test")
	proxy := apiproxy.NewYtsaurus(&resource, nil, nil, nil)
	cfgen := ytconfig.NewGenerator(&resource, "cluster.local")
	hp := NewHTTPProxy(cfgen, proxy, nil, resource.Spec.HTTPProxies[0])
	hp.SetStatus(ComponentStatusUpdating("replacing pods"))
	resource.Status.State = ytv1.ClusterStateUpdating
	resource.Status.UpdateStatus.State = ytv1.UpdateStateWaitingForPodsRemoval
	resource.Status.UpdateStatus.UpdatingComponents = []ytv1.Component{hp.GetComponent()}
	resource.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{
		Component: ytv1.Component{Type: consts.HttpProxyType},
		Strategy:  &ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}},
	}}
	sts := hp.server.(*serverImpl).statefulSet.OldObject()
	sts.ResourceVersion = "1"
	sts.Generation = 2
	sts.Spec.Replicas = ptr.To(int32(3))
	sts.Status.ObservedGeneration = sts.Generation
	sts.Status.AvailableReplicas = 1

	require.False(t, hp.clientDependencyStatus().IsReady(), "proxy service does not exist")
	hp.balancingService.OldObject().ResourceVersion = "1"
	require.True(t, hp.clientDependencyStatus().IsReady(), "one old-revision replica can serve")
	sts.Status.ObservedGeneration--
	require.False(t, hp.clientDependencyStatus().IsReady(), "stale StatefulSet observations cannot establish availability")
	sts.Status.ObservedGeneration = sts.Generation
	sts.Spec.Replicas = ptr.To(int32(0))
	require.False(t, hp.clientDependencyStatus().IsReady(), "a scaled-down workload may still report an available replica")
	sts.Spec.Replicas = ptr.To(int32(3))

	hp.SetStatus(ComponentStatusReady())
	sts.Status.UpdatedReplicas = *sts.Spec.Replicas
	sts.Status.AvailableReplicas = 0
	require.False(t, hp.clientDependencyStatus().IsReady(), "an updated revision can have no serving pods")

	hp.SetStatus(ComponentStatusUpdating("replacing pods"))
	sts.Status.AvailableReplicas = 1
	resource.Spec.UpdatePlan[0].Strategy = &ytv1.ComponentUpdateStrategy{OnDelete: &ytv1.ComponentOnDeleteUpdateMode{}}
	require.True(t, hp.clientDependencyStatus().IsReady(), "manual pod replacement also keeps serving")
	resource.Spec.UpdatePlan[0].Strategy = &ytv1.ComponentUpdateStrategy{}
	require.False(t, hp.clientDependencyStatus().IsReady(), "bulk scale-down must preserve the lifecycle gate")

	// A peer can use a bulk proxy again while the batch is still removing pods.
	resource.Status.UpdateStatus.GroupProgress = []ytv1.ComponentUpdateProgress{{
		Component: hp.GetComponent(), Phase: ytv1.ComponentUpdatePhaseCreating,
	}}
	require.True(t, hp.clientDependencyStatus().IsReady())
	sts.Status.AvailableReplicas = 0
	require.False(t, hp.clientDependencyStatus().IsReady())
}
