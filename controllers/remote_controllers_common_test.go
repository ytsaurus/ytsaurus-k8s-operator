package controllers_test

import (
	"path/filepath"
	"testing"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	remoteYtsaurusHostname = "test-hostname"
	remoteYtsaurusName     = "test-remote-ytsaurus"
)

func startHelperWithController(t *testing.T, namespace string, reconcilerSetupFunc func(mgr ctrl.Manager) error) *testutil.TestHelper {
	h := testutil.NewTestHelper(t, namespace, filepath.Join("..", "config", "crd", "bases"))
	h.Start(reconcilerSetupFunc)
	return h
}

func buildRemoteYtsaurus(h *testutil.TestHelper, remoteYtsaurusName, remoteYtsaurusHostname string) ytv1.RemoteYtsaurus {
	remoteYtsaurus := ytv1.RemoteYtsaurus{
		ObjectMeta: h.GetObjectMeta(remoteYtsaurusName),
		Spec: ytv1.RemoteYtsaurusSpec{
			MasterConnectionSpec: ytv1.MasterConnectionSpec{
				CellTag: 100,
				HostAddresses: []string{
					remoteYtsaurusHostname,
				},
			},
		},
	}
	return remoteYtsaurus
}

func waitRemoteYtsaurusDeployed(h *testutil.TestHelper, remoteYtsaurusName string) {
	testutil.FetchEventually(h, remoteYtsaurusName, &ytv1.RemoteYtsaurus{})
}
