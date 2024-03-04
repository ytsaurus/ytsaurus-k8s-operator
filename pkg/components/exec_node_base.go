package components

import (
	"context"
	"log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	ptr "k8s.io/utils/pointer"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"

	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type baseExecNode struct {
	server server
	cfgen  *ytconfig.NodeGenerator
	spec   *ytv1.ExecNodesSpec
}

func (n *baseExecNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}

func (n *baseExecNode) doBuildBase() error {
	statefulSet := n.server.buildStatefulSet()
	podSpec := &statefulSet.Spec.Template.Spec

	setContainerPrivileged := func(ct *corev1.Container) {
		if ct.SecurityContext == nil {
			ct.SecurityContext = &corev1.SecurityContext{}
		}
		ct.SecurityContext.Privileged = ptr.Bool(n.spec.Privileged)
	}

	if len(podSpec.Containers) != 1 {
		log.Panicf("Number of exec node containers is expected to be 1, actual %v", len(podSpec.Containers))
	}
	setContainerPrivileged(&podSpec.Containers[0])

	for i := range podSpec.InitContainers {
		setContainerPrivileged(&podSpec.InitContainers[i])
	}

	for _, sidecarSpec := range n.spec.Sidecars {
		sidecar := corev1.Container{}
		if err := yaml.Unmarshal([]byte(sidecarSpec), &sidecar); err != nil {
			return err
		}
		podSpec.Containers = append(podSpec.Containers, sidecar)
	}

	return nil
}

func (n *baseExecNode) doSyncBase(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error
	if !dry {
		err = n.doBuildBase()
		if err != nil {
			return WaitingStatus(SyncStatusBlocked, "cannot build exec node spec"), err
		}
		err = n.server.Sync(ctx)
	}
	return WaitingStatus(SyncStatusPending, "components"), err
}
