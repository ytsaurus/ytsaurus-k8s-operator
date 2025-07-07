package components

import (
	"context"
	"fmt"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Timbertruck struct {
	localComponent

	cfgen          *ytconfig.Generator
	tabletNodes    []Component
	ytsaurusClient *YtsaurusClient

	ytsaurus *apiproxy.Ytsaurus

	timbertruckSecret *resources.StringSecret
}

func NewTimbertruck(
	cfgen *ytconfig.Generator,
	ytsaurus *apiproxy.Ytsaurus,
	tnds []Component,
	yc *YtsaurusClient,
) *Timbertruck {
	l := cfgen.GetComponentLabeller(consts.TimbertruckType, "")

	return &Timbertruck{
		localComponent: newLocalComponent(l, ytsaurus),
		cfgen:          cfgen,
		tabletNodes:    tnds,
		ytsaurusClient: yc,
		ytsaurus:       ytsaurus,
		timbertruckSecret: resources.NewStringSecret(
			fmt.Sprintf("%s-secret", consts.TimbertruckUserName),
			l,
			ytsaurus.APIProxy()),
	}
}

func (tt *Timbertruck) initTimbertruckUser(ctx context.Context) error {
	login := consts.TimbertruckUserName
	token, _ := tt.timbertruckSecret.GetValue(consts.TokenSecretKey)

	ytClient := tt.ytsaurusClient.GetYtClient()

	if ok, err := ytClient.NodeExists(ctx, ypath.Path("//sys/users/"+login), &yt.NodeExistsOptions{}); err != nil {
		return fmt.Errorf("failed to check if timbertruck user exists: %w", err)
	} else if ok {
		return nil
	}

	_, err := ytClient.CreateNode(ctx, ypath.Path("//sys/admin/logs"), yt.NodeMap, &yt.CreateNodeOptions{
		Recursive:      true,
		IgnoreExisting: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create admin logs directory: %w", err)
	}

	err = CreateUser(ctx, ytClient, login, token, false)
	if err != nil {
		return fmt.Errorf("failed to create timbertruck user: %w", err)
	}

	err = ytClient.SetNode(ctx, ypath.Path("//sys/admin/logs/@acl"), []yt.ACE{
		{
			Action:          "allow",
			Subjects:        []string{login},
			Permissions:     []yt.Permission{"read", "write", "remove", "create"},
			InheritanceMode: "object_and_descendants",
		},
	}, &yt.SetNodeOptions{
		Recursive: true,
	})
	if err != nil {
		return fmt.Errorf("failed to set admin logs ACL: %w", err)
	}
	err = ytClient.SetNode(ctx, ypath.Path("//sys/accounts/sys/@acl/end"), yt.ACE{
		Action:      "allow",
		Subjects:    []string{login},
		Permissions: []yt.Permission{"use"},
	}, &yt.SetNodeOptions{
		Recursive: true,
	})
	if err != nil {
		return fmt.Errorf("failed to set sys account ACL: %w", err)
	}

	return nil
}

func (tt *Timbertruck) handleUpdatingState(ctx context.Context) (ComponentStatus, error) {
	var err error

	if tt.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForTimbertruckPrepared {
		if !tt.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTimbertruckPrepared) {
			err := tt.prepareTimbertruckTables(ctx)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}

			tt.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    consts.ConditionTimbertruckPrepared,
				Status:  metav1.ConditionTrue,
				Reason:  "Update",
				Message: "Timbertruck prepared successfully",
			})
			return SimpleStatus(SyncStatusUpdating), nil
		}
	}

	return SimpleStatus(SyncStatusUpdating), err
}

func (tt *Timbertruck) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if tt.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if tt.ytsaurus.GetResource().Status.UpdateStatus.State == ytv1.UpdateStateImpossibleToStart {
			return SimpleStatus(SyncStatusReady), err
		}
		if dry {
			return SimpleStatus(SyncStatusUpdating), err
		}
		return tt.handleUpdatingState(ctx)
	}

	if tt.timbertruckSecret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			token := ytconfig.RandString(30)
			sec := tt.timbertruckSecret.Build()
			sec.StringData = map[string]string{
				consts.TokenSecretKey: token,
			}
			err = tt.timbertruckSecret.Sync(ctx)
		}
		return WaitingStatus(SyncStatusPending, tt.timbertruckSecret.Name()), err
	}

	ytClientStatus, err := tt.ytsaurusClient.Status(ctx)
	if err != nil {
		return ytClientStatus, err
	}

	if !IsRunningStatus(ytClientStatus.SyncStatus) {
		return WaitingStatus(SyncStatusBlocked, tt.ytsaurusClient.GetFullName()), nil
	}

	if len(tt.tabletNodes) > 0 {
		for _, tnd := range tt.tabletNodes {
			tndStatus, err := tnd.Status(ctx)
			if err != nil {
				return tndStatus, err
			}
			if !IsRunningStatus(tndStatus.SyncStatus) {
				return WaitingStatus(SyncStatusBlocked, tnd.GetFullName()), nil
			}
		}

		if len(tt.getDeliveryLoggers()) > 0 {
			if !tt.ytsaurus.IsStatusConditionTrue(consts.ConditionTimbertruckUserInitialized) {
				if !dry {
					if err := tt.initTimbertruckUser(ctx); err != nil {
						return SimpleStatus(SyncStatusUpdating), err
					}
					tt.ytsaurus.SetStatusCondition(metav1.Condition{
						Type:    consts.ConditionTimbertruckUserInitialized,
						Status:  metav1.ConditionTrue,
						Reason:  "Initialization",
						Message: "Timbertruck user initialized successfully",
					})
				}
				return WaitingStatus(SyncStatusPending, "waiting for timbertruck user initialization"), nil
			}
			if !tt.ytsaurus.IsStatusConditionTrue(consts.ConditionTimbertruckPrepared) {
				if !dry {
					if err := tt.prepareTimbertruckTables(ctx); err != nil {
						return SimpleStatus(SyncStatusUpdating), err
					}
					tt.ytsaurusClient.ytsaurus.SetStatusCondition(metav1.Condition{
						Type:    consts.ConditionTimbertruckPrepared,
						Status:  metav1.ConditionTrue,
						Reason:  "Initialization",
						Message: "Timbertruck prepared successfully",
					})
				}
				return WaitingStatus(SyncStatusPending, "waiting for timbertruck preparation"), nil
			}
		}
	}

	return SimpleStatus(SyncStatusReady), err
}

