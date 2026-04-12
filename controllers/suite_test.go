package controllers_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/testutil"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	testYtsaurusImage = "test-ytsaurus-image"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	testutil.SetupResultsHistory()
	RunSpecs(t, "Controllers Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
})
