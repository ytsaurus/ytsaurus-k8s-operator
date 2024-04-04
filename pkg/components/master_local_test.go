package components

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	ptr "k8s.io/utils/pointer"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/testutil"
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
	ctx := context.Background()
	shortName := "ms"
	longName := "master"
	namespace := longName

	h, ytsaurus, cfgen := prepareTest(t, namespace)
	defer h.Stop()

	component := NewMaster(cfgen, ytsaurus, &fakeYtsaurusForMaster{})
	testutil.Eventually(h, shortName+" became ready", func() bool {
		needBuild, err := component.NeedBuild(ctx)
		require.NoError(t, err)
		if !needBuild {
			return true
		}
		require.NoError(t, component.BuildInitial(ctx))
		return false
	})

	cmData := testutil.FetchConfigMapData(h, "yt-"+longName+"-config", "ytserver-"+longName+".yson")
	require.Contains(t, cmData, "ms-0.masters."+namespace+".svc."+domain+":9010")

	// TODO: replace with get
	testutil.FetchEventually(
		h,
		shortName,
		&appsv1.StatefulSet{},
	)

	setImage := func(ytsaurus *ytv1.Ytsaurus, image *string) {
		ytsaurus.Spec.PrimaryMasters.Image = image
	}
	// update
	// TODO: update2 to be sure that first update doesn't end with wrong state
	// after fix jobs deletion
	for i := 1; i <= 1; i++ {
		t.Logf("Update %s #%d", shortName, i)
		newImage := ptr.String(fmt.Sprintf("new-image-%d", i))
		setImage(ytsaurus.GetResource(), newImage)

		component = NewMaster(cfgen, ytsaurus, &fakeYtsaurusForMaster{})
		syncUntilReady(t, h, component)
		sts := &appsv1.StatefulSet{}
		testutil.GetObject(h, shortName, sts)
		require.Equal(t, *newImage, sts.Spec.Template.Spec.Containers[0].Image)
	}
}
