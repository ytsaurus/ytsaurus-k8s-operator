package components

import (
	"context"
)

func (s ComponentStatus) IsReady() bool {
	return s.SyncStatus == SyncStatusReady
}

type Component2 interface {
	Component
	Status2(context.Context) (ComponentStatus, error)
	Sync2(context.Context) error
}

func (d *Discovery) Status2(_ context.Context) (ComponentStatus, error) {
	return SimpleStatus(SyncStatusReady), nil
}
func (d *Discovery) Sync2(context.Context) error { return nil }

func (m *Master) Status2(_ context.Context) (ComponentStatus, error) {
	return SimpleStatus(SyncStatusReady), nil
}
func (m *Master) Sync2(context.Context) error { return nil }

func (hp *HTTPProxy) Status2(_ context.Context) (ComponentStatus, error) {
	return SimpleStatus(SyncStatusReady), nil
}
func (hp *HTTPProxy) Sync2(context.Context) error { return nil }

func (yc *ytsaurusClient) Status2(_ context.Context) (ComponentStatus, error) {
	return SimpleStatus(SyncStatusReady), nil
}
func (yc *ytsaurusClient) Sync2(context.Context) error { return nil }

func (n *DataNode) Status2(_ context.Context) (ComponentStatus, error) {
	return SimpleStatus(SyncStatusReady), nil
}
func (n *DataNode) Sync2(context.Context) error { return nil }
