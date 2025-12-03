package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type BundleController struct {
	localServerComponent

	cfgen *ytconfig.Generator
}

func NewBundleController(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
) *BundleController {
	l := cfgen.GetComponentLabeller(consts.BundleControllerType, "")

	resource := ytsaurus.GetResource()
	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.BundleController.InstanceSpec,
		"/usr/bin/ytserver-bundle-controller",
		"ytserver-bundle-controller.yson",
		func() ([]byte, error) { return cfgen.GetBundleControllerConfig(resource.Spec.BundleController) },
		consts.BundleControllerMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.BundleControllerRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &BundleController{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
	}
}

func (bc *BundleController) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, bc.server)
}

func (bc *BundleController) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(bc.ytsaurus.GetClusterState()) && bc.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedUpdate), err
	}

	if bc.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, bc.ytsaurus, bc, &bc.localComponent, bc.server, dry); status != nil {
			return *status, err
		}
	}

	if bc.NeedSync() {
		if !dry {
			err = bc.doServerSync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if !bc.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (bc *BundleController) Status(ctx context.Context) (ComponentStatus, error) {
	return bc.doSync(ctx, true)
}

func (bc *BundleController) Sync(ctx context.Context) error {
	_, err := bc.doSync(ctx, false)
	return err
}

func (bc *BundleController) doServerSync(ctx context.Context) error {
	return bc.server.Sync(ctx)
}

func (bc *BundleController) HasCustomUpdateState() bool {
	return false
}
