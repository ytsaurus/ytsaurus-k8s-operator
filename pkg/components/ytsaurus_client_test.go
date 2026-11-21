package components

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	mock_yt "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/mock"
)

func TestTabletCellMapReadyAfterMasterRestart(t *testing.T) {
	ctx := t.Context()
	client := mock_yt.NewMockClient(gomock.NewController(t))
	resource := &ytv1.Ytsaurus{}
	cluster := apiproxy.NewYtsaurus(resource, nil, nil, nil)
	yc := &YtsaurusClient{ytClient: client}
	yc.ytsaurus = cluster

	ready, err := yc.tabletCellMapReadyAfterMasterRestart(ctx)
	require.NoError(t, err)
	require.True(t, ready, "an update without a master restart skips the check")

	cluster.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:   cluster.GetUpdateStateCompleteCondition(ytv1.UpdateStateWaitingForMasterExitReadOnly),
		Status: metav1.ConditionTrue,
	})
	resource.Spec.EphemeralCluster = true
	resource.Spec.PrimaryMasters.MinReadyInstanceCount = ptr.To(int32(0))
	ready, err = yc.tabletCellMapReadyAfterMasterRestart(ctx)
	require.NoError(t, err)
	require.True(t, ready, "an ephemeral cluster without live masters skips Cypress")
	resource.Spec.EphemeralCluster = false
	resource.Spec.PrimaryMasters.MinReadyInstanceCount = nil

	options := &yt.ListNodeOptions{
		MasterReadOptions: &yt.MasterReadOptions{ReadFrom: yt.ReadFromLeader},
		MaxSize:           ptr.To(int64(1)),
	}
	list := func() *gomock.Call {
		return client.EXPECT().ListNode(ctx, ypath.Path(consts.TabletCellsPath), gomock.Any(), options)
	}

	list().Return(yterrors.Err(yterrors.CodeResolveError, "no such child"))
	ready, err = yc.tabletCellMapReadyAfterMasterRestart(ctx)
	require.NoError(t, err)
	require.False(t, ready)
	condition := cluster.GetUpdateStatusCondition(consts.ConditionCypressPatchApplied)
	require.NotNil(t, condition)
	require.Equal(t, metav1.ConditionFalse, condition.Status)
	require.Equal(t, "WaitingForTabletCellMap", condition.Reason)

	list().SetArg(2, []string{}).Return(nil)
	ready, err = yc.tabletCellMapReadyAfterMasterRestart(ctx)
	require.NoError(t, err)
	require.True(t, ready, "a published map can be empty")

	readErr := errors.New("master unavailable")
	list().Return(readErr)
	ready, err = yc.tabletCellMapReadyAfterMasterRestart(ctx)
	require.ErrorIs(t, err, readErr)
	require.False(t, ready)
}
