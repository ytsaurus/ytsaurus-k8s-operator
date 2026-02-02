package components

import (
	"context"
	"fmt"

	"go.ytsaurus.tech/yt/go/ypath"
	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type Discovery struct {
	localServerComponent
	cfgen          *ytconfig.Generator
	ytsaurusClient internalYtsaurusClient
}

func NewDiscovery(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, yc internalYtsaurusClient) *Discovery {
	l := cfgen.GetComponentLabeller(consts.DiscoveryType, "")
	resource := ytsaurus.GetResource()

	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.Discovery.InstanceSpec,
		"/usr/bin/ytserver-discovery",
		"ytserver-discovery.yson",
		func() ([]byte, error) {
			return cfgen.GetDiscoveryConfig(&resource.Spec.Discovery)
		},
		consts.DiscoveryMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.DiscoveryRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &Discovery{
		localServerComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:                cfgen,
		ytsaurusClient:       yc,
	}
}

func (d *Discovery) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, d.server)
}

func (d *Discovery) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if d.ytsaurus.IsReadyToUpdate() && d.NeedUpdate() {
		return SimpleStatus(SyncStatusNeedUpdate), err
	}

	if d.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(d.ytsaurus, d) {
			switch getComponentUpdateStrategy(d.ytsaurus, consts.DiscoveryType, d.GetShortName()) {
			case ytv1.ComponentUpdateModeTypeOnDelete:
				if status, err := handleOnDeleteUpdatingClusterState(ctx, d.ytsaurus, d, &d.localComponent, d.server, dry); status != nil {
					return *status, err
				}
			default:
				if status, err := handleBulkUpdatingClusterState(ctx, d.ytsaurus, d, &d.localComponent, d.server, dry); status != nil {
					return *status, err
				}
			}

			if d.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation {
				return ComponentStatusReady(), err
			}
		} else {
			return ComponentStatusReadyAfter("Not updating component"), nil
		}
	}

	if d.NeedSync() {
		if !dry {
			err = d.server.Sync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if !d.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	return ComponentStatusReady(), err
}

func (d *Discovery) Status(ctx context.Context) (ComponentStatus, error) {
	return d.doSync(ctx, true)
}

func (d *Discovery) Sync(ctx context.Context) error {
	_, err := d.doSync(ctx, false)
	return err
}

func (d *Discovery) UpdatePreCheck(ctx context.Context) ComponentStatus {
	if d.ytsaurusClient == nil {
		return ComponentStatusBlocked("YtsaurusClient component is not available")
	}

	ytClient := d.ytsaurusClient.GetYtClient()
	if ytClient == nil {
		return ComponentStatusBlocked("YT client is not available")
	}

	// Check that the number of instances in YT matches the expected instanceCount
	expectedCount := int(d.ytsaurus.GetResource().Spec.Discovery.InstanceCount)
	if err := IsInstanceCountEqualYTSpec(ctx, ytClient, consts.DiscoveryType, expectedCount); err != nil {
		return ComponentStatusBlocked(err.Error())
	}

	var discoveryServers []string
	cypressPath := consts.ComponentCypressPath(consts.DiscoveryType)

	err := d.ytsaurusClient.GetYtClient().ListNode(ctx, ypath.Path(cypressPath), &discoveryServers, nil)
	if err != nil {
		return ComponentStatusBlocked(err.Error())
	}

	// Check that all discovery_servers are alive
	for _, server := range discoveryServers {
		orchidPath := ypath.Path(fmt.Sprintf("%s/%s/orchid", cypressPath, server))
		var orchidData map[string]interface{}
		if err := ytClient.GetNode(ctx, orchidPath, &orchidData, nil); err != nil {
			return ComponentStatusBlocked(fmt.Sprintf("Failed to get discovery orchid data for %s: %v", server, err))
		}
	}

	return ComponentStatusReady()
}
