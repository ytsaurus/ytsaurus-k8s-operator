package ytflow

import (
	"context"
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
func (yc *fakeYtsaurusClient) SaveTableCells(context.Context) error {
	yc.spy.record("yc.SaveTableCells")
	return nil
}
func (yc *fakeYtsaurusClient) RemoveTableCells(context.Context) error {
	yc.spy.record("yc.RemoveTableCells")
	return nil
}
func (yc *fakeYtsaurusClient) RecoverTableCells(context.Context) error {
	yc.spy.record("yc.RecoverTableCells")
	return nil
}
func (yc *fakeYtsaurusClient) AreTabletCellsRemoved(context.Context) (bool, error) {
	yc.spy.record("yc.AreTabletCellsRemoved")
	return true, nil
}
func (yc *fakeYtsaurusClient) SaveMasterMonitoringPaths(context.Context) error {
	yc.spy.record("yc.SaveMasterMonitoringPaths")
	return nil
}
func (yc *fakeYtsaurusClient) StartBuildingMasterSnapshots(context.Context) error {
	yc.spy.record("yc.StartBuildingMasterSnapshots")
	return nil
}
func (yc *fakeYtsaurusClient) AreMasterSnapshotsBuilt(context.Context) (bool, error) {
	yc.spy.record("yc.AreMasterSnapshotsBuilt")
	return true, nil
}
