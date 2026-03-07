package components

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/metrics"
)

const (
	timbertruckInitScriptPrefix = "mkdir -p /etc/timbertruck; echo '"
	timbertruckInitScriptSuffix = "' > /etc/timbertruck/config.yaml; chmod 644 /etc/timbertruck/config.yaml; /usr/bin/timbertruck_os -config /etc/timbertruck/config.yaml"
	// OnDeleteUpdateModeWarningTimeout is the duration after which a warning is logged
	// if the component is still waiting for manual pod deletion in OnDelete mode
	OnDeleteUpdateModeWarningTimeout = 15 * 60 // 15 minutes in seconds
)

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
	return ytsaurus.IsUpdatingComponent(component.GetType(), component.GetShortName())
}

func handleUpdatingClusterState(
	ctx context.Context,
	ytsaurus *apiproxy.Ytsaurus,
	cmp Component,
	cmpBase *component,
	server server,
	dry bool,
) (*ComponentStatus, error) {
	var err error

	if IsUpdatingComponent(ytsaurus, cmp) {
		if ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			if !dry {
				err = removePods(ctx, server, cmpBase)
			}
			return ptr.To(ComponentStatusUpdateStep("pods removal")), err
		}

		if ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsCreation {
			return ptr.To(ComponentStatusReady()), err
		}
	} else {
		return ptr.To(ComponentStatusReadyAfter("Not updating component")), err
	}
	return nil, err
}

// handleBulkUpdatingClusterState handles the BulkUpdate mode with pre-checks.
func handleBulkUpdatingClusterState(
	ctx context.Context,
	ytsaurus *apiproxy.Ytsaurus,
	cmp Component,
	cmpBase *component,
	server server,
	dry bool,
) (*ComponentStatus, error) {
	var err error

	switch ytsaurus.GetUpdateState() {
	case ytv1.UpdateStateWaitingForPodsRemoval:
		// Check if this component is using the new update mode
		if !doesComponentUseNewUpdateMode(ytsaurus, cmp.GetType(), cmp.GetFullName()) {
			if !dry {
				err = removePods(ctx, server, cmpBase)
			}
			return ptr.To(ComponentStatusUpdateStep("pods removal")), err
		}
		ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    cmp.GetLabeller().GetCondition(consts.ConditionBulkUpdateModeStarted),
			Status:  metav1.ConditionTrue,
			Reason:  "BulkUpdateModeStarted",
			Message: "bulk update mode started",
		})

		// Run pre-checks if needed
		if ytsaurus.ShouldRunPreChecks(cmp.GetType(), cmp.GetFullName()) {
			if status, err := runPrechecks(ctx, ytsaurus, cmp); status != nil {
				return status, err
			}
		}

		// Remove pods
		if !dry {
			ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
				Type:    cmp.GetLabeller().GetScalingDownCondition(),
				Status:  metav1.ConditionTrue,
				Reason:  "RemovingPods",
				Message: "removing pods",
			})
			err = removePods(ctx, server, cmpBase)
		}
		return ptr.To(ComponentStatusUpdateStep("pods removal")), err

	case ytv1.UpdateStateWaitingForPodsCreation:
		ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    cmp.GetLabeller().GetScalingUpCondition(),
			Status:  metav1.ConditionTrue,
			Reason:  "CreatingPods",
			Message: "creating new pods",
		})
		// Let the component handle its own post-removal / pod-creation logic.
		return nil, err

	default:
		// Not in the pod removal phase, let the component handle other update states
		return nil, err
	}
}

