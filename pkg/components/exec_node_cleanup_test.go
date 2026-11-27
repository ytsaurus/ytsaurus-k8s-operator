package components

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type schedulerJobsTestClient struct {
	yt.Client
	t      *testing.T
	writes int
}

func (c *schedulerJobsTestClient) SetNode(_ context.Context, _ ypath.YPath, value any, _ *yt.SetNodeOptions) error {
	c.writes++
	require.Equal(c.t, false, value)
	if c.writes == 1 {
		return errors.New("temporary write failure")
	}
	return nil
}

// After PodsUpdated, the operator retries re-enabling scheduler jobs if YT is
// unavailable or a write fails. Retries stop once it records SchedulerJobsEnabled.
func TestExecNodeRetriesSchedulerJobsCleanup(t *testing.T) {
	resource := testutil.BuildMinimalYtsaurus("test", "test")
	owner := apiproxy.NewYtsaurus(&resource, nil, nil, nil)
	client := &YtsaurusClient{}
	cfgen := ytconfig.NewGenerator(&resource, "cluster.local")
	master := NewMaster(cfgen, owner, &resource.Spec.PrimaryMasters, nil)
	master.SetStatus(ComponentStatusPending("initializing"))
	n := NewExecNode(&cfgen.NodeGenerator, owner, master, ytv1.ExecNodesSpec{
		InstanceSpec: ytv1.InstanceSpec{InstanceCount: 2},
	}, client)
	mode := &ytv1.ComponentUpdateStrategy{RollingUpdate: &ytv1.ComponentRollingUpdateMode{}}
	resource.Spec.UpdatePlan = []ytv1.ComponentUpdateSelector{{Component: n.GetComponent(), Strategy: mode}}
	resource.Status.State = ytv1.ClusterStateUpdating
	resource.Status.UpdateStatus = ytv1.UpdateStatus{
		State:              ytv1.UpdateStateWaitingForPodsRemoval,
		UpdatingComponents: []ytv1.Component{n.GetComponent()},
	}
	n.server = &updateProgressServer{
		server:       n.server,
		targetStatus: ComponentStatusReady(),
		rolling:      stsRollingStatus{totalCount: 2, updatedReplicas: 2, availableReplicas: 2},
	}
	setPodsUpdatedCondition(t.Context(), owner, n)
	expectPending := func(dry bool) {
		t.Helper()
		status, err := n.Sync(t.Context(), dry)
		require.NoError(t, err)
		require.Equal(t, SyncStatusPending, status.SyncStatus)
		require.Contains(t, status.Message, "scheduler jobs")
	}
	expectPending(true)
	expectPending(false) // No YT client yet, but other groups can still pass the removal barrier.
	require.True(t, owner.IsUpdateStatusConditionTrue(n.GetLabeller().GetPodsUpdatedCondition()))
	writer := &schedulerJobsTestClient{t: t}
	client.ytClient = writer
	expectPending(false)
	expectPending(true)
	require.Equal(t, 1, writer.writes)
	for _, dry := range []bool{false, true, false} {
		status, err := n.Sync(t.Context(), dry)
		require.NoError(t, err)
		require.Equal(t, SyncStatusBlocked, status.SyncStatus, "after cleanup the normal master dependency applies")
		require.True(t, owner.IsUpdateStatusConditionTrue(n.GetLabeller().GetCondition(consts.ConditionSchedulerJobsEnabled)))
		require.Equal(t, 3, writer.writes, "two successful ordinal writes, then no further writes")
	}
}
