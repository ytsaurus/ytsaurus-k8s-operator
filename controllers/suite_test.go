package controllers_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"
)

const (
	testYtsaurusImage = "test-ytsaurus-image"

	remoteYtsaurusHostname = "test-hostname"
	remoteYtsaurusName     = "test-remote-ytsaurus"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
})

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
