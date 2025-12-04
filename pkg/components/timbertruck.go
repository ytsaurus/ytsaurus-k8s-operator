package components

import (
	"context"
	"fmt"
	"path"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
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
			return ComponentStatusReady(), err
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
		return ComponentStatusWaitingFor(tt.timbertruckSecret.Name()), err
	}

	ytClientStatus, err := tt.ytsaurusClient.Status(ctx)
	if err != nil {
		return ytClientStatus, err
	}

	if !ytClientStatus.IsRunning() {
		return ComponentStatusBlockedBy(tt.ytsaurusClient.GetFullName()), nil
	}

	if len(tt.tabletNodes) > 0 {
		status, err := tt.handleTabletNodes(ctx, dry)
		if err != nil || status.SyncStatus != SyncStatusReady {
			return status, err
		}
	}

	return ComponentStatusReady(), err
}

func (tt *Timbertruck) handleTabletNodes(ctx context.Context, dry bool) (ComponentStatus, error) {
	for _, tnd := range tt.tabletNodes {
		tndStatus, err := tnd.Status(ctx)
		if err != nil {
			return tndStatus, err
		}
		if !tndStatus.IsRunning() {
			return ComponentStatusBlockedBy(tnd.GetFullName()), nil
		}
	}

	deliveryLoggers := tt.GetDeliveryLoggers()
	if len(deliveryLoggers) == 0 {
		return ComponentStatusReady(), nil
	}

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
		return ComponentStatusWaitingFor("waiting for timbertruck user initialization"), nil
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
		return ComponentStatusWaitingFor("waiting for timbertruck preparation"), nil
	}

	return ComponentStatusReady(), nil
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
			LogsDeliveryPath:  getLogsDirectoryPath(timbertruck),
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

func getTimbertruckInitScript(timbertruckConfig *ytconfig.TimbertruckConfig) (string, error) {
	configText, err := yaml.Marshal(timbertruckConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Timbertruck config: %w", err)
	}
	return fmt.Sprintf("%s%s%s", timbertruckInitScriptPrefix, string(configText), timbertruckInitScriptSuffix), nil
}

func prepareTimbertruckTablesFromConfig(ctx context.Context, ytClient yt.Client, timbertruckConfig *ytconfig.TimbertruckConfig, logsDeliveryPath string) error {
	for _, jsonLog := range timbertruckConfig.JsonLogs {
		for _, ytQueue := range jsonLog.YTQueue {
			queuePath := ytQueue.QueuePath
			exportPath := fmt.Sprintf("%s/export/%s", logsDeliveryPath, jsonLog.Name)
			if err := prepareQueue(ctx, ytClient, queuePath, exportPath); err != nil {
				return fmt.Errorf("failed to prepare YT queue %s with export destination %s: %w", queuePath, exportPath, err)
			}
			producerPath := ytQueue.ProducerPath
			if err := prepareProducer(ctx, ytClient, producerPath); err != nil {
				return fmt.Errorf("failed to prepare YT producer %s: %w", producerPath, err)
			}
			if err := prepareExportDestination(ctx, ytClient, queuePath, exportPath); err != nil {
				return fmt.Errorf("failed to prepare export destination %s for YT queue %s: %w", exportPath, queuePath, err)
			}
		}
	}
	return nil
}

func prepareQueue(ctx context.Context, ytClient yt.Client, queuePath, exportPath string) error {
	_, err := ytClient.CreateNode(
		ctx,
		ypath.Path(queuePath),
		yt.NodeTable,
		&yt.CreateNodeOptions{
			Attributes: map[string]any{
				"dynamic": true,
				"schema":  consts.RawLogsQueueSchema,
				"auto_trim_config": map[string]any{
					"enable":                     true,
					"retained_lifetime_duration": 24 * 60 * 60 * 1000, // 24 hours
				},
				"static_export_config": map[string]any{
					"default": map[string]any{
						"export_directory": exportPath,
						"export_period":    30 * 60 * 1000,           // 30 min
						"export_ttl":       14 * 24 * 60 * 60 * 1000, // 14 days
					},
				},
				"tablet_cell_bundle": "sys",
				"commit_ordering":    "strong",
				"optimize_for":       "scan",
			},
			Recursive:      true,
			IgnoreExisting: true,
		})
	if err != nil {
		return fmt.Errorf("failed to create YT queue %s: %w", queuePath, err)
	}
	err = ytClient.MountTable(ctx, ypath.Path(queuePath), &yt.MountTableOptions{})
	if err != nil {
		return fmt.Errorf("failed to mount YT queue %s: %w", queuePath, err)
	}
	return nil
}

