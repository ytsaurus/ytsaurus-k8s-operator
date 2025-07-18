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
			buildUserCredentialsSecretname(consts.TimbertruckUserName),
			l,
			ytsaurus.APIProxy()),
	}
}

func (tt *Timbertruck) initTimbertruckUser(ctx context.Context, deliveryLoggers []ComponentLoggers) error {
	login := consts.TimbertruckUserName
	token, _ := tt.timbertruckSecret.GetValue(consts.TokenSecretKey)

	ytClient := tt.ytsaurusClient.GetYtClient()

	if ok, err := ytClient.NodeExists(ctx, ypath.Path("//sys/users/"+login), &yt.NodeExistsOptions{}); err != nil {
		return fmt.Errorf("failed to check if timbertruck user exists: %w", err)
	} else if ok {
		return nil
	}

	err := CreateUser(ctx, ytClient, login, token, false)
	if err != nil {
		return fmt.Errorf("failed to create timbertruck user: %w", err)
	}

	logsDeliveryPaths := make(map[string]struct{})
	for _, logger := range deliveryLoggers {
		logsDeliveryPaths[logger.LogsDeliveryPath] = struct{}{}
	}
	for logsDeliveryPath := range logsDeliveryPaths {
		_, err := ytClient.CreateNode(ctx, ypath.Path(logsDeliveryPath), yt.NodeMap, &yt.CreateNodeOptions{
			Recursive:      true,
			IgnoreExisting: true,
		})
		if err != nil {
			return fmt.Errorf("failed to create logs delivery path %s: %w", logsDeliveryPath, err)
		}

		err = ytClient.SetNode(ctx, ypath.Path(fmt.Sprintf("%s/@acl", logsDeliveryPath)), []yt.ACE{
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
			return fmt.Errorf("failed to set ACL for logs delivery path %s: %w", logsDeliveryPath, err)
		}
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

		if deliveryLoggers := tt.GetDeliveryLoggers(); len(deliveryLoggers) > 0 {
			if !tt.ytsaurus.IsStatusConditionTrue(consts.ConditionTimbertruckUserInitialized) {
				if !dry {
					if err := tt.initTimbertruckUser(ctx, deliveryLoggers); err != nil {
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
					tt.ytsaurus.SetStatusCondition(metav1.Condition{
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
	LogsDeliveryPath  string
}

func (tt *Timbertruck) GetDeliveryLoggers() []ComponentLoggers {
	spec := tt.ytsaurus.GetResource().Spec
	allDeliveryLoggers := []ComponentLoggers{}

	extractDeliveryLoggers := func(componentName string, timbertruck *ytv1.TimbertruckSpec, structuredLoggers []ytv1.StructuredLoggerSpec) {
		if timbertruck == nil || timbertruck.Image == nil || len(structuredLoggers) == 0 {
			return
		}
		allDeliveryLoggers = append(allDeliveryLoggers, ComponentLoggers{
			ComponentName:     componentName,
			StructuredLoggers: structuredLoggers,
			LogsDeliveryPath:  getLogsDeliveryPath(timbertruck),
		})
	}
	extractDeliveryLoggers(consts.GetServiceKebabCase(consts.MasterType), spec.PrimaryMasters.Timbertruck, spec.PrimaryMasters.InstanceSpec.StructuredLoggers)
	return allDeliveryLoggers
}

func (tt *Timbertruck) prepareTimbertruckTables(ctx context.Context) error {
	if tt.ytsaurusClient.GetYtClient() == nil {
		return fmt.Errorf("ytClient is not initialized")
	}

	allDeliveryLoggers := tt.GetDeliveryLoggers()

	for _, structuredLoggers := range allDeliveryLoggers {
		timbertruckConfig := ytconfig.NewTimbertruckConfig(
			structuredLoggers.StructuredLoggers,
			"",
			structuredLoggers.ComponentName,
			"",
			tt.cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole),
			structuredLoggers.LogsDeliveryPath,
		)
		if timbertruckConfig == nil {
			continue
		}
		err := prepareTimbertruckTablesFromConfig(ctx, tt.ytsaurusClient.GetYtClient(), timbertruckConfig, structuredLoggers.LogsDeliveryPath)
		if err != nil {
			return fmt.Errorf("failed to prepare timbertruck tables: %w", err)
		}
	}
	return nil
}
