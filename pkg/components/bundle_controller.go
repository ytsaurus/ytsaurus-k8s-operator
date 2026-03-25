package components

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type BundleController struct {
	serverComponent

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
		[]ConfigGenerator{{
			"ytserver-bundle-controller.yson",
			ConfigFormatYson,
			func() ([]byte, error) { return cfgen.GetBundleControllerConfig(resource.Spec.BundleController) },
		}},
		consts.BundleControllerMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.BundleControllerRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &BundleController{
		serverComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:           cfgen,
	}
}

func (bc *BundleController) Fetch(ctx context.Context) error {
	return bc.server.Fetch(ctx)
}

func (bc *BundleController) Exists() bool {
	return bc.server.Exists()
}

func (bc *BundleController) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if bc.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, bc.ytsaurus, bc, &bc.component, bc.server, dry); status != nil {
			return *status, err
		}
	}

	if bc.NeedSync() {
		if !dry {
			err = bc.doServerSync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	return bc.ArePodsReady(ctx)
}

func (bc *BundleController) doServerSync(ctx context.Context) error {
	return bc.server.Sync(ctx)
}
