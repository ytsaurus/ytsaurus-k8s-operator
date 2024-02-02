package components

import (
	"context"
	"log"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	ptr "k8s.io/utils/pointer"

	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
)

type baseExecNode struct {
	server     server
	cfgen      *ytconfig.NodeGenerator
	sidecars   []string
	privileged bool
}

func (n *baseExecNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server)
}

func (n *baseExecNode) doSyncBase(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error
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
			if err = yaml.Unmarshal([]byte(sidecarSpec), &sidecar); err != nil {
				return WaitingStatus(SyncStatusBlocked, "invalid sidecar"), err
			}
			*containers = append(*containers, sidecar)
		}
		err = n.server.Sync(ctx)
	}
	return WaitingStatus(SyncStatusPending, "components"), err
}
