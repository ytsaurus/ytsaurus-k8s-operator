package ytflow

import (
	"context"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type fakeComponent struct {
	name   ComponentName
	status syncStatus
	spy    *executionSpy
}

type fakeMasterComponent struct {
	fakeComponent
}

type fakeSchedulerComponent struct {
	fakeComponent
}

type fakeQueryTrackerComponent struct {
	fakeComponent
}

func newFakeComponent(name ComponentName, spy *executionSpy) *fakeComponent {
	return &fakeComponent{
		name:   name,
		status: components.SyncStatusPending,
		spy:    spy,
	}
}

func (c *fakeComponent) GetName() string               { return string(c.name) }
func (c *fakeComponent) Fetch(_ context.Context) error { return nil }
func (c *fakeComponent) Status(_ context.Context) componentStatus {
	return componentStatus{
		SyncStatus: c.status,
		Message:    "test comp status",
	}
}
func (c *fakeComponent) SetStatus(status components.SyncStatus) {
	c.status = status
}
func (c *fakeComponent) Sync(_ context.Context) error {
	c.status = components.SyncStatusReady
	c.spy.record(string(c.name))
	return nil
}

func newFakeMasterComponent(spy *executionSpy) *fakeMasterComponent {
	return &fakeMasterComponent{
		fakeComponent: fakeComponent{
			name:   MasterName,
			status: components.SyncStatusPending,
			spy:    spy,
		},
	}
}

func (c *fakeMasterComponent) GetMasterExitReadOnlyJob() *components.InitJob {
	return &components.InitJob{}
}

func newFakeSchedulerComponent(spy *executionSpy) *fakeSchedulerComponent {
	return &fakeSchedulerComponent{
		fakeComponent: fakeComponent{
			name:   SchedulerName,
			status: components.SyncStatusPending,
			spy:    spy,
		},
	}
}

func (c *fakeSchedulerComponent) GetUpdateOpArchiveJob() *components.InitJob {
	return &components.InitJob{}
}

func (c *fakeSchedulerComponent) PrepareInitOperationArchive(*components.InitJob) {
	return
}

func newFakeQueryTrackerComponent(spy *executionSpy) *fakeQueryTrackerComponent {
	return &fakeQueryTrackerComponent{
		fakeComponent: fakeComponent{
			name:   QueryTrackerName,
			status: components.SyncStatusPending,
			spy:    spy,
		},
	}
}

func (c *fakeQueryTrackerComponent) GetInitQueryTrackerJob() *components.InitJob {
	return &components.InitJob{}
}

func (c *fakeQueryTrackerComponent) PrepareInitQueryTrackerState(*components.InitJob) {
	return
}
