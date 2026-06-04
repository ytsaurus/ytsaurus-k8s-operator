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

// timbertruckDelivery holds the resolved timbertruck delivery settings for a single component.
type timbertruckDelivery struct {
	Image string
	// LogsDirectory is the component's local logs location path (the sidecar's work dir base).
	LogsDirectory string
	// LogsDeliveryPath is the cypress directory logs are delivered to.
	LogsDeliveryPath string
	Loggers          []ytv1.StructuredLoggerSpec
}

// effectiveTimbertruckImage resolves the timbertruck image for a component: the component-level
// override wins, then the cluster-wide CommonSpec default. Returns "" if neither is set.
func effectiveTimbertruckImage(componentTT, commonTT *ytv1.TimbertruckSpec) string {
	for _, tt := range []*ytv1.TimbertruckSpec{componentTT, commonTT} {
		if tt != nil && tt.Image != nil && *tt.Image != "" {
			return *tt.Image
		}
	}
	return ""
}

// effectiveLogsDeliveryPath resolves the cypress path logs are delivered to, with the same
// component-then-cluster precedence, falling back to the default path.
func effectiveLogsDeliveryPath(componentTT, commonTT *ytv1.TimbertruckSpec) string {
	for _, tt := range []*ytv1.TimbertruckSpec{componentTT, commonTT} {
		if tt != nil && tt.DirectoryPath != nil && *tt.DirectoryPath != "" {
			return *tt.DirectoryPath
		}
	}
	return consts.DefaultTimbertruckDirectoryPath
}

// deliveredStructuredLoggers returns the structured loggers timbertruck should deliver for a
// component. Per-log enableDelivery flags take precedence; when none of the loggers set the flag
// the legacy behaviour applies: if a component-level timbertruck spec is present, all structured
// loggers are delivered. Only masters currently set a component-level spec, so in practice this
// legacy path is the master backward-compatibility mode.
func deliveredStructuredLoggers(componentTT *ytv1.TimbertruckSpec, loggers []ytv1.StructuredLoggerSpec) []ytv1.StructuredLoggerSpec {
	var explicit []ytv1.StructuredLoggerSpec
	anyExplicit := false
	for _, logger := range loggers {
		if logger.EnableDelivery == nil {
			continue
		}
		anyExplicit = true
		if *logger.EnableDelivery {
			explicit = append(explicit, logger)
		}
	}
	if anyExplicit {
		return explicit
	}
	if componentTT != nil {
		return loggers
	}
	return nil
}

// resolveTimbertruckDelivery returns the resolved delivery settings for a component, or nil if
// timbertruck delivery is not enabled for it. Delivery requires loggers to deliver, a configured
// image, and a logs location on the instance. This is the single source of truth shared by
// serverImpl (sidecar/configmap), TimbertruckDeliveryEnabled (virtual component + update step) and
// GetDeliveryLoggers (YT-side preparation), so all of them agree on which components deliver.
func resolveTimbertruckDelivery(componentTT, commonTT *ytv1.TimbertruckSpec, instanceSpec *ytv1.InstanceSpec) *timbertruckDelivery {
	delivered := deliveredStructuredLoggers(componentTT, instanceSpec.StructuredLoggers)
	if len(delivered) == 0 {
		return nil
	}
	image := effectiveTimbertruckImage(componentTT, commonTT)
	if image == "" {
		return nil
	}
	logsLocation := ytv1.FindFirstLocation(instanceSpec.Locations, ytv1.LocationTypeLogs)
	if logsLocation == nil {
		return nil
	}
	return &timbertruckDelivery{
		Image:            image,
		LogsDirectory:    logsLocation.Path,
		LogsDeliveryPath: effectiveLogsDeliveryPath(componentTT, commonTT),
		Loggers:          delivered,
	}
}

// timbertruckComponentName is the per-component name used to build delivery names and the
// corresponding YT queue paths. It must be identical between the sidecar config (built in
// serverImpl) and the YT-side table preparation (GetDeliveryLoggers), so it is derived solely
// from the labeller. For single-group components (e.g. primary master) it equals the service
// kebab-case name, preserving the original master paths.
func timbertruckComponentName(l *labeller.Labeller) string {
	name := consts.GetServiceKebabCase(l.ComponentType)
	if l.InstanceGroup != "" && l.InstanceGroup != consts.DefaultName {
		name += "-" + l.InstanceGroup
	}
	return name
}