func prepareProducer(ctx context.Context, ytClient yt.Client, producerPath string) error {
	_, err := ytClient.CreateNode(
		ctx,
		ypath.Path(producerPath),
		yt.NodeQueueProducer,
		&yt.CreateNodeOptions{
			Attributes: map[string]any{
				"min_data_versions":  0,
				"min_data_ttl":       0,
				"max_data_ttl":       2592000000,
				"tablet_cell_bundle": "sys",
			},
			Recursive:      true,
			IgnoreExisting: true,
		})
	if err != nil {
		return fmt.Errorf("failed to create YT producer (this functionality is supported on YTsaurus versions 24.1 and higher) %s: %w", producerPath, err)
	}
	err = ytClient.MountTable(ctx, ypath.Path(producerPath), &yt.MountTableOptions{})
	if err != nil {
		return fmt.Errorf("failed to mount YT producer %s: %w", producerPath, err)
	}
	return nil
}

func prepareExportDestination(ctx context.Context, ytClient yt.Client, queuePath, exportPath string) error {
	_, err := ytClient.CreateNode(ctx, ypath.Path(exportPath), yt.NodeMap, &yt.CreateNodeOptions{
		IgnoreExisting: true,
		Recursive:      true,
	})
	if err != nil {
		return fmt.Errorf("failed to create export destination %s: %w", exportPath, err)
	}

	var queueId string
	err = ytClient.GetNode(ctx, ypath.Path(queuePath).Attr("id"), &queueId, &yt.GetNodeOptions{})
	if err != nil {
		return fmt.Errorf("failed to get queue ID for %s: %w", queuePath, err)
	}

	err = ytClient.SetNode(ctx, ypath.Path(exportPath).Attr("queue_static_export_destination"), map[string]any{"originating_queue_id": queueId}, &yt.SetNodeOptions{
		Recursive: true,
	})
	if err != nil {
		return fmt.Errorf("failed to set originating queue ID for export destination %s: %w", exportPath, err)
	}
	return nil
}

func getLogsDirectoryPath(timbertruck *ytv1.TimbertruckSpec) string {
	if timbertruck != nil && timbertruck.DirectoryPath != nil && *timbertruck.DirectoryPath != "" {
		return *timbertruck.DirectoryPath
	}
	return consts.DefaultTimbertruckDirectoryPath
}

func checkAndAddTimbertruckToPodSpec(timbertruck *ytv1.TimbertruckSpec, podSpec *corev1.PodSpec, instanceSpec *ytv1.InstanceSpec, labeler *labeller.Labeller, cfgen *ytconfig.Generator) error {
	if timbertruck == nil || timbertruck.Image == nil || *timbertruck.Image == "" {
		return nil
	}

	if len(instanceSpec.StructuredLoggers) == 0 {
		return nil
	}

	logsDeliveryPath := getLogsDirectoryPath(timbertruck)

	logsLocation := ytv1.FindFirstLocation(instanceSpec.Locations, ytv1.LocationTypeLogs)
	if logsLocation == nil {
		return fmt.Errorf("you are trying to use Timbertruck, but no logs location is defined in the instance spec")
	}
	structuredLoggres := instanceSpec.StructuredLoggers
	logsDirectory := logsLocation.Path
	componentName := consts.GetServiceKebabCase(labeler.ComponentType)
	workDir := fmt.Sprintf("%s/%s", logsDirectory, consts.TimbertruckWorkDirName)
	deliveryProxy := cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)

	timbertruckConfig := ytconfig.NewTimbertruckConfig(
		structuredLoggres,
		workDir,
		componentName,
		logsDirectory,
		deliveryProxy,
		logsDeliveryPath,
	)

	if timbertruckConfig == nil {
		return nil
	}

	timbertruckInitScript, err := getTimbertruckInitScript(timbertruckConfig)
	if err != nil {
		return fmt.Errorf("failed to get timbertruck init script: %w", err)
	}

	podSpec.Containers = append(podSpec.Containers, corev1.Container{
		Name:    consts.TimbertruckContainerName,
		Image:   *timbertruck.Image,
		Command: []string{"/bin/bash", "-c", timbertruckInitScript},
		Env: append([]corev1.EnvVar{
			{
				Name: consts.TokenSecretKey,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: buildUserCredentialsSecretname(consts.TimbertruckUserName),
						},
						Key: consts.TokenSecretKey,
					},
				},
			},
			{
				Name:  "YT_PROXY",
				Value: deliveryProxy,
			},
		}, getDefaultEnv()...),
		VolumeMounts: []corev1.VolumeMount{
			{Name: path.Base(logsDirectory), MountPath: logsDirectory, ReadOnly: false},
		},
		ImagePullPolicy: corev1.PullIfNotPresent,
	})
	return nil
}

func checkAndAddTimbertruckToServerOptions(options *[]Option, timbertruck *ytv1.TimbertruckSpec, structuredLoggers []ytv1.StructuredLoggerSpec) {
	if timbertruck != nil && timbertruck.Image != nil && *timbertruck.Image != "" && len(structuredLoggers) > 0 {
		*options = append(*options, WithSidecarImage(
			consts.TimbertruckContainerName,
			*timbertruck.Image,
		))
	}
}

func (tt *Timbertruck) HasCustomUpdateState() bool {
	return false
}
