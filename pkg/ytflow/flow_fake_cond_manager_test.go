package ytflow

import (
	"context"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

type fakeStateManager struct {
	// TODO: msg
	store           map[Condition]bool
	bundles         []ytv1.TabletCellBundleInfo
	monitoringPaths []string
}

func newFakeStateManager() *fakeStateManager {
	return &fakeStateManager{
		store: make(map[Condition]bool),
	}
}

func (cm *fakeStateManager) SetTrue(ctx context.Context, cond Condition, msg string) error {
	return cm.Set(ctx, cond, true, msg)
}
func (cm *fakeStateManager) SetFalse(ctx context.Context, cond Condition, msg string) error {
	return cm.Set(ctx, cond, false, msg)
}
func (cm *fakeStateManager) Set(_ context.Context, cond Condition, val bool, _ string) error {
	cm.store[cond] = val
	return nil
}
func (cm *fakeStateManager) IsTrue(condName Condition) bool {
	return cm.store[condName]
}
func (cm *fakeStateManager) IsFalse(condName Condition) bool {
	return !cm.IsTrue(condName)
}
func (cm *fakeStateManager) Get(condName Condition) bool {
	return cm.store[condName]
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
