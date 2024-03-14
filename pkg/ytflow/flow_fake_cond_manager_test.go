package ytflow

import (
	"context"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

type fakeStateManager struct {
	// TODO: msg
	conds           map[ConditionName]bool
	clusterState    ytv1.ClusterState
	bundles         []ytv1.TabletCellBundleInfo
	monitoringPaths []string
}

func newFakeStateManager() *fakeStateManager {
	return &fakeStateManager{
		conds: make(map[ConditionName]bool),
	}
}

func (cm *fakeStateManager) SetTrue(ctx context.Context, cond ConditionName, msg string) error {
	return cm.Set(ctx, cond, true, msg)
}
func (cm *fakeStateManager) SetFalse(ctx context.Context, cond ConditionName, msg string) error {
	return cm.Set(ctx, cond, false, msg)
}
func (cm *fakeStateManager) Set(_ context.Context, cond ConditionName, val bool, _ string) error {
	cm.conds[cond] = val
	return nil
}
func (cm *fakeStateManager) IsTrue(condName ConditionName) bool {
	return cm.conds[condName]
}
func (cm *fakeStateManager) IsFalse(condName ConditionName) bool {
	return !cm.IsTrue(condName)
}
func (cm *fakeStateManager) Get(condName ConditionName) bool {
	return cm.conds[condName]
}

func (cm *fakeStateManager) SetTabletCellBundles(_ context.Context, bundles []ytv1.TabletCellBundleInfo) error {
	cm.bundles = bundles
	return nil
}
func (cm *fakeStateManager) SetMasterMonitoringPaths(_ context.Context, paths []string) error {
	cm.monitoringPaths = paths
	return nil
}
func (cm *fakeStateManager) GetTabletCellBundles() []ytv1.TabletCellBundleInfo {
	return cm.bundles
}
func (cm *fakeStateManager) GetMasterMonitoringPaths() []string {
	return cm.monitoringPaths
}

func (cm *fakeStateManager) SetClusterState(_ context.Context, clusterState ytv1.ClusterState) error {
	cm.clusterState = clusterState
	return nil
}
func (cm *fakeStateManager) GetClusterState() ytv1.ClusterState {
	return cm.clusterState
}