func (tt *Timbertruck) Fetch(ctx context.Context) error {
	return resources.Fetch(
		ctx,
		tt.timbertruckSecret,
	)
}

func (tt *Timbertruck) Status(ctx context.Context) (ComponentStatus, error) {
	return tt.doSync(ctx, true)
}

func (tt *Timbertruck) Sync(ctx context.Context) error {
	_, err := tt.doSync(ctx, false)
	return err
}

type ComponentLoggers struct {
	ComponentName     string
	StructuredLoggers []ytv1.StructuredLoggerSpec
}

func (tt *Timbertruck) getDeliveryLoggers() []ComponentLoggers {
	spec := tt.ytsaurus.GetResource().Spec
	allDeliveryLoggers := []ComponentLoggers{}

	extractDeliveryLoggers := func(componentName string, structuredLoggers []ytv1.StructuredLoggerSpec) {
		structuredLoggersWithDelivery := []ytv1.StructuredLoggerSpec{}

		for _, logger := range structuredLoggers {
			if logger.CypressDelivery != nil && logger.CypressDelivery.Enabled {
				structuredLoggersWithDelivery = append(structuredLoggersWithDelivery, logger)
			}
		}

		if len(structuredLoggersWithDelivery) > 0 {
			allDeliveryLoggers = append(allDeliveryLoggers, ComponentLoggers{
				ComponentName:     componentName,
				StructuredLoggers: structuredLoggersWithDelivery,
			})
		}
	}

	extractDeliveryLoggers(consts.GetServiceKebabCase(consts.DiscoveryType), spec.Discovery.StructuredLoggers)
	extractDeliveryLoggers(consts.GetServiceKebabCase(consts.MasterType), spec.PrimaryMasters.StructuredLoggers)
	for _, secondaryMaster := range spec.SecondaryMasters {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.MasterType), secondaryMaster.StructuredLoggers)
	}
	if spec.MasterCaches != nil {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.MasterCacheType), spec.MasterCaches.StructuredLoggers)
	}
	for _, httpProxy := range spec.HTTPProxies {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.HttpProxyType), httpProxy.StructuredLoggers)
	}
	for _, rpcProxy := range spec.RPCProxies {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.RpcProxyType), rpcProxy.StructuredLoggers)
	}
	for _, tcpProxy := range spec.TCPProxies {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.TcpProxyType), tcpProxy.StructuredLoggers)
	}
	for _, kafkaProxy := range spec.KafkaProxies {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.KafkaProxyType), kafkaProxy.StructuredLoggers)
	}
	for _, dataNode := range spec.DataNodes {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.DataNodeType), dataNode.StructuredLoggers)
	}
	for _, execNode := range spec.ExecNodes {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.ExecNodeType), execNode.StructuredLoggers)
	}
	if spec.Schedulers != nil {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.SchedulerType), spec.Schedulers.StructuredLoggers)
	}
	if spec.ControllerAgents != nil {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.ControllerAgentType), spec.ControllerAgents.StructuredLoggers)
	}
	for _, tabletNode := range spec.TabletNodes {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.TabletNodeType), tabletNode.StructuredLoggers)
	}
	if spec.QueryTrackers != nil {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.QueryTrackerType), spec.QueryTrackers.StructuredLoggers)
	}
	if spec.YQLAgents != nil {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.YqlAgentType), spec.YQLAgents.StructuredLoggers)
	}
	if spec.QueueAgents != nil {
		extractDeliveryLoggers(consts.GetServiceKebabCase(consts.QueueAgentType), spec.QueueAgents.StructuredLoggers)
	}
	return allDeliveryLoggers
}

func (tt *Timbertruck) prepareTimbertruckTables(ctx context.Context) error {
	if tt.ytsaurusClient.GetYtClient() == nil {
		return fmt.Errorf("ytClient is not initialized")
	}

	allDeliveryLoggers := tt.getDeliveryLoggers()

	for _, structuredLoggers := range allDeliveryLoggers {
		timbertruckConfig := ytconfig.NewTimbertruckConfig(
			structuredLoggers.StructuredLoggers,
			"",
			structuredLoggers.ComponentName,
			"",
			tt.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole),
		)
		if timbertruckConfig == nil {
			continue
		}
		err := prepareTimbertruckTablesFromConfig(ctx, tt.ytsaurusClient.GetYtClient(), timbertruckConfig)
		if err != nil {
			return fmt.Errorf("failed to prepare timbertruck tables: %w", err)
		}
	}
	return nil
}
