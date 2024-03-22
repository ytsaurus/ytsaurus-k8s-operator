package components

import (
	"context"
	"testing"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type fakeYtsaurusForMaster struct {
}

func (y *fakeYtsaurusForMaster) HandlePossibilityCheck(context.Context) (bool, string, error) {
	return true, "", nil
}
func (y *fakeYtsaurusForMaster) EnableSafeMode(context.Context) error {
	return nil
}
func (y *fakeYtsaurusForMaster) DisableSafeMode(context.Context) error {
	return nil
}
func (y *fakeYtsaurusForMaster) GetMasterMonitoringPaths(context.Context) ([]string, error) {
	return []string{"path1", "path2"}, nil
}
func (y *fakeYtsaurusForMaster) StartBuildMasterSnapshots(context.Context, []string) error {
	return nil
}
func (y *fakeYtsaurusForMaster) AreMasterSnapshotsBuilt(context.Context, []string) (bool, error) {
	return true, nil
}

func TestMasterFlow(t *testing.T) {
	testComponentFlow(
		t,
		"ms",
		"master",
		func(cfgen *ytconfig.Generator, ytsaurus *apiProxy.Ytsaurus) Component {
			return NewMaster(cfgen, ytsaurus, &fakeYtsaurusForMaster{})
		},
		func(ytsaurus *ytv1.Ytsaurus, image *string) {
			ytsaurus.Spec.PrimaryMasters.Image = image
		},
	)
}