// handleOnDeleteUpdatingClusterState handles the OnDelete mode where pods must be manually deleted by the user.
func handleOnDeleteUpdatingClusterState(
	ctx context.Context,
	ytsaurus *apiproxy.Ytsaurus,
	cmp Component,
	cmpBase *component,
	server server,
	dry bool,
) (*ComponentStatus, error) {
	logger := log.FromContext(ctx)
	var err error

	if ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsRemoval {
		// Not in the pod removal phase, let the component handle other update states
		return nil, err
	}

	onDeleteWaitingCondition := cmp.GetLabeller().GetWaitingOnDeleteUpdateCondition()
	// If this is a dry run, check the update status
	if dry {
		// Check if we've already synced the StatefulSet with OnDelete strategy
		if !ytsaurus.IsUpdateStatusConditionTrue(onDeleteWaitingCondition) {
			return ptr.To(ComponentStatusWaitingFor("OnDelete mode setup")), nil
		}

		// Pods not updated yet, wait for manual action
		return ptr.To(ComponentStatusWaitingFor("manual pod update by user")), nil
	}

	// Run pre-checks if needed
	if ytsaurus.ShouldRunPreChecks(cmp.GetType(), cmp.GetFullName()) {
		if status, err := runPrechecks(ctx, ytsaurus, cmp); status != nil {
			return status, err
		}
	}

	// Set the update strategy to OnDelete
	server.setUpdateStrategy(appsv1.OnDeleteStatefulSetStrategyType, 0, 0)
	logger.Info("Setting StatefulSet update strategy to OnDelete",
		"component", cmp.GetFullName())

	// Sync the StatefulSet
	if err := server.Sync(ctx); err != nil {
		logger.Error(err, "Failed to sync StatefulSet in OnDelete mode", "component", cmp.GetFullName())
		return ptr.To(ComponentStatusBlocked("Failed to sync StatefulSet")), err
	}

	// Fetch the StatefulSet to get the updated status from Kubernetes.
	if err := server.Fetch(ctx); err != nil {
		logger.Error(err, "Failed to fetch StatefulSet after sync", "component", cmp.GetFullName())
		return ptr.To(ComponentStatusBlocked("Failed to fetch StatefulSet")), err
	}

	logger.Info("StatefulSet synced with OnDelete strategy and updated spec",
		"component", cmp.GetFullName())

	// Set condition that OnDelete mode has started
	ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    onDeleteWaitingCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "OnDeleteModeStarted",
		Message: "OnDelete update mode started, StatefulSet synced, waiting for manual pod update",
	})

	// Check if all pods are updated to the new revision
	if server.arePodsUpdatedToNewRevision(ctx) {
		logger.Info("All pods have been updated to the new revision, proceeding with update",
			"component", cmp.GetFullName())

		// Set pods updated condition
		ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    cmp.GetLabeller().GetPodsUpdatedCondition(),
			Status:  metav1.ConditionTrue,
			Reason:  consts.ConditionPodsUpdated,
			Message: "All pods have been updated to new revision",
		})
		ytsaurus.UpdateOnDeleteComponentsSummary(ctx, onDeleteWaitingCondition, false)

		return nil, nil
	}

	// Pods are not yet updated, continue waiting

	// Update the summary with waiting time information
	ytsaurus.UpdateOnDeleteComponentsSummary(ctx, onDeleteWaitingCondition, true)

	return ptr.To(ComponentStatusUpdateStep("pods removal")), err
}

// dispatchComponentUpdate routes the update to the appropriate handler based on
// the component's update strategy from the UpdatePlan (Rolling, OnDelete, or Bulk).
func dispatchComponentUpdate(
	ctx context.Context,
	ytsaurus *apiproxy.Ytsaurus,
	cmp Component,
	cmpBase *component,
	server server,
	dry bool,
) (*ComponentStatus, error) {
	if !IsUpdatingComponent(ytsaurus, cmp) {
		return ptr.To(ComponentStatusReadyAfter("Not updating component")), nil
	}

	switch getComponentUpdateStrategy(ytsaurus, cmp.GetType(), cmp.GetShortName()) {
	case ytv1.ComponentUpdateModeTypeRollingUpdate:
		return handleRollingUpdatingClusterState(ctx, ytsaurus, cmp, server, dry)
	case ytv1.ComponentUpdateModeTypeOnDelete:
		return handleOnDeleteUpdatingClusterState(ctx, ytsaurus, cmp, cmpBase, server, dry)
	default:
		return handleBulkUpdatingClusterState(ctx, ytsaurus, cmp, cmpBase, server, dry)
	}
}

func handleRollingUpdatingClusterState(
	ctx context.Context,
	ytsaurus *apiproxy.Ytsaurus,
	cmp Component,
	server server,
	dry bool,
) (*ComponentStatus, error) {
	logger := log.FromContext(ctx)
	logger.Info("Started Rolling update for component", "component", cmp.GetFullName())
	if ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForPodsRemoval {
		// Not in the pod removal phase, let the component handle other update states.
		logger.Info("Not in the pod removal phase, let the component handle other update states",
			"component", cmp.GetFullName())
		return nil, nil
	}

	minReady := server.getMinReadyInstanceCount()
	if minReady == nil {
		minReady = ptr.To(1)
	}
	maxUnavailable := int(server.getReplicaCount()) - *minReady
	if maxUnavailable <= 0 {
		return ptr.To(ComponentStatusBlocked("instanceCount - minReadyInstanceCount must be positive for rolling update on %s", cmp.GetFullName())), nil
	}

	if dry {
		return ptr.To(ComponentStatusUpdateStep("rolling update")), nil
	}

	sts, ok := server.getRollingUpdateStatus(ctx)
	if !ok {
		return ptr.To(ComponentStatusUpdateStep("rolling update")), nil
	}

	totalCount := sts.totalCount

	// Init guard: the StatefulSet still differs from desired spec (needUpdate=true),
	// so this is the start of a new rolling cycle and we must initialize
	// RollingUpdate strategy with a fresh partition/maxUnavailable.
	if status := server.needUpdate(); status.IsNeedUpdate() {
		server.setUpdateStrategy(appsv1.RollingUpdateStatefulSetStrategyType, totalCount-1, maxUnavailable)
		if err := server.Sync(ctx); err != nil {
			return ptr.To(ComponentStatusBlocked("failed to initialize rolling update for %s", cmp.GetFullName())), err
		}
		return ptr.To(ComponentStatusUpdateStep("rolling update")), nil
	}

	partition := *sts.partition

	// Completion: all pods are exposed and on the new revision.
	if partition == 0 && sts.updatedReplicas == totalCount {
		setPodsUpdatedCondition(ctx, ytsaurus, cmp)
		return nil, nil
	}

	// Budget calculation.
	// inProgress: pods committed to update (below partition) but not yet on the new revision.
	// This covers the timing window where partition was lowered but pods haven't restarted yet.
	inProgress := max(int32(0), (totalCount-partition)-sts.updatedReplicas)
	// effective: take the larger of actual unavailability and in-progress count to avoid
	// double-counting pods that are both in-progress and already unavailable.
	effective := max(totalCount-sts.availableReplicas, inProgress)
	budget := maxUnavailable - int(effective)

	if partition == 0 {
		return ptr.To(ComponentStatusUpdateStep("rolling update")), nil
	}

	if budget <= 0 {
		setRollingBudgetExhaustedCondition(ctx, ytsaurus, cmp, maxUnavailable, int(effective), partition)
		return ptr.To(ComponentStatusUpdateStep("rolling update")), nil
	}

	if ytsaurus.ShouldRunPreChecks(cmp.GetType(), cmp.GetFullName()) {
		if status, err := runPrechecks(ctx, ytsaurus, cmp); status != nil {
			return status, err
		}
	}

	server.setUpdateStrategy(appsv1.RollingUpdateStatefulSetStrategyType, partition-1, maxUnavailable)
	if err := server.Sync(ctx); err != nil {
		return ptr.To(ComponentStatusBlocked("failed to advance rolling update partition for %s", cmp.GetFullName())), err
	}
	return ptr.To(ComponentStatusUpdateStep("rolling update")), nil
}

