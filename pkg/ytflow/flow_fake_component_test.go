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
func (c *fakeComponent) Sync(_ context.Context) error {
	c.status = components.SyncStatusReady
	c.spy.record(string(c.name))
	return nil
}
