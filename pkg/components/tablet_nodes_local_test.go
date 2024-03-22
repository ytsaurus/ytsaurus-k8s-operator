package components

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	mock_yt "github.com/ytsaurus/yt-k8s-operator/pkg/mock"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type fakeYtsaurusForTnd struct {
	mockClient *mock_yt.MockClient
}

func (y *fakeYtsaurusForTnd) GetYtClient() yt.Client {
	return y.mockClient
}
func (y *fakeYtsaurusForTnd) GetName() string { return "fakeYtClient" }
func (y *fakeYtsaurusForTnd) Status(_ context.Context) (ComponentStatus, error) {
	return ComponentStatus{}, nil
}

func (y *fakeYtsaurusForTnd) GetTabletCells(context.Context) ([]ytv1.TabletCellBundleInfo, error) {
	return []ytv1.TabletCellBundleInfo{}, nil
}
func (y *fakeYtsaurusForTnd) RemoveTabletCells(context.Context) error {
	return nil
}
func (y *fakeYtsaurusForTnd) RecoverTableCells(context.Context, []ytv1.TabletCellBundleInfo) error {
	return nil
}
func (y *fakeYtsaurusForTnd) AreTabletCellsRemoved(context.Context) (bool, error) {
	return true, nil
}

func TestTabletNodesFlow(t *testing.T) {
	ctrl = gomock.NewController(t)
	ytCli := mock_yt.NewMockClient(ctrl)
	ytCli.EXPECT().NodeExists(context.Background(), ypath.Path("//sys/tablet_cell_bundles/sys"), nil).Return(true, nil)
	count := 0
	ytCli.EXPECT().GetNode(context.Background(), ypath.Path("//sys/tablet_cell_bundles/default/@tablet_cell_count"), &count, nil).Return(nil)

	testComponentFlow(
		t,
		"tnd",
		"tablet-nodes",
		func(_ *ytconfig.Generator, ytsaurus *apiProxy.Ytsaurus) Component {
			cfgen := ytconfig.NewLocalNodeGenerator(ytsaurus.GetResource(), domain)
			return NewTabletNode(
				cfgen,
				ytsaurus,
				&fakeYtsaurusForTnd{ytCli},
				ytsaurus.GetResource().Spec.TabletNodes[0],
				true,
			)
		},
		func(ytsaurus *ytv1.Ytsaurus, image *string) {
			ytsaurus.Spec.Schedulers.Image = image
		},
	)
}