// timbertruckLoggerSource describes one server component that may deliver structured logs.
type timbertruckLoggerSource struct {
	componentTT  *ytv1.TimbertruckSpec
	instanceSpec *ytv1.InstanceSpec
	// buildLabeller produces the component labeller; needed only to compute delivery names.
	buildLabeller func(cfgen *ytconfig.Generator) *labeller.Labeller
}

// timbertruckLoggerSources enumerates every server component that may deliver logs, pairing each
// with its component-level timbertruck override (nil for non-masters) and its instance spec
// (structured loggers + logs location). This is the single source of truth for which components
// participate in timbertruck delivery.
func timbertruckLoggerSources(spec *ytv1.YtsaurusSpec) []timbertruckLoggerSource {
	var sources []timbertruckLoggerSource
	addMaster := func(componentTT *ytv1.TimbertruckSpec, instanceSpec *ytv1.InstanceSpec, cellTag uint16) {
		sources = append(sources, timbertruckLoggerSource{componentTT, instanceSpec, func(cfgen *ytconfig.Generator) *labeller.Labeller {
			return cfgen.GetMasterLabeller(cellTag)
		}})
	}
	add := func(componentType consts.ComponentType, instanceGroup string, instanceSpec *ytv1.InstanceSpec) {
		sources = append(sources, timbertruckLoggerSource{nil, instanceSpec, func(cfgen *ytconfig.Generator) *labeller.Labeller {
			return cfgen.GetComponentLabeller(componentType, instanceGroup)
		}})
	}

	// Masters keep a per-component timbertruck override for backward compatibility.
	addMaster(spec.PrimaryMasters.Timbertruck, &spec.PrimaryMasters.InstanceSpec, spec.PrimaryMasters.CellTag)
	for i := range spec.SecondaryMasters {
		sm := &spec.SecondaryMasters[i]
		addMaster(sm.Timbertruck, &sm.InstanceSpec, sm.CellTag)
	}

	// All other server components rely on the cluster-wide spec.timbertruck plus per-log enableDelivery.
	if spec.MasterCaches != nil {
		add(consts.MasterCacheType, "", &spec.MasterCaches.InstanceSpec)
	}
	add(consts.DiscoveryType, "", &spec.Discovery.InstanceSpec)
	for i := range spec.HTTPProxies {
		add(consts.HttpProxyType, spec.HTTPProxies[i].Role, &spec.HTTPProxies[i].InstanceSpec)
	}
	for i := range spec.RPCProxies {
		add(consts.RpcProxyType, spec.RPCProxies[i].Role, &spec.RPCProxies[i].InstanceSpec)
	}
	for i := range spec.TCPProxies {
		add(consts.TcpProxyType, spec.TCPProxies[i].Role, &spec.TCPProxies[i].InstanceSpec)
	}
	for i := range spec.KafkaProxies {
		add(consts.KafkaProxyType, spec.KafkaProxies[i].Role, &spec.KafkaProxies[i].InstanceSpec)
	}
	for i := range spec.DataNodes {
		add(consts.DataNodeType, spec.DataNodes[i].Name, &spec.DataNodes[i].InstanceSpec)
	}
	for i := range spec.ExecNodes {
		add(consts.ExecNodeType, spec.ExecNodes[i].Name, &spec.ExecNodes[i].InstanceSpec)
	}
	for i := range spec.TabletNodes {
		add(consts.TabletNodeType, spec.TabletNodes[i].Name, &spec.TabletNodes[i].InstanceSpec)
	}
	if spec.Schedulers != nil {
		add(consts.SchedulerType, "", &spec.Schedulers.InstanceSpec)
	}
	if spec.ControllerAgents != nil {
		add(consts.ControllerAgentType, "", &spec.ControllerAgents.InstanceSpec)
	}
	if spec.QueryTrackers != nil {
		add(consts.QueryTrackerType, "", &spec.QueryTrackers.InstanceSpec)
	}
	if spec.YQLAgents != nil {
		add(consts.YqlAgentType, "", &spec.YQLAgents.InstanceSpec)
	}
	if spec.QueueAgents != nil {
		add(consts.QueueAgentType, "", &spec.QueueAgents.InstanceSpec)
	}
	if spec.CypressProxies != nil {
		add(consts.CypressProxyType, "", &spec.CypressProxies.InstanceSpec)
	}
	if spec.BundleController != nil {
		add(consts.BundleControllerType, "", &spec.BundleController.InstanceSpec)
	}
	if spec.TabletBalancer != nil {
		add(consts.TabletBalancerType, "", &spec.TabletBalancer.InstanceSpec)
	}

	return sources
}

