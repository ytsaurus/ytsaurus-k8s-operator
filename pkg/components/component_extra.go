package components

import (
	"context"
)

func (s ComponentStatus) IsReady() bool {
	return s.SyncStatus == SyncStatusReady
}

// needSync2 is a copy of needSync but without ytsaurus status checking
// (server resources shouldn't know about parent ytsaurus component)
func (s *serverImpl) needSync2() bool {
	needReload, err := s.configHelper.NeedReload()
	if err != nil {
		needReload = false
	}
	return s.configHelper.NeedInit() ||
		needReload ||
		!s.exists() ||
		s.statefulSet.NeedSync(s.instanceSpec.InstanceCount)
}

type Component2 interface {
	Component
	Status2(context.Context) (ComponentStatus, error)
	Sync2(context.Context) error
}

func serverStatus2(ctx context.Context, srv server) (ComponentStatus, error) {
	// exists but images or config is not up-to-date
	if srv.needUpdate() {
		return SimpleStatus(SyncStatusPending), nil
	}

	// configMap not exists
	// OR config is not up-to-date
	// OR server not exists
	// OR not enough pods in sts
	if srv.needSync2() {
		return SimpleStatus(SyncStatusPending), nil
	}

	// TODO: possible leaking abstraction, we should only check if server ready or nor
	if !srv.arePodsReady(ctx) {
		return SimpleStatus(SyncStatusPending), nil
	}

	return SimpleStatus(SyncStatusReady), nil
}

func (d *Discovery) Status2(ctx context.Context) (ComponentStatus, error) {
	return serverStatus2(ctx, d.server)
}
func (d *Discovery) Sync2(ctx context.Context) error {
	// TODO: we are dropping this remove pods thing, but should check if it works ok without it.
	// should we detect `pods are not ready` status and don't do sync
	// will it make things easier or more observable or faster?
	return d.server.Sync(ctx)
}

func (m *Master) Status2(ctx context.Context) (ComponentStatus, error) {
	return serverStatus2(ctx, m.server)

}
func (m *Master) Sync2(ctx context.Context) error { return m.doServerSync(ctx) }

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
