package components

import (
	"context"
	"os"
	"time"

	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yt/ythttp"

	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

func (d *discovery) Status(ctx context.Context) ComponentStatus {
	// exists but images or config is not up-to-date
	if d.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	// configMap not exists
	// OR config is not up-to-date
	// OR server not exists
	// OR not enough pods in sts
	if d.server.needSync() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	// FIXME: possible leaking abstraction, we should only check if server ready or nor
	if !d.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusUpdating, "pods")
	}

	return SimpleStatus(SyncStatusReady)
}
func (d *discovery) Sync(ctx context.Context) error {
	// TODO: we are dropping this remove pods thing, but should check if it works ok without it.
	// should we detect `pods are not ready` status and don't do sync
	// will it make things easier or more observable or faster?
	return d.server.Sync(ctx)
}

func (m *master) Status(ctx context.Context) ComponentStatus {
	if m.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate)
	}
	if m.server.needSync() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}
	if !m.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusUpdating, "pods")
	}
	// TODO: refactor access to inner initjob
	if !resources.Exists(m.initJob.initJob) {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}
	if !m.initJob.initJob.Completed() {
		return WaitingStatus(SyncStatusUpdating, "init-job")
	}
	return SimpleStatus(SyncStatusReady)
}
func (m *master) Sync(ctx context.Context) error {
	err := m.doServerSync(ctx)
	if err != nil {
		return err
	}
	m.initJob.SetInitScript(m.createInitScript())
	_, err = m.initJob.Sync(ctx, false)
	return err
}
func (m *master) DoExitReadOnly(ctx context.Context) error {
	// FIXME: test how it goes if ran several times
	err := m.exitReadOnlyJob.Fetch(ctx)
	if err != nil {
		return err
	}
	m.exitReadOnlyJob.SetInitScript(m.createExitReadOnlyScript())
	_, err = m.exitReadOnlyJob.Sync(ctx, false)
	return err
}

func (hp *httpProxy) Status(ctx context.Context) ComponentStatus {
	if hp.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	// FIXME: stopped checking master running status:
	// we either run hps after master in flow or hps don't need working master to be
	// deployed correctly (though they might not be considered healthy,
	// I suppose such check could another step or some other component dependency)

	if hp.server.needSync() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	if !resources.Exists(hp.balancingService) {
		return WaitingStatus(SyncStatusNeedLocalUpdate, hp.balancingService.Name())
	}

	if !hp.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusUpdating, "pods")
	}

	return SimpleStatus(SyncStatusReady)
}
func (hp *httpProxy) Sync(ctx context.Context) error {
	statefulSet := hp.server.buildStatefulSet()
	if hp.httpsSecret != nil {
		hp.httpsSecret.AddVolume(&statefulSet.Spec.Template.Spec)
		hp.httpsSecret.AddVolumeMount(&statefulSet.Spec.Template.Spec.Containers[0])
	}
	err := hp.server.Sync(ctx)
	if err != nil {
		return err
	}

	if !resources.Exists(hp.balancingService) {
		s := hp.balancingService.Build()
		s.Spec.Type = hp.serviceType
		err = hp.balancingService.Sync(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *dataNode) Status(ctx context.Context) ComponentStatus {
	if n.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate)
	}
	// TODO: we don't checking master running status anymore
	if n.server.needSync() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}
	if !n.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusUpdating, "pods")
	}
	return SimpleStatus(SyncStatusReady)
}
func (n *dataNode) Sync(ctx context.Context) error {
	return n.server.Sync(ctx)
}

func (yc *ytsaurusClient) Status(_ context.Context) ComponentStatus {
	// FIXME: stopped checking hp running status:
	// though but maybe we still should?
	// It depends on if hps could be deployed before master or not
	// maybe we need some action-step for healthcheck before ytsaurus client

	// FIXME: Why this not in New?
	var err error
	if yc.ytClient == nil {
		token, _ := yc.secret.GetValue(consts.TokenSecretKey)
		timeout := time.Second * 10
		proxy, ok := os.LookupEnv("YTOP_PROXY")
		disableProxyDiscovery := true
		if !ok {
			proxy = yc.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)
			disableProxyDiscovery = false
		}
		yc.ytClient, err = ythttp.NewClient(&yt.Config{
			Proxy:                 proxy,
			Token:                 token,
			LightRequestTimeout:   &timeout,
			DisableProxyDiscovery: disableProxyDiscovery,
		})

		// We should return error I think.
		if err != nil {
			return ComponentStatus{SyncStatusUpdating, "ytClient init " + err.Error()}
		}
	}

	if yc.secret.NeedSync(consts.TokenSecretKey, "") {
		return WaitingStatus(SyncStatusNeedLocalUpdate, yc.secret.Name())
	}

	// TODO: understand how check for initUserJob runs
	// we need to differentiate when it needed to be run and when it already ran

	return SimpleStatus(SyncStatusReady)
}
func (yc *ytsaurusClient) Sync(ctx context.Context) error {
	var err error
	if yc.secret.NeedSync(consts.TokenSecretKey, "") {
		s := yc.secret.Build()
		s.StringData = map[string]string{
			consts.TokenSecretKey: ytconfig.RandString(30),
		}
		err = yc.secret.Sync(ctx)
		if err != nil {
			return err
		}
	}

	// todo run init job if necessary

	return nil
}

func (s *scheduler) Status(ctx context.Context) ComponentStatus {
	if s.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	// FIXME: i've removed wait for master & exec nodes here

	if s.secret.NeedSync(consts.TokenSecretKey, "") {
		return WaitingStatus(SyncStatusNeedLocalUpdate, s.secret.Name())
	}

	if s.server.needSync() {
		return SimpleStatus(SyncStatusNeedLocalUpdate)
	}

	if !s.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusUpdating, "pods")
	}

	return SimpleStatus(SyncStatusReady)
}
func (s *scheduler) Sync(ctx context.Context) error {
	if err := s.secret.Sync(ctx); err != nil {
		return err
	}
	if err := s.server.Sync(ctx); err != nil {
		return err
	}
	return nil
}
