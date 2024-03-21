package components

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	ptr "k8s.io/utils/pointer"
	ctrlrt "sigs.k8s.io/controller-runtime"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	apiProxy "github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/testutil"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

const (
	ytsaurusName = "testsaurus"
)

func TestDiscoveryFlow(t *testing.T) {
	ctx := context.Background()
	namespace := "discovery"

	h := testutil.NewTestHelper(t, namespace, filepath.Join("..", "..", "config", "crd", "bases"))
	h.Start(func(mgr ctrlrt.Manager) error { return nil })
	defer h.Stop()

	ytsaurusResource := testutil.BuildMinimalYtsaurus(namespace, ytsaurusName)
	// Deploy of ytsaurus spec is required, so it could set valid owner references for child resources.
	testutil.DeployObject(h, &ytsaurusResource)

	scheme := runtime.NewScheme()
	utilruntime.Must(ytv1.AddToScheme(scheme))
	fakeRecorder := record.NewFakeRecorder(100)

	ytsaurus := apiProxy.NewYtsaurus(&ytsaurusResource, h.GetK8sClient(), fakeRecorder, scheme)
	domain := "testdomain"
	cfgen := ytconfig.NewGenerator(&ytsaurusResource, domain)

	// initial creation
	discovery := NewDiscovery(cfgen, ytsaurus)
	testutil.Eventually(h, "ds became ready", func() bool {
		st, err := discovery.Status(ctx)
		require.NoError(t, err)
		if st.SyncStatus == SyncStatusReady {
			return true
		}
		require.NoError(t, discovery.Sync(ctx))
		return false
	})

	cmData := testutil.FetchConfigMapData(h, "yt-discovery-config", "ytserver-discovery.yson")
	require.Contains(t, cmData, "ms-0.masters."+namespace+".svc."+domain+":9010")

	testutil.FetchEventually(
		h,
		"ds",
		&appsv1.StatefulSet{},
	)

	// update
	// + update2 to be sure that first update doesn't end with wrong state
	for i := 1; i <= 2; i++ {
		t.Logf("Update ds #%d", i)
		newImage := ptr.String(fmt.Sprintf("new-image-%d", i))
		ytsaurusResource.Spec.Discovery.Image = newImage

		discovery = NewDiscovery(cfgen, ytsaurus)
		testutil.Eventually(h, "ds became ready", func() bool {
			st, err := discovery.Status(ctx)
			require.NoError(t, err)
			if st.SyncStatus == SyncStatusReady {
				return true
			}
			require.NoError(t, discovery.Sync(ctx))
			return false
		})
		sts := &appsv1.StatefulSet{}
		testutil.GetObject(h, "ds", sts)
		require.Equal(t, *newImage, sts.Spec.Template.Spec.Containers[0].Image)
	}
}