// TimbertruckDeliveryEnabled reports whether any server component in the cluster has timbertruck log
// delivery enabled. It mirrors the per-component resolution used by serverImpl, so the virtual
// Timbertruck component (user/queue/export setup) and its update-flow step are created exactly when
// at least one component will run a timbertruck sidecar.
func TimbertruckDeliveryEnabled(spec *ytv1.YtsaurusSpec) bool {
	for _, source := range timbertruckLoggerSources(spec) {
		if resolveTimbertruckDelivery(source.componentTT, spec.Timbertruck, source.instanceSpec) != nil {
			return true
		}
	}
	return false
}

// GetDeliveryLoggers enumerates every component that has timbertruck delivery enabled, so the
// virtual Timbertruck component can prepare the YT-side user, queues, producers and exports.
// The component name and delivery path must match what serverImpl writes into each sidecar config.
func (tt *Timbertruck) GetDeliveryLoggers() []ComponentLoggers {
	spec := &tt.ytsaurus.GetResource().Spec
	commonTT := spec.Timbertruck
	var result []ComponentLoggers

	for _, source := range timbertruckLoggerSources(spec) {
		delivery := resolveTimbertruckDelivery(source.componentTT, commonTT, source.instanceSpec)
		if delivery == nil {
			continue
		}
		result = append(result, ComponentLoggers{
			ComponentName:     timbertruckComponentName(source.buildLabeller(tt.cfgen)),
			StructuredLoggers: delivery.Loggers,
			LogsDeliveryPath:  delivery.LogsDeliveryPath,
		})
	}

	return result
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

// buildTimbertruckConfigMap builds the ConfigMapBuilder for a component's timbertruck sidecar
// config from already-resolved delivery settings (which include the component's logs location).
// Callers must only invoke it when delivery is enabled.
func buildTimbertruckConfigMap(
	proxy apiproxy.APIProxy,
	configOverrides *corev1.LocalObjectReference,
	delivery *timbertruckDelivery,
	labeler *labeller.Labeller,
	cfgen *ytconfig.Generator,
) *ConfigMapBuilder {
	workDir := fmt.Sprintf("%s/%s", delivery.LogsDirectory, consts.TimbertruckWorkDirName)
	deliveryProxy := cfgen.GetHTTPProxiesAddress(consts.DefaultHTTPProxyRole)

	timbertruckConfig := ytconfig.NewTimbertruckConfig(
		delivery.Loggers,
		workDir,
		timbertruckComponentName(labeler),
		delivery.LogsDirectory,
		deliveryProxy,
		delivery.LogsDeliveryPath,
	)
	if timbertruckConfig == nil {
		return nil
	}

	return NewConfigMapBuilder(
		labeler,
		proxy,
		labeler.GetSidecarConfigMapName(consts.TimbertruckContainerName),
		configOverrides,
		ConfigGenerator{
			FileName:  "config.yaml",
			Format:    ConfigFormatYaml,
			Generator: timbertruckConfig.ToYSON,
		},
	)
}

// addTimbertruckSidecar appends the timbertruck sidecar container and its config volume to podSpec.
// The logs location mount is resolved from instanceSpec, so the sidecar sees the very same volume
// (and subPath) as the server container.
func addTimbertruckSidecar(podSpec *corev1.PodSpec, image string, instanceSpec *ytv1.InstanceSpec, configMapName, deliveryProxy string) error {
	const configVolumeName = consts.TimbertruckContainerName + "-config"

	volumeMounts, err := buildTimbertruckVolumeMounts(instanceSpec, configVolumeName)
	if err != nil {
		return err
	}

	podSpec.Volumes = append(podSpec.Volumes, createConfigVolume(configVolumeName, configMapName, nil))

	podSpec.Containers = append(podSpec.Containers, corev1.Container{
		Name:    consts.TimbertruckContainerName,
		Image:   image,
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
