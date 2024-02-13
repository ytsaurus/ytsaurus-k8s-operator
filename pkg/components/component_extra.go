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

func (d *Discovery) Status2(ctx context.Context) (ComponentStatus, error) {
	// exists but images or config is not up-to-date
	if d.server.needUpdate() {
		return SimpleStatus(SyncStatusPending), nil
	}

	// configMap not exists
	// OR config is not up-to-date
	// OR server not exists
	// OR not enough pods in sts
	if d.server.needSync2() {
		return SimpleStatus(SyncStatusPending), nil
	}

	// FIXME: possible leaking abstraction, we should only check if server ready or nor
	if !d.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusPending, "pods"), nil
	}

	return SimpleStatus(SyncStatusReady), nil
}
func (d *Discovery) Sync2(ctx context.Context) error {
	// TODO: we are dropping this remove pods thing, but should check if it works ok without it.
	// should we detect `pods are not ready` status and don't do sync
	// will it make things easier or more observable or faster?
	return d.server.Sync(ctx)
}

func (m *Master) Status2(ctx context.Context) (ComponentStatus, error) {
	if m.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedFullUpdate), nil
	}
	if m.server.needSync2() {
		return SimpleStatus(SyncStatusPending), nil
	}
	if !m.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusPending, "pods"), nil
	}
	return SimpleStatus(SyncStatusReady), nil
}
func (m *Master) Sync2(ctx context.Context) error { return m.doServerSync(ctx) }

func (hp *HTTPProxy) Status2(ctx context.Context) (ComponentStatus, error) {
	if hp.server.needUpdate() {
		return SimpleStatus(SyncStatusPending), nil
	}

	// FIXME: stopped checking master running status:
	// we either run hps after master in flow or hps don't need working master to be
	// deployed correctly (though they might not be considered healthy,
	// I suppose such check could another step or some other component dependency)

	if hp.server.needSync2() {
		return SimpleStatus(SyncStatusPending), nil
	}

	if !resources.Exists(hp.balancingService) {
		return WaitingStatus(SyncStatusPending, hp.balancingService.Name()), nil
	}

	if !hp.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusPending, "pods"), nil
	}

	return SimpleStatus(SyncStatusReady), nil
}
func (hp *HTTPProxy) Sync2(ctx context.Context) error {
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

func (yc *ytsaurusClient) Status2(_ context.Context) (ComponentStatus, error) {
	// FIXME: stopped checking hp running status:
	// though but maybe we still should?
	// It depends on if hps could be deployed before master or not
	// maybe we need some action-step for healthcheck before ytsaurus client

	if yc.secret.NeedSync(consts.TokenSecretKey, "") {
		return WaitingStatus(SyncStatusPending, yc.secret.Name()), nil
	}

	// TODO: understand how check for initUserJob runs
	// we need to differentiate when it needed to be run and when it already ran

	return SimpleStatus(SyncStatusReady), nil
}
func (yc *ytsaurusClient) Sync2(ctx context.Context) error {
	var err error

	if yc.secret.NeedSync(consts.TokenSecretKey, "") {
		s := yc.secret.Build()
		s.StringData = map[string]string{
			consts.TokenSecretKey: ytconfig.RandString(30),
		}
		err = yc.secret.Sync(ctx)
	}

	// todo run init job if necessary

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

		if err != nil {
			return err
		}
	}
	return nil
}

func (n *DataNode) Status2(_ context.Context) (ComponentStatus, error) {
	return SimpleStatus(SyncStatusReady), nil
}
func (n *DataNode) Sync2(context.Context) error { return nil }
