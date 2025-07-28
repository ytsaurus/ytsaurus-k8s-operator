package components

import (
	"context"
	"fmt"
	"path"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/yaml"

	"k8s.io/utils/ptr"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
	"go.ytsaurus.tech/yt/go/yterrors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

const timbertruckInitScriptPrefix = "mkdir -p /etc/timbertruck; echo '"
const timbertruckInitScriptSuffix = "' > /etc/timbertruck/config.yaml; chmod 644 /etc/timbertruck/config.yaml; /usr/bin/timbertruck_os -config /etc/timbertruck/config.yaml"

func CreateTabletCells(ctx context.Context, ytClient yt.Client, bundle string, tabletCellCount int) error {
	logger := log.FromContext(ctx)

	var initTabletCellCount int

	if err := ytClient.GetNode(
		ctx,
		ypath.Path(fmt.Sprintf("//sys/tablet_cell_bundles/%s/@tablet_cell_count", bundle)),
		&initTabletCellCount,
		nil); err != nil {
		logger.Error(err, "Getting table_cell_count failed")
		return err
	}

	for i := initTabletCellCount; i < tabletCellCount; i += 1 {
		_, err := ytClient.CreateObject(ctx, "tablet_cell", &yt.CreateObjectOptions{
			Attributes: map[string]interface{}{
				"tablet_cell_bundle": bundle,
			},
		})

		if err != nil {
			logger.Error(err, "Creating tablet_cell failed")
			return err
		}
	}
	return nil
}

func GetNotGoodTabletCellBundles(ctx context.Context, ytClient yt.Client) ([]string, error) {
	var tabletCellBundles []TabletCellBundleHealth
	err := ytClient.ListNode(
		ctx,
		ypath.Path("//sys/tablet_cell_bundles"),
		&tabletCellBundles,
		&yt.ListNodeOptions{Attributes: []string{"health"}})

	if err != nil {
		return nil, err
	}

	notGoodBundles := make([]string, 0)
	for _, bundle := range tabletCellBundles {
		if bundle.Health != "good" {
			notGoodBundles = append(notGoodBundles, bundle.Name)
		}
	}

	return notGoodBundles, err
}

func WaitTabletStateMounted(ctx context.Context, ytClient yt.Client, path ypath.Path) (bool, error) {
	var currentState string
	err := ytClient.GetNode(ctx, path.Attr("tablet_state"), &currentState, nil)
	if err != nil {
		return false, err
	}
	if currentState == yt.TabletMounted {
		return true, nil
	}
	return false, nil
}

func WaitTabletCellHealth(ctx context.Context, ytClient yt.Client, cellID yt.NodeID) (bool, error) {
	var cellHealth string
	err := ytClient.GetNode(ctx, ypath.Path(fmt.Sprintf("//sys/tablet_cells/%s/@health", cellID)), &cellHealth, nil)
	if err != nil {
		return false, err
	}
	if cellHealth == "good" {
		return true, nil
	}
	return false, nil
}

func CreateUser(ctx context.Context, ytClient yt.Client, userName, token string, isSuperuser bool) error {
	var err error

	_, err = ytClient.CreateObject(ctx, yt.NodeUser, &yt.CreateObjectOptions{
		IgnoreExisting: true,
		Attributes: map[string]interface{}{
			"name": userName,
		}})
	if err != nil {
		return err
	}

	if token != "" {
		tokenHash := sha256String(token)
		tokenPath := fmt.Sprintf("//sys/cypress_tokens/%s", tokenHash)

		_, err := ytClient.CreateNode(
			ctx,
			ypath.Path(tokenPath),
			yt.NodeMap,
			&yt.CreateNodeOptions{
				IgnoreExisting: true,
			},
		)
		if err != nil {
			return err
		}

		err = ytClient.SetNode(ctx, ypath.Path(tokenPath).Attr("user"), userName, nil)
		if err != nil {
			return err
		}
	}

	if isSuperuser {
		err = ytClient.AddMember(ctx, "superusers", userName, nil)
		if err != nil && !yterrors.ContainsErrorCode(err, yterrors.CodeAlreadyPresentInGroup) {
			return err
		}
	}

	return nil
}

func IsUpdatingComponent(ytsaurus *apiproxy.Ytsaurus, component Component) bool {
	components := ytsaurus.GetUpdatingComponents()
	for _, c := range components {
		if c.Type == component.GetType() && c.Name == component.GetShortName() {
			return true
		}
	}
	return false
}

func handleUpdatingClusterState(
	ctx context.Context,
	ytsaurus *apiproxy.Ytsaurus,
	cmp Component,
	cmpBase *localComponent,
	server server,
	dry bool,
) (*ComponentStatus, error) {
	var err error

	if IsUpdatingComponent(ytsaurus, cmp) {
		if ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			if !dry {
				err = removePods(ctx, server, cmpBase)
			}
			return ptr.To(WaitingStatus(SyncStatusUpdating, "pods removal")), err
		}

		if ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation {
			return ptr.To(NewComponentStatus(SyncStatusReady, "Nothing to do now")), err
		}
	} else {
		return ptr.To(NewComponentStatus(SyncStatusReady, "Not updating component")), err
	}
	return nil, err
}

func SetPathAcl(path string, acl []yt.ACE) (string, error) {
	formattedAcl, err := yson.MarshalFormat(acl, yson.FormatText)
	if err != nil {
		return "", fmt.Errorf("failed to marshal ACL: %w", err)
	}
	return fmt.Sprintf("/usr/bin/yt set %s/@acl '%s'", path, string(formattedAcl)), nil
}

