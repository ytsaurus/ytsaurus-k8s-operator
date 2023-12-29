package apiproxy

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/yt-k8s-operator/api/v1"
)

type RemoteDataNodes struct {
	remoteDataNodes *ytv1.RemoteDataNodes
	apiProxy        APIProxy
}

func NewRemoteDataNodes(
	remoteDataNodes *ytv1.RemoteDataNodes,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme) *RemoteDataNodes {
	return &RemoteDataNodes{
		remoteDataNodes: remoteDataNodes,
		apiProxy:        NewAPIProxy(remoteDataNodes, client, recorder, scheme),
	}
}

func (n *RemoteDataNodes) APIProxy() APIProxy {
	return n.apiProxy
}

func (n *RemoteDataNodes) GetResource() *ytv1.RemoteDataNodes {
	return n.remoteDataNodes
}

func (n *RemoteDataNodes) SaveDeployStatus(ctx context.Context, status ytv1.RemoteDataNodesDeployStatus) error {
	n.GetResource().Status.DeployStatus = status
	if err := n.apiProxy.UpdateStatus(ctx); err != nil {
		return fmt.Errorf("unable to update remote data nodes deploy status: %w", err)
	}
	return nil
}
