package components

import (
	"context"
	"os"
	"testing"

	"go.uber.org/mock/gomock"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"go.ytsaurus.tech/yt/go/yt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	mock_yt "github.com/ytsaurus/ytsaurus-k8s-operator/pkg/mock"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ypatch"
)

var mockCtrl *gomock.Controller

func TestComponents(t *testing.T) {
	RegisterFailHandler(Fail)

	mockCtrl = gomock.NewController(t)

	RunSpecs(t, "Components fake suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(os.Stderr), zap.UseDevMode(true)))
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

func (fc *FakeComponent) Sync(ctx context.Context) error {
	return nil
}

func (fc *FakeComponent) Status(ctx context.Context) (ComponentStatus, error) {
	return fc.status, nil
}

func (fc *FakeComponent) NeedSync() bool {
	return false
}

func (fc *FakeComponent) NeedUpdate() ComponentStatus {
	return ComponentStatus{}
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

func (fc *FakeComponent) GetReadyCondition() ComponentStatus {
	return fc.status
}

func (fc *FakeComponent) SetReadyCondition(status ComponentStatus) {}

type FakeServer struct {
	podsReady bool
}

var _ server = (*FakeServer)(nil)

func NewFakeServer() *FakeServer {
	return &FakeServer{podsReady: true}
}

func (fs *FakeServer) Fetch(ctx context.Context) error {
	return nil
}

func (fs *FakeServer) needUpdate() ComponentStatus {
	return ComponentStatus{}
}

func (fs *FakeServer) podsImageCorrespondsToSpec() bool {
	return true
}

func (fs *FakeServer) Exists() bool {
	return true
}

func (fs *FakeServer) needSync(updating bool) bool {
	return false
}

func (fs *FakeServer) preheatSpec() (images []string, nodeSelector map[string]string, tolerations []corev1.Toleration) {
	return nil, nil, nil
}

func (fs *FakeServer) arePodsRemoved(ctx context.Context) bool {
	return true
}

func (fs *FakeServer) arePodsReady(ctx context.Context) bool {
	return fs.podsReady
}

func (fs *FakeServer) arePodsUpdatedToNewRevision(ctx context.Context) bool {
	return true
}

func (fs *FakeServer) setUpdateStrategy(strategy appsv1.StatefulSetUpdateStrategyType) {
	// No-op for fake server
}

func (fs *FakeServer) Sync(ctx context.Context) error {
	return nil
}

func (fs *FakeServer) buildStatefulSet() *appsv1.StatefulSet {
	return nil
}

func (fs *FakeServer) rebuildStatefulSet() *appsv1.StatefulSet {
	return nil
}

func (fs *FakeServer) removePods(ctx context.Context) error {
	return nil
}

func (fs *FakeServer) addCARootBundle(c *corev1.Container) {
}

func (fs *FakeServer) addTlsSecretMount(c *corev1.Container) {
}

func (fs *FakeServer) addMonitoringPort(port corev1.ServicePort) {
}

func (fs *FakeServer) GetImage() string {
	return ""
}

type FakeYtsaurusClient struct {
	FakeComponent
	client *mock_yt.MockClient
}

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
