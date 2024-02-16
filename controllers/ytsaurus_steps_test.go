package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.ytsaurus.tech/yt/go/yt"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type fakeComponent struct {
	name   string
	status components.ComponentStatus
}

func (c fakeComponent) Fetch(context.Context) error { return nil }
func (c fakeComponent) Sync(context.Context) error  { return nil }
func (c fakeComponent) Sync2(context.Context) error { return nil }
func (c fakeComponent) Status(context.Context) components.ComponentStatus {
	return c.status
}
func (c fakeComponent) Status2(context.Context) (components.ComponentStatus, error) {
	return c.status, nil
}
func (c fakeComponent) GetName() string {
	return c.name
}
func (c fakeComponent) GetLabel() string {
	return c.name + "-label"
}
func (c fakeComponent) SetReadyCondition(components.ComponentStatus) {}
func (c fakeComponent) IsUpdatable() bool                            { return false }

type fakeYtClient struct {
	fakeComponent
}

func (yc *fakeYtClient) GetYtClient() yt.Client { return nil }
func (yc *fakeYtClient) HandlePossibilityCheck(context.Context) (bool, string, error) {
	return false, "", nil
}
func (yc *fakeYtClient) EnableSafeMode(context.Context) error                   { return nil }
func (yc *fakeYtClient) DisableSafeMode(context.Context) error                  { return nil }
func (yc *fakeYtClient) IsSafeModeEnabled(context.Context) (bool, error)        { return false, nil }
func (yc *fakeYtClient) SaveTableCellsAndUpdateState(ctx context.Context) error { return nil }
func (yc *fakeYtClient) IsTableCellsSaved() bool                                { return false }
func (yc *fakeYtClient) RemoveTableCells(context.Context) error                 { return nil }
func (yc *fakeYtClient) RecoverTableCells(context.Context) error                { return nil }
func (yc *fakeYtClient) AreTabletCellsRemoved(context.Context) (bool, error)    { return false, nil }
func (yc *fakeYtClient) AreTabletCellsRecovered(context.Context) (bool, error)  { return false, nil }
func (yc *fakeYtClient) StartBuildMasterSnapshots(ctx context.Context) error    { return nil }
func (yc *fakeYtClient) IsMasterReadOnly(context.Context) (bool, error)         { return false, nil }

func TestStepChosenCorrectly(t *testing.T) {
	httpProxies := []components.Component2{
		fakeComponent{"HTTPProxy", components.SimpleStatus(components.SyncStatusReady)},
	}
	dataNodes := []components.Component2{
		fakeComponent{"DataNodes", components.SimpleStatus(components.SyncStatusReady)},
	}
	ytClient := fakeYtClient{
		fakeComponent{
			name:   "ytclient",
			status: components.SimpleStatus(components.SyncStatusReady),
		},
	}
	store := componentsStore{
		discovery:   fakeComponent{"Discovery", components.SimpleStatus(components.SyncStatusReady)},
		master:      fakeComponent{"Master", components.SimpleStatus(components.SyncStatusNeedLocalUpdate)},
		httpProxies: httpProxies,
		ytClient:    &ytClient,
		dataNodes:   dataNodes,
	}

	ytsaurusStatus := ytv1.YtsaurusStatus{
		State:        ytv1.ClusterStateRunning,
		Conditions:   nil,
		UpdateStatus: ytv1.UpdateStatus{State: ytv1.UpdateStateNone},
	}
	steps, err := NewYtsaurusSteps(store, ytsaurusStatus)
	require.NoError(t, err)

	step, status, err := steps.getNextStep(context.Background())
	require.NoError(t, err)
	require.NotNil(t, status)
	require.Equal(t, "Master", string(step.GetName()))
}