func setRollingBudgetExhaustedCondition(
	ctx context.Context,
	ytsaurus *apiproxy.Ytsaurus,
	cmp Component,
	maxUnavailable int,
	effectiveUnavailable int,
	partition int32,
) {
	message := fmt.Sprintf(
		"rolling update is paused: budget exhausted (effectiveUnavailable=%d, maxUnavailable=%d, partition=%d)",
		effectiveUnavailable,
		maxUnavailable,
		partition,
	)

	ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    cmp.GetLabeller().GetRollingBudgetExhaustedCondition(),
		Status:  metav1.ConditionTrue,
		Reason:  "BudgetExhausted",
		Message: message,
	})

	resource := ytsaurus.GetResource()
	metrics.ObserveRollingBudgetExhausted(
		resource.Name,
		resource.Namespace,
		cmp.GetLabeller().GetComponentShortName(),
		true,
	)
}

func setPodsUpdatedCondition(ctx context.Context, ytsaurus *apiproxy.Ytsaurus, cmp Component) {
	ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    cmp.GetLabeller().GetPodsUpdatedCondition(),
		Status:  metav1.ConditionTrue,
		Reason:  consts.ConditionPodsUpdated,
		Message: "All pods updated",
	})
}

func runPrechecks(ctx context.Context, ytsaurus *apiproxy.Ytsaurus, cmp Component) (*ComponentStatus, error) {
	ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    cmp.GetLabeller().GetPreChecksRunningCondition(),
		Status:  metav1.ConditionTrue,
		Reason:  "RunningPreChecks",
		Message: "running pre-checks",
	})
	preCheckStatus := cmp.UpdatePreCheck(ctx)
	if preCheckStatus.SyncStatus != SyncStatusReady {
		msg := preCheckStatus.Message
		if msg == "" {
			msg = "pre-checks failed"
		}
		ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    cmp.GetLabeller().GetReadyCondition(),
			Status:  metav1.ConditionFalse,
			Reason:  "PreChecksFailed",
			Message: msg,
		})
		return ptr.To(ComponentStatusBlocked("%v", msg)), yterrors.Err(msg)
	}
	// Set PreChecksCompleted condition for this component
	ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type:    cmp.GetLabeller().GetReadyCondition(),
		Status:  metav1.ConditionTrue,
		Reason:  "PreChecksCompleted",
		Message: "pre-checks completed",
	})
	return nil, nil
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

func doesComponentUseNewUpdateMode(ytsaurus *apiproxy.Ytsaurus, componentType consts.ComponentType, componentName string) bool {
	for _, selector := range ytsaurus.GetResource().Spec.UpdatePlan {
		if selector.Component.Type == componentType &&
			(selector.Component.Name == "" || selector.Component.Name == componentName) {
			return selector.Strategy != nil
		}
	}
	return false
}

func getComponentUpdateStrategy(ytsaurus *apiproxy.Ytsaurus, componentType consts.ComponentType, componentName string) ytv1.ComponentUpdateModeType {
	for _, selector := range ytsaurus.GetResource().Spec.UpdatePlan {
		if selector.Component.Type == componentType &&
			(selector.Component.Name == "" || selector.Component.Name == componentName) &&
			selector.Strategy != nil {
			return selector.Strategy.Type()
		}
	}
	return ""
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

func ptrDefault[T any](ptr, def *T) *T {
	if ptr != nil {
		return ptr
	}
	return def
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

func buildUserCredentialsSecretname(username string) string {
	return fmt.Sprintf("%s-secret", username)
}
