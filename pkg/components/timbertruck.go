package components

import (
	"context"
	"fmt"

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
)

type Timbertruck struct {
	virtualComponent

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
		virtualComponent: virtualComponent{
			component: newComponent(l, ytsaurus),
		},
		cfgen:          cfgen,
		tabletNodes:    tnds,
		ytsaurusClient: yc,
		ytsaurus:       ytsaurus,
		timbertruckSecret: resources.NewStringSecret(
			buildUserCredentialsSecretname(consts.TimbertruckUserName),
			l,
			ytsaurus),
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

func (tt *Timbertruck) handleUpdatingState(ctx context.Context, dry bool) (ComponentStatus, error) {
	if tt.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForTimbertruckPrepared &&
		!tt.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionTimbertruckPrepared) {
		if dry {
			return SimpleStatus(SyncStatusUpdating), nil
		}
		if err := tt.prepareTimbertruckTables(ctx); err != nil {
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

	return ComponentStatusReady(), nil
}

func (tt *Timbertruck) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if tt.ytsaurus.GetClusterState() == ytv1.ClusterStateUpdating {
		if tt.ytsaurus.GetUpdateState() == ytv1.UpdateStateImpossibleToStart {
			return ComponentStatusReady(), err
		}
		return tt.handleUpdatingState(ctx, dry)
	}

	if tt.timbertruckSecret.NeedSync(consts.TokenSecretKey, "") {
		if !dry {
			token := tt.cfgen.GenerateToken()
			sec := tt.timbertruckSecret.Build()
			sec.StringData = map[string]string{
				consts.TokenSecretKey: token,
			}
			err = tt.timbertruckSecret.Sync(ctx)
		}
		return ComponentStatusWaitingFor(tt.timbertruckSecret.Name()), err
	}

	if ytClientStatus := tt.ytsaurusClient.GetStatus(); !ytClientStatus.IsRunning() {
		return ytClientStatus.Blocker(), nil
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
		if tndStatus := tnd.GetStatus(); !tndStatus.IsRunning() {
			return tndStatus.Blocker(), nil
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
	return tt.timbertruckSecret.Fetch(ctx)
}

func (tt *Timbertruck) Exists() bool {
	return tt.timbertruckSecret.Exists()
}

func (tt *Timbertruck) NeedSync() bool {
	return false
}

func (tt *Timbertruck) NeedUpdate() ComponentStatus {
	return ComponentStatusReady()
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

// newTimbertruckConfigBuilder returns a ConfigMapBuilder for the timbertruck
// sidecar config, or nil if timbertruck is not enabled for this component.
func newTimbertruckConfigBuilder(
	proxy apiproxy.APIProxy,
	configOverrides *corev1.LocalObjectReference,
	timbertruck *ytv1.TimbertruckSpec,
	instanceSpec *ytv1.InstanceSpec,
	labeler *labeller.Labeller,
	cfgen *ytconfig.Generator,
) (*ConfigMapBuilder, error) {
	if timbertruck == nil || timbertruck.Image == nil || *timbertruck.Image == "" {
		return nil, nil
	}
	if len(instanceSpec.StructuredLoggers) == 0 {
		return nil, nil
	}

	logsLocation := ytv1.FindFirstLocation(instanceSpec.Locations, ytv1.LocationTypeLogs)
	if logsLocation == nil {
		return nil, fmt.Errorf("you are trying to use Timbertruck, but no logs location is defined in the instance spec")
	}
	logsDirectory := logsLocation.Path
	componentName := consts.GetServiceKebabCase(labeler.ComponentType)
	workDir := fmt.Sprintf("%s/%s", logsDirectory, consts.TimbertruckWorkDirName)
	deliveryProxy := cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)

	timbertruckConfig := ytconfig.NewTimbertruckConfig(
		instanceSpec.StructuredLoggers,
		workDir,
		componentName,
		logsDirectory,
		deliveryProxy,
		getLogsDirectoryPath(timbertruck),
	)
	if timbertruckConfig == nil {
		return nil, nil
	}

	configMapName := labeler.GetSidecarConfigMapName(consts.TimbertruckContainerName)
	return NewConfigMapBuilder(
		labeler,
		proxy,
		configMapName,
		configOverrides,
		ConfigGenerator{
			FileName:  "config.yaml",
			Format:    ConfigFormatYaml,
			Generator: timbertruckConfig.ToYSON,
		},
	), nil
}

// timbertruckConfigMapNeedsSync reports whether the timbertruck configmap
// content differs from what the operator would generate.
func timbertruckConfigMapNeedsSync(
	ctx context.Context,
	proxy apiproxy.APIProxy,
	configOverrides *corev1.LocalObjectReference,
	timbertruck *ytv1.TimbertruckSpec,
	instanceSpec *ytv1.InstanceSpec,
	labeler *labeller.Labeller,
	cfgen *ytconfig.Generator,
) (bool, error) {
	builder, err := newTimbertruckConfigBuilder(proxy, configOverrides, timbertruck, instanceSpec, labeler, cfgen)
	if err != nil || builder == nil {
		return false, err
	}
	if err := builder.Fetch(ctx); err != nil {
		return false, fmt.Errorf("failed to fetch timbertruck configmap: %w", err)
	}
	if !builder.Exists() {
		return true, nil
	}
	status, err := builder.needReload()
	if err != nil {
		return false, err
	}
	return status.IsNeedUpdate(), nil
}

func checkAndAddTimbertruckToPodSpec(ctx context.Context, proxy apiproxy.APIProxy, configOverrides *corev1.LocalObjectReference, timbertruck *ytv1.TimbertruckSpec, podSpec *corev1.PodSpec, instanceSpec *ytv1.InstanceSpec, labeler *labeller.Labeller, cfgen *ytconfig.Generator) error {
	configBuilder, err := newTimbertruckConfigBuilder(proxy, configOverrides, timbertruck, instanceSpec, labeler, cfgen)
	if err != nil {
		return err
	}
	if configBuilder == nil {
		return nil
	}

	if err := configBuilder.Fetch(ctx); err != nil {
		return fmt.Errorf("failed to fetch timbertruck configmap: %w", err)
	}
	if _, err := configBuilder.Build(); err != nil {
		return fmt.Errorf("failed to build timbertruck configmap: %w", err)
	}
	if err := configBuilder.Sync(ctx); err != nil {
		return fmt.Errorf("failed to sync timbertruck configmap: %w", err)
	}

	deliveryProxy := cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)
	configMapName := configBuilder.GetConfigMapName()

	const configVolumeName = consts.TimbertruckContainerName + "-config"
	podSpec.Volumes = append(podSpec.Volumes, createConfigVolume(configVolumeName, configMapName, nil))

	volumeMounts, err := buildTimbertruckVolumeMounts(instanceSpec, configVolumeName)
	if err != nil {
		return err
	}

	podSpec.Containers = append(podSpec.Containers, corev1.Container{
		Name:    consts.TimbertruckContainerName,
		Image:   *timbertruck.Image,
		Command: []string{"/usr/bin/timbertruck_os", "-config", "/etc/timbertruck/config.yaml"},
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
		VolumeMounts:    volumeMounts,
		ImagePullPolicy: corev1.PullIfNotPresent,
	})
	return nil
}

// buildTimbertruckVolumeMounts resolves the spec-derived log volume mount for
// the timbertruck sidecar and appends the read-only mount for its config.
func buildTimbertruckVolumeMounts(instanceSpec *ytv1.InstanceSpec, configVolumeName string) ([]corev1.VolumeMount, error) {
	logMounts, err := resolveLocationMounts(
		instanceSpec,
		[]ytv1.LocationType{ytv1.LocationTypeLogs},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve mounts for timbertruck: %w", err)
	}
	return append(logMounts, corev1.VolumeMount{
		Name:      configVolumeName,
		MountPath: "/etc/timbertruck",
		ReadOnly:  true,
	}), nil
}

func checkAndAddTimbertruckToServerOptions(options *[]Option, timbertruck *ytv1.TimbertruckSpec, structuredLoggers []ytv1.StructuredLoggerSpec) {
	if timbertruck != nil && timbertruck.Image != nil && *timbertruck.Image != "" && len(structuredLoggers) > 0 {
		*options = append(*options, WithSidecarImage(
			consts.TimbertruckContainerName,
			*timbertruck.Image,
		))
	}
}