func AppendPathAcl(path string, acl yt.ACE) (string, error) {
	formattedAcl, err := yson.MarshalFormat(acl, yson.FormatText)
	if err != nil {
		return "", fmt.Errorf("failed to marshal ACL: %w", err)
	}
	return fmt.Sprintf("/usr/bin/yt set %s/@acl/end '%s'", path, string(formattedAcl)), nil
}

func RunIfCondition(condition string, commands ...string) string {
	var wrappedCommands []string
	wrappedCommands = append(wrappedCommands, fmt.Sprintf("if [ %s ]; then", condition))
	wrappedCommands = append(wrappedCommands, commands...)
	wrappedCommands = append(wrappedCommands, "fi")
	return strings.Join(wrappedCommands, "\n")
}

func RunIfNonexistent(path string, commands ...string) string {
	return RunIfCondition(fmt.Sprintf("$(/usr/bin/yt exists %s) = 'false'", path), commands...)
}

func RunIfExists(path string, commands ...string) string {
	return RunIfCondition(fmt.Sprintf("$(/usr/bin/yt exists %s) = 'true'", path), commands...)
}

func SetWithIgnoreExisting(path string, value string) string {
	return RunIfNonexistent(path, fmt.Sprintf("/usr/bin/yt set %s %s", path, value))
}

func AddAffinity(statefulSet *appsv1.StatefulSet,
	nodeSelectorRequirementKey string,
	nodeSelectorRequirementValues []string) {
	affinity := &corev1.Affinity{}
	if statefulSet.Spec.Template.Spec.Affinity != nil {
		affinity = statefulSet.Spec.Template.Spec.Affinity
	}

	nodeAffinity := &corev1.NodeAffinity{}
	if affinity.NodeAffinity != nil {
		nodeAffinity = affinity.NodeAffinity
	}

	selector := &corev1.NodeSelector{}
	if nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		selector = nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	}

	selector.NodeSelectorTerms = append(selector.NodeSelectorTerms, corev1.NodeSelectorTerm{
		MatchExpressions: []corev1.NodeSelectorRequirement{
			{
				Key:      nodeSelectorRequirementKey,
				Operator: corev1.NodeSelectorOpIn,
				Values:   nodeSelectorRequirementValues,
			},
		},
	})
	nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = selector
	affinity.NodeAffinity = nodeAffinity
	statefulSet.Spec.Template.Spec.Affinity = affinity
}

func AddSidecarsToPodSpec(sidecar []string, podSpec *corev1.PodSpec) error {
	for _, sidecarSpec := range sidecar {
		sidecar, err := DecodeSidecar(sidecarSpec)
		if err != nil {
			return err
		}
		podSpec.Containers = append(podSpec.Containers, sidecar)
	}
	return nil
}

func DecodeSidecar(sidecarSpec string) (corev1.Container, error) {
	sidecarContainer := corev1.Container{}
	if err := yaml.UnmarshalStrict([]byte(sidecarSpec), &sidecarContainer); err != nil {
		return corev1.Container{}, fmt.Errorf("failed to parse sidecar: %w", err)
	}
	return sidecarContainer, nil
}

func AddInitContainersToPodSpec(initContainers []string, podSpec *corev1.PodSpec) error {
	containers := make([]corev1.Container, len(initContainers), len(initContainers)+len(podSpec.InitContainers))
	for i, spec := range initContainers {
		if err := yaml.UnmarshalStrict([]byte(spec), &containers[i]); err != nil {
			return err
		}
	}
	// Insert new containers into head
	podSpec.InitContainers = append(containers, podSpec.InitContainers...)
	return nil
}

func getImageWithDefault(componentImage *string, defaultImage string) string {
	if componentImage != nil {
		return *componentImage
	}
	return defaultImage
}

func getTolerationsWithDefault(componentTolerations, defaultTolerations []corev1.Toleration) []corev1.Toleration {
	if len(componentTolerations) != 0 {
		return componentTolerations
	}
	return defaultTolerations
}

func getNodeSelectorWithDefault(componentNodeSelector, defaultNodeSelector map[string]string) map[string]string {
	if len(componentNodeSelector) != 0 {
		return componentNodeSelector
	}
	return defaultNodeSelector
}

func getDNSConfigWithDefault(componentDNSConfig, defaultDNSConfig *corev1.PodDNSConfig) *corev1.PodDNSConfig {
	if componentDNSConfig != nil {
		return componentDNSConfig
	}
	// Otherwise, fall back to the default DNSConfig.
	return defaultDNSConfig
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
						"export_period":    4 * 60 * 60 * 1000, // 4 hours
					},
				},
				"commit_ordering": "strong",
				"optimize_for":    "scan",
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
				"min_data_versions": 0,
				"min_data_ttl":      0,
				"max_data_ttl":      2592000000,
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

func getLogsDeliveryPath(timbertruck *ytv1.TimbertruckSpec) string {
	if timbertruck != nil && timbertruck.LogsDeliveryPath != nil && *timbertruck.LogsDeliveryPath != "" {
		return *timbertruck.LogsDeliveryPath
	}
	return "//sys/admin/logs"
}

func checkAndAddTimbertruckToPodSpec(timbertruck *ytv1.TimbertruckSpec, podSpec *corev1.PodSpec, instanceSpec *ytv1.InstanceSpec, labeler *labeller.Labeller, cfgen *ytconfig.Generator) error {
	if timbertruck == nil || timbertruck.Image == nil || *timbertruck.Image == "" {
		return nil
	}

	if len(instanceSpec.StructuredLoggers) == 0 {
		return nil
	}

	logsDeliveryPath := getLogsDeliveryPath(timbertruck)

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
		Env: []corev1.EnvVar{
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
		},
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

func buildUserCredentialsSecretname(username string) string {
	return fmt.Sprintf("%s-secret", username)
}
