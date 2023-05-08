package components

import (
	"context"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mock_yt "github.com/ytsaurus/yt-k8s-operator/pkg/mock"
	"go.ytsaurus.tech/yt/go/yt"
	appsv1 "k8s.io/api/apps/v1"
	"os"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"
)

var ctrl *gomock.Controller

func TestComponents(t *testing.T) {
	RegisterFailHandler(Fail)

	ctrl = gomock.NewController(t)

	RunSpecs(t, "Components fake suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(os.Stderr), zap.UseDevMode(true)))
})

type FakeComponent struct {
	name   string
	status SyncStatus
}

func NewFakeComponent(name string) *FakeComponent {
	return &FakeComponent{name: name, status: SyncStatusReady}
}

func (fc *FakeComponent) Fetch(ctx context.Context) error {
	return nil
}

func (fc *FakeComponent) Sync(ctx context.Context) error {
	return nil
}

func (fc *FakeComponent) Status(ctx context.Context) SyncStatus {
	return fc.status
}

func (fc *FakeComponent) GetName() string {
	return fc.name
}

type FakeServer struct {
	arePodsReady bool
}

func NewFakeServer() *FakeServer {
	return &FakeServer{arePodsReady: true}
}

func (fs *FakeServer) Fetch(ctx context.Context) error {
	return nil
}

func (fs *FakeServer) IsInSync() bool {
	return true
}

func (fs *FakeServer) ArePodsReady(ctx context.Context) bool {
	return fs.arePodsReady
}

func (fs *FakeServer) Sync(ctx context.Context) error {
	return nil
}

func (fs *FakeServer) BuildStatefulSet() *appsv1.StatefulSet {
	return nil
}

type FakeYtsaurusClient struct {
	FakeComponent
	client *mock_yt.MockClient
}

func NewFakeYtsaurusClient(client *mock_yt.MockClient) *FakeYtsaurusClient {
	return &FakeYtsaurusClient{
		FakeComponent: *NewFakeComponent("ytsaurus_client"),
		client:        client,
	}
}

func (fyc *FakeYtsaurusClient) GetYtClient() yt.Client {
	return fyc.client
}

func (fyc *FakeYtsaurusClient) SetStatus(status SyncStatus) {
	fyc.status = status
}
