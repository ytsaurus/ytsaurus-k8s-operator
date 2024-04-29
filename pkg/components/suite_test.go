package components

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.ytsaurus.tech/yt/go/yt"
	appsv1 "k8s.io/api/apps/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	mock_yt "github.com/ytsaurus/yt-k8s-operator/pkg/mock"
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
		status:   SimpleStatus(SyncStatusReady),
	}
}

func (fc *FakeComponent) IsUpdatable() bool {
	return false
}

func (fc *FakeComponent) Fetch(ctx context.Context) error {
	return nil
}

func (fc *FakeComponent) Sync(ctx context.Context) error {
	return nil
}

func (fc *FakeComponent) Status(ctx context.Context) (ComponentStatus, error) {
	return fc.status, nil
}

func (fc *FakeComponent) IsUpdating() bool {
	return false
}

func (fc *FakeComponent) GetName() string {
	return fc.name
}

func (fc *FakeComponent) GetType() consts.ComponentType {
	return fc.compType
}

func (fc *FakeComponent) GetMetaLabelMap(isInitJob bool) map[string]string {
	ytComponentLabelValue := fmt.Sprintf("%s-%s", fc.GetName(), fc.GetName())
	if isInitJob {
		ytComponentLabelValue = fmt.Sprintf("%s-%s", ytComponentLabelValue, "init-job")
	}

	return map[string]string{
		"app.kubernetes.io/name":       "Ytsaurus",
		"app.kubernetes.io/instance":   fc.GetName(),
		"app.kubernetes.io/component":  fc.GetName(),
		"app.kubernetes.io/managed-by": "Ytsaurus-k8s-operator",
		consts.YTComponentLabelName:    ytComponentLabelValue,
	}
}

func (fc *FakeComponent) SetReadyCondition(status ComponentStatus) {}

type FakeServer struct {
	podsReady bool
}

func NewFakeServer() *FakeServer {
	return &FakeServer{podsReady: true}
}

func (fs *FakeServer) Fetch(ctx context.Context) error {
	return nil
}

func (fs *FakeServer) needUpdate() bool {
	return false
}

func (fs *FakeServer) podsImageCorrespondsToSpec() bool {
	return true
}

func (fs *FakeServer) configNeedsReload() bool {
	return false
}

func (fs *FakeServer) needBuild() bool {
	return false
}

func (fs *FakeServer) needSync() bool {
	return false
}

func (fs *FakeServer) arePodsRemoved(ctx context.Context) bool {
	return true
}

func (fs *FakeServer) arePodsReady(ctx context.Context) bool {
	return fs.podsReady
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

func (fyc *FakeYtsaurusClient) IsUpdatable() bool {
	return false
}
