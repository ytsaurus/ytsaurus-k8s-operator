package components

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
)

type ControllerAgent struct {
	serverComponent

	cfgen          *ytconfig.Generator
	master         Component
	ytsaurusClient internalYtsaurusClient
}

func NewControllerAgent(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, master Component, yc internalYtsaurusClient) *ControllerAgent {
	l := cfgen.GetComponentLabeller(consts.ControllerAgentType, "")
	resource := ytsaurus.GetResource()

	srv := newServer(
		l,
		ytsaurus,
		&resource.Spec.ControllerAgents.InstanceSpec,
		"/usr/bin/ytserver-controller-agent",
		"ytserver-controller-agent.yson",
		func() ([]byte, error) { return cfgen.GetControllerAgentConfig(resource.Spec.ControllerAgents) },
		consts.ControllerAgentMonitoringPort,
		WithContainerPorts(corev1.ContainerPort{
			Name:          consts.YTRPCPortName,
			ContainerPort: consts.ControllerAgentRPCPort,
			Protocol:      corev1.ProtocolTCP,
		}),
	)

	return &ControllerAgent{
		serverComponent: newLocalServerComponent(l, ytsaurus, srv),
		cfgen:           cfgen,
		master:          master,
		ytsaurusClient:  yc,
	}
}

func (ca *ControllerAgent) Fetch(ctx context.Context) error {
	return resources.Fetch(ctx, ca.server)
}

func (ca *ControllerAgent) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if ca.ytsaurus.IsReadyToUpdate() && ca.NeedUpdate() {
		return SimpleStatus(SyncStatusNeedUpdate), err
	}

	if ca.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if IsUpdatingComponent(ca.ytsaurus, ca) {
			switch getComponentUpdateStrategy(ca.ytsaurus, consts.ControllerAgentType, ca.GetShortName()) {
			case ytv1.ComponentUpdateModeTypeOnDelete:
				if status, err := handleOnDeleteUpdatingClusterState(ctx, ca.ytsaurus, ca, &ca.component, ca.server, dry); status != nil {
					return *status, err
				}
			default:
				if status, err := handleBulkUpdatingClusterState(ctx, ca.ytsaurus, ca, &ca.component, ca.server, dry); status != nil {
					return *status, err
				}
			}

			if ca.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation {
				return ComponentStatusReady(), err
			}
		} else {
			return ComponentStatusReadyAfter("Not updating component"), nil
		}
	}

	masterStatus, err := ca.master.Status(ctx)
	if err != nil {
		return masterStatus, err
	}
	if !masterStatus.IsRunning() {
		return ComponentStatusBlockedBy(ca.master.GetFullName()), err
	}

	if ca.NeedSync() {
		if !dry {
			err = ca.server.Sync(ctx)
		}
		return ComponentStatusWaitingFor("components"), err
	}

	if !ca.server.arePodsReady(ctx) {
		return ComponentStatusBlockedBy("pods"), err
	}

	return ComponentStatusReady(), err
}

func (ca *ControllerAgent) Status(ctx context.Context) (ComponentStatus, error) {
	return ca.doSync(ctx, true)
}

func (ca *ControllerAgent) Sync(ctx context.Context) error {
	_, err := ca.doSync(ctx, false)
	return err
}

type ControllerAgentsWithMaintenance struct {
	Address     string   `yson:",value"`
	Maintenance bool     `yson:"maintenance,attr"`
	Alerts      []string `yson:"alerts,attr"`
}

func (ca *ControllerAgent) UpdatePreCheck(ctx context.Context) ComponentStatus {
	if ca.ytsaurusClient == nil {
		return ComponentStatusBlocked("YtsaurusClient component is not available")
	}

	ytClient := ca.ytsaurusClient.GetYtClient()
	if ytClient == nil {
		return ComponentStatusBlocked("YT client is not available")
	}

	// Check that the number of instances in YT matches the expected instanceCount
	expectedCount := int(ca.ytsaurus.GetResource().Spec.ControllerAgents.InstanceCount)
	if err := IsInstanceCountEqualYTSpec(ctx, ytClient, consts.ControllerAgentType, expectedCount); err != nil {
		return ComponentStatusBlocked(err.Error())
	}

	controllerAgentsWithMaintenance := make([]ControllerAgentsWithMaintenance, 0)
	cypressPath := consts.ComponentCypressPath(consts.ControllerAgentType)

	err := ca.ytsaurusClient.GetYtClient().ListNode(ctx, ypath.Path(cypressPath), &controllerAgentsWithMaintenance, &yt.ListNodeOptions{
		Attributes: []string{"maintenance", "alerts"}})
	if err != nil {
		return ComponentStatusBlocked(err.Error())
	}

	for _, controllerAgent := range controllerAgentsWithMaintenance {
		var connected bool
		err = ca.ytsaurusClient.GetYtClient().GetNode(
			ctx,
			ypath.Path(fmt.Sprintf("%v/%v/orchid/controller_agent/service/connected", cypressPath, controllerAgent.Address)),
			&connected,
			nil)
		if err != nil {
			return ComponentStatusBlocked(err.Error())
		}

		if !connected {
			msg := fmt.Sprintf("Controller agent is not connected: %v", controllerAgent.Address)
			return ComponentStatusBlocked(msg)
		}

		if controllerAgent.Maintenance {
			msg := fmt.Sprintf("There is a controller agent in maintenance: %v", controllerAgent.Address)
			return ComponentStatusBlocked(msg)
		}
	}

	return ComponentStatusReady()
}
