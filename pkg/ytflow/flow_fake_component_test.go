package ytflow

import (
	"context"

	"github.com/ytsaurus/yt-k8s-operator/pkg/components"
)

type fakeComponent struct {
	name ComponentName
	ran  bool

	spy *executionSpy
}

func newFakeComponent(name ComponentName, spy *executionSpy) *fakeComponent {
	return &fakeComponent{
		name: name,

		spy: spy,
	}
}

func (c *fakeComponent) GetName() string               { return string(c.name) }
func (c *fakeComponent) Fetch(_ context.Context) error { return nil }
func (c *fakeComponent) Status(_ context.Context) componentStatus {
	if c.ran {
		return componentStatus{
			SyncStatus: components.SyncStatusReady,
			Message:    "test comp sync ok",
		}
	}
	return componentStatus{
		SyncStatus: components.SyncStatusPending,
		Message:    "test comp sync not ok",
	}
}
func (c *fakeComponent) Sync(_ context.Context) error {
	c.ran = true
	c.spy.record(string(c.name))
	return nil
}
