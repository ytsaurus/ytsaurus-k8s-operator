package components_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"

	"go.ytsaurus.tech/yt/go/yt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	mock_yt "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/mock"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ypatch"
)

func TestTabletNodeComponents(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Tablet node components suite")
}

var (
	testEnv    *envtest.Environment
	testConfig *rest.Config
	testClient client.Client
	testScheme *runtime.Scheme
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(os.Stderr), zap.UseDevMode(true)))

	testEnv = &envtest.Environment{
		BinaryAssetsDirectory:    filepath.Join("..", "..", "bin", "envtest-assets"),
		CRDDirectoryPaths:        []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing:    true,
		AttachControlPlaneOutput: true,
	}
	testEnv.ControlPlane.GetAPIServer().Configure().Set("advertise-address", "127.0.0.1")

	var err error
	testConfig, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(testConfig).NotTo(BeNil())

	testScheme = runtime.NewScheme()
	Expect(clientgoscheme.AddToScheme(testScheme)).To(Succeed())
	Expect(ytv1.AddToScheme(testScheme)).To(Succeed())
	testClient, err = client.New(testConfig, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	Expect(testEnv.Stop()).To(Succeed())
})

type FakeComponent struct {
	name     string
	compType consts.ComponentType
	status   ComponentStatus
}

func NewFakeComponent(name string, compType consts.ComponentType) *FakeComponent {
	return &FakeComponent{
		name:     name,
		compType: compType,
		status:   ComponentStatusReady(),
	}
}

func (fc *FakeComponent) Fetch(ctx context.Context) error {
	return nil
}

func (fc *FakeComponent) Exists() bool {
	return true
}

func (fc *FakeComponent) ArePodsReady(ctx context.Context) (ComponentStatus, error) {
	return ComponentStatusReady(), nil
}

func (fc *FakeComponent) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	return fc.status, nil
}

func (fc *FakeComponent) GetStatus() ComponentStatus {
	return fc.status
}

func (fc *FakeComponent) SetStatus(status ComponentStatus) {
	fc.status = status
}

func (fc *FakeComponent) NeedSync() bool {
	return false
}

func (fc *FakeComponent) NeedUpdate() ComponentStatus {
	return ComponentStatusReady()
}

func (fc *FakeComponent) IsUpdating() bool {
	return false
}

func (fc *FakeComponent) GetShortName() string {
	return fc.name
}

func (fc *FakeComponent) GetFullName() string {
	return fc.name
}

func (fc *FakeComponent) GetType() consts.ComponentType {
	return fc.compType
}

func (fc *FakeComponent) GetComponent() ytv1.Component {
	return ytv1.Component{
		Type: fc.compType,
		Name: fc.name,
	}
}

func (fc *FakeComponent) GetLabeller() *labeller.Labeller {
	return nil
}

func (fc *FakeComponent) GetCypressPatch() ypatch.PatchSet {
	return nil
}

func (fc *FakeComponent) GetImageHeaterTarget() *ImageHeaterTarget {
	return nil
}

func (fc *FakeComponent) GetReadyCondition() ComponentStatus {
	return fc.status
}

func (fc *FakeComponent) SetReadyCondition(status ComponentStatus) {}

type FakeYtsaurusClient struct {
	FakeComponent
	client *mock_yt.MockClient
}

// TODO: Add option to inject mock client into normal YtsaurusClient component.
func NewFakeYtsaurusClient(client *mock_yt.MockClient) *FakeYtsaurusClient {
	return &FakeYtsaurusClient{
		FakeComponent: *NewFakeComponent("ytsaurus_client", consts.YtsaurusClientType),
		client:        client,
	}
}

func (fyc *FakeYtsaurusClient) GetYtClient() yt.Client {
	return fyc.client
}

func (fyc *FakeYtsaurusClient) SetStatus(status ComponentStatus) {
	fyc.status = status
}

func (fyc *FakeYtsaurusClient) UpdatePreCheck(ctx context.Context) ComponentStatus {
	return ComponentStatusReady()
}

func (fyc *FakeYtsaurusClient) ShouldSkipCypressOperations() bool {
	return false
}
