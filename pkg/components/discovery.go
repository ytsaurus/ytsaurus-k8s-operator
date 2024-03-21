package components

import (
	"context"
	"fmt"

	"go.ytsaurus.tech/library/go/ptr"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type Discovery struct {
	localServerComponent
	cfgen *ytconfig.Generator
}

func NewDiscovery(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus) *Discovery {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: consts.YTComponentLabelDiscovery,
		ComponentName:  string(consts.DiscoveryType),
	}

	if resource.Spec.Discovery.InstanceSpec.MonitoringPort == nil {
		resource.Spec.Discovery.InstanceSpec.MonitoringPort = ptr.Int32(consts.DiscoveryMonitoringPort)
	}

	srv := newServer(
		&l,
		ytsaurus,
		&resource.Spec.Discovery.InstanceSpec,
		"/usr/bin/ytserver-discovery",
		"ytserver-discovery.yson",
		cfgen.GetDiscoveryStatefulSetName(),
		cfgen.GetDiscoveryServiceName(),
		func() ([]byte, error) {
			return cfgen.GetDiscoveryConfig(&resource.Spec.Discovery)
		},
	)

	return &Discovery{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
	}
}

func (d *Discovery) IsUpdatable() bool {
	return true
}

func (d *Discovery) GetType() consts.ComponentType { return consts.DiscoveryType }

func (d *Discovery) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, d.server)
}

func (d *Discovery) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(d.ytsaurus.GetClusterState()) && d.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if d.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, d.ytsaurus, d, &d.localComponent, d.server, dry); status != nil {
			return *status, err
		}
	}

	if d.NeedSync() {
		if !dry {
			err = d.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !d.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (d *Discovery) Status(ctx context.Context) (ComponentStatus, error) {
	if err := d.Fetch(ctx); err != nil {
		return ComponentStatus{}, fmt.Errorf("failed to fetch component %s: %w", d.GetName(), err)
	}

	if d.condManager.Is(not(buildFinished(d.GetName()))) {
		return NeedSyncStatus("initial build not yet have finished"), nil
	}

	needUpdate, err := d.server.hasDiff(ctx)
	if err != nil {
		return ComponentStatus{}, err
	}

	if needUpdate || d.condManager.Is(updateRequired(d.GetName())) {
		return NeedSyncStatus("component needs update"), nil
	}

	return ReadyStatus(), nil
}

func (d *Discovery) StatusOld(ctx context.Context) ComponentStatus {
	st, err := d.Status(ctx)
	if err != nil {
		panic(err)
	}
	return st
}

func (d *Discovery) Sync(ctx context.Context) error {
	srv := d.server.(*serverImpl)

	// Initial component creation.
	builtStartedCond := buildStarted(d.GetName())
	if d.condManager.Is(not(builtStartedCond)) {
		return d.runUntilNoErr(ctx, d.server.Sync, builtStartedCond)
	}

	builtCond := buildFinished(d.GetName())
	if d.condManager.Is(not(builtCond)) {
		return d.runUntilOk(ctx, func(ctx context.Context) (bool, error) {
			diff, err := d.server.hasDiff(ctx)
			return !diff, err
		}, builtCond)
	}

	// Update in case of a diff.
	needUpdate, err := srv.hasDiff(ctx)
	if err != nil {
		return err
	}
	updateRequiredCond := updateRequired(d.GetName())
	if needUpdate {
		if err = d.condManager.SetCond(ctx, updateRequiredCond); err != nil {
			return err
		}
	}
	if d.condManager.Is(updateRequiredCond) {
		return d.runUntilOkWithCleanup(ctx, d.handleUpdate, d.handlePostUpdate, not(updateRequiredCond))
	}

	return nil
}

func (d *Discovery) handleUpdate(ctx context.Context) (bool, error) {
	podsWereRemoved := podsRemoved(d.GetName())
	if d.condManager.Is(not(podsWereRemoved)) {
		return false, d.runUntilNoErr(ctx, d.server.removePods, podsWereRemoved)
	}
	return true, nil
}

func (d *Discovery) handlePostUpdate(ctx context.Context) error {
	for _, cond := range d.getConditionsSetByUpdate() {
		if err := d.condManager.SetCond(ctx, not(cond)); err != nil {
			return err
		}
	}
	return nil
}

func (d *Discovery) getConditionsSetByUpdate() []Condition {
	var result []Condition
	conds := []Condition{
		podsRemoved(d.GetName()),
	}
	for _, cond := range conds {
		if d.condManager.IsSatisfied(cond) {
			result = append(result, cond)
		}
	}
	return result
}
