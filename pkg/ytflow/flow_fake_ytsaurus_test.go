package ytflow

import (
	"context"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

type fakeYtsaurusClient struct {
	fakeComponent
	spy *executionSpy
}

func newFakeYtsaurusClient(spy *executionSpy) *fakeYtsaurusClient {
	return &fakeYtsaurusClient{
		fakeComponent: fakeComponent{
			name: YtsaurusClientName,
			spy:  spy,
		},
		spy: spy,
	}
}

func (yc *fakeYtsaurusClient) HandlePossibilityCheck(context.Context) (bool, string, error) {
	yc.spy.record("yc.HandlePossibilityCheck")
	return true, "", nil
}
func (yc *fakeYtsaurusClient) EnableSafeMode(context.Context) error {
	yc.spy.record("yc.EnableSafeMode")
	return nil
}
func (yc *fakeYtsaurusClient) DisableSafeMode(context.Context) error {
	yc.spy.record("yc.DisableSafeMode")
	return nil
}

func (yc *fakeYtsaurusClient) GetTabletCells(context.Context) ([]ytv1.TabletCellBundleInfo, error) {
	yc.spy.record("yc.GetTabletCells")
	return []ytv1.TabletCellBundleInfo{}, nil
}
func (yc *fakeYtsaurusClient) RemoveTabletCells(context.Context) error {
	yc.spy.record("yc.RemoveTabletCells")
	return nil
}
func (yc *fakeYtsaurusClient) RecoverTableCells(context.Context, []ytv1.TabletCellBundleInfo) error {
	yc.spy.record("yc.RecoverTableCells")
	return nil
}
func (yc *fakeYtsaurusClient) AreTabletCellsRemoved(context.Context) (bool, error) {
	yc.spy.record("yc.AreTabletCellsRemoved")
	return true, nil
}

func (yc *fakeYtsaurusClient) GetMasterMonitoringPaths(context.Context) ([]string, error) {
	yc.spy.record("yc.GetMasterMonitoringPaths")
	return []string{}, nil
}
func (yc *fakeYtsaurusClient) StartBuildMasterSnapshots(context.Context, []string) error {
	yc.spy.record("yc.StartBuildingMasterSnapshots")
	return nil
}
func (yc *fakeYtsaurusClient) AreMasterSnapshotsBuilt(context.Context, []string) (bool, error) {
	yc.spy.record("yc.AreMasterSnapshotsBuilt")
	return true, nil
}
