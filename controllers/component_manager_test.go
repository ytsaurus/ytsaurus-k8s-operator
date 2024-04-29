package controllers

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/testutil"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
)

func prepareTest(t *testing.T, namespace string, ytsaurusResource ytv1.Ytsaurus) (*testutil.TestHelper, *apiproxy.Ytsaurus, *ytconfig.Generator) {
	h := testutil.NewTestHelper(t, namespace, filepath.Join("..", "config", "crd", "bases"))
	h.Start(func(mgr ctrl.Manager) error { return nil })

	// Deploy of ytsaurus spec is required, so it could set valid owner references for child resources.
	testutil.DeployObject(h, &ytsaurusResource)

	scheme := runtime.NewScheme()
	utilruntime.Must(ytv1.AddToScheme(scheme))
	fakeRecorder := record.NewFakeRecorder(100)

	ytsaurus := apiproxy.NewYtsaurus(&ytsaurusResource, h.GetK8sClient(), fakeRecorder, scheme)
	cfgen := ytconfig.NewGenerator(ytsaurus.GetResource(), DefaultClusterDomain)
	return h, ytsaurus, cfgen
}

func TestComponentLabels(t *testing.T) {
	name := "ytsaurus-minimum"

	ytsaurusResource := testutil.BuildMinimalYtsaurus(name, name)

	h, ytsaurus, cfgen := prepareTest(t, name, ytsaurusResource)
	defer h.Stop()

	nodeCfgGen := ytconfig.NewLocalNodeGenerator(ytsaurus.GetResource(), DefaultClusterDomain)

	ytNamePrefixes := map[consts.ComponentType]string{
		consts.DiscoveryType:      consts.YTComponentLabelDiscovery,
		consts.MasterType:         consts.YTComponentLabelMaster,
		consts.HttpProxyType:      consts.YTComponentLabelHTTPProxy,
		consts.DataNodeType:       consts.YTComponentLabelDataNode + "-" + ytsaurusResource.Spec.DataNodes[0].Name,
		consts.YtsaurusClientType: consts.YTComponentLabelClient,
	}

	allComponents, _, _ := getComponentInstances(cfgen, nodeCfgGen, ytsaurus)
	for _, c := range allComponents {
		mustNamePrefix, ok := ytNamePrefixes[c.GetType()]
		require.Truef(t, ok, "unknown component %s type: %s", c.GetName(), c.GetType())

		mustLabels := map[string]string{
			"app.kubernetes.io/name":       "Ytsaurus",
			"app.kubernetes.io/instance":   name,
			"app.kubernetes.io/component":  mustNamePrefix,
			"app.kubernetes.io/managed-by": "Ytsaurus-k8s-operator",
			consts.YTComponentLabelName:    fmt.Sprintf("%s-%s", name, mustNamePrefix),
		}

		labels := c.GetMetaLabelMap(false)

		require.Equal(t, mustLabels, labels)
	}
}
