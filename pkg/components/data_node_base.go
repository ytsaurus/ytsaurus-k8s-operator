package components

import (
	"context"
	"log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type baseDataNode struct {
	server server
	cfgen  *ytconfig.NodeGenerator
	spec   *ytv1.DataNodesSpec

	sidecarConfig *ConfigHelper
}

func (n *baseDataNode) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, n.server, n.sidecarConfig)
}

func (n *baseDataNode) doBuildBase() error {
	statefulSet := n.server.buildStatefulSet()
	podSpec := &statefulSet.Spec.Template.Spec

	if len(podSpec.Containers) != 1 {
		log.Panicf("Number of data node containers is expected to be 1, actual %v", len(podSpec.Containers))
	}

	if n.sidecarConfig != nil {
		podSpec.Volumes = append(podSpec.Volumes, createConfigVolume(consts.ContainerdConfigVolumeName,
			n.sidecarConfig.labeller.GetSidecarConfigMapName(consts.JobsContainerName), nil))

		n.sidecarConfig.Build()
	}

	return nil
}

func (n *baseDataNode) doSyncBase(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error
	if !dry {
		err = n.doBuildBase()
		if err != nil {
			return WaitingStatus(SyncStatusBlocked, "cannot build data node spec"), err
		}
		err = resources.Sync(ctx, n.server, n.sidecarConfig)
	}
	return WaitingStatus(SyncStatusPending, "components"), err
}
