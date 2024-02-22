package components

import (
	"context"
	"log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	ptr "k8s.io/utils/pointer"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type execNode struct {
	localServerComponent
	cfgen      *ytconfig.NodeGenerator
	master     Component
	sidecars   []string
	privileged bool
}

func NewExecNode(
	cfgen *ytconfig.NodeGenerator,
	ytsaurus *apiproxy.Ytsaurus,
	master Component,
	spec ytv1.ExecNodesSpec,
) Component {
	resource := ytsaurus.GetResource()
	l := labeller.Labeller{
		ObjectMeta:     &resource.ObjectMeta,
		APIProxy:       ytsaurus.APIProxy(),
		ComponentLabel: cfgen.FormatComponentStringWithDefault(consts.YTComponentLabelExecNode, spec.Name),
		ComponentName:  cfgen.FormatComponentStringWithDefault("ExecNode", spec.Name),
		MonitoringPort: consts.ExecNodeMonitoringPort,
	}

	srv := newServer(
		&l,
		ytsaurus,
		&spec.InstanceSpec,
		"/usr/bin/ytserver-node",
		"ytserver-exec-node.yson",
		cfgen.GetExecNodesStatefulSetName(spec.Name),
		cfgen.GetExecNodesServiceName(spec.Name),
		func() ([]byte, error) {
			return cfgen.GetExecNodeConfig(spec)
		},
	)

	return &execNode{
		localServerComponent: newLocalServerComponent(&l, ytsaurus, srv),
		cfgen:                cfgen,
		master:               master,
		sidecars:             spec.Sidecars,
		privileged:           spec.Privileged,
	}
}

func (n *execNode) IsUpdatable() bool {
	return true
}

func (n *execNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}

func (n *execNode) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ytv1.IsReadyToUpdateClusterState(n.ytsaurus.GetClusterState()) && n.server.needUpdate() {
		return SimpleStatus(SyncStatusNeedLocalUpdate), err
	}

	if n.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if status, err := handleUpdatingClusterState(ctx, n.ytsaurus, n, &n.localComponent, n.server, dry); status != nil {
			return *status, err
		}
	}

	if !IsRunningStatus(n.master.Status(ctx).SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, n.master.GetName()), err
	}

	if n.NeedSync() {
		if !dry {
			setContainerPrivileged := func(ct *corev1.Container) {
				if ct.SecurityContext == nil {
					ct.SecurityContext = &corev1.SecurityContext{}
				}
				ct.SecurityContext.Privileged = ptr.Bool(n.privileged)
			}

			statefulSet := n.server.buildStatefulSet()
			containers := &statefulSet.Spec.Template.Spec.Containers
			if len(*containers) != 1 {
				log.Panicf("length of exec node containers is expected to be 1, actual %v", len(*containers))
			}
			setContainerPrivileged(&(*containers)[0])

			initContainers := &statefulSet.Spec.Template.Spec.InitContainers
			for i := range *initContainers {
				setContainerPrivileged(&(*initContainers)[i])
			}

			for _, sidecarSpec := range n.sidecars {
				sidecar := corev1.Container{}
				if err := yaml.Unmarshal([]byte(sidecarSpec), &sidecar); err != nil {
					return WaitingStatus(SyncStatusBlocked, "invalid sidecar"), err
				}
				*containers = append(*containers, sidecar)
			}
			err = n.server.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, "components"), err
	}

	if !n.server.arePodsReady(ctx) {
		return WaitingStatus(SyncStatusBlocked, "pods"), err
	}

	return SimpleStatus(SyncStatusReady), err
}

func (n *execNode) Status(ctx context.Context) ComponentStatus {
	status, err := n.doSync(ctx, true)
	if err != nil {
		panic(err)
	}

	return status
}

func (n *execNode) Sync(ctx context.Context) error {
	_, err := n.doSync(ctx, false)
	return err
}
