package components

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/util/yaml"

	"k8s.io/utils/ptr"

	"go.ytsaurus.tech/yt/go/ypath"
	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yt"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
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
		_ = ytClient.AddMember(ctx, "superusers", userName, nil)
	}

	return err
}

func IsUpdatingComponent(ytsaurus *apiproxy.Ytsaurus, component Component) bool {
	componentNames := ytsaurus.GetLocalUpdatingComponents()
	return (componentNames == nil && component.IsUpdatable()) || slices.Contains(componentNames, component.GetName())
}

func handleUpdatingClusterState(
	ctx context.Context,
	ytsaurus *apiproxy.Ytsaurus,
	cmp Component,
	cmpBase *localComponent,
	server server,
	dry bool,
) (*ComponentStatus, error) {
	return handleUpdatingClusterStateForUpdateStates(
		ctx,
		ytsaurus,
		cmp,
		cmpBase,
		server,
		dry,
		ytv1.UpdateStateWaitingForPodsRemoval,
		ytv1.UpdateStateWaitingForPodsCreation,
	)
}

func handleUpdatingClusterStateForMaster(
	ctx context.Context,
	ytsaurus *apiproxy.Ytsaurus,
	cmp Component,
	cmpBase *localComponent,
	server server,
	dry bool,
) (*ComponentStatus, error) {
	return handleUpdatingClusterStateForUpdateStates(
		ctx,
		ytsaurus,
		cmp,
		cmpBase,
		server,
		dry,
		ytv1.UpdateStateWaitingForMasterPodsRemoval,
		ytv1.UpdateStateWaitingForMasterPodsCreation,
	)
}

func handleUpdatingClusterStateForUpdateStates(
	ctx context.Context,
	ytsaurus *apiproxy.Ytsaurus,
	cmp Component,
	cmpBase *localComponent,
	server server,
	dry bool,
	podsRemovalState ytv1.UpdateState,
	podsCreationState ytv1.UpdateState,
) (*ComponentStatus, error) {
	var err error

	if IsUpdatingComponent(ytsaurus, cmp) {
		if ytsaurus.GetUpdateState() == podsRemovalState {
			if !dry {
				err = removePods(ctx, server, cmpBase)
			}
			return ptr.To(WaitingStatus(SyncStatusUpdating, "pods removal")), err
		}

		if ytsaurus.GetUpdateState() != podsCreationState {
			return ptr.To(NewComponentStatus(SyncStatusReady, "Nothing to do now")), err
		}
	} else {
		return ptr.To(NewComponentStatus(SyncStatusReady, "Not updating component")), err
	}
	return nil, err
}

func SetPathAcl(path string, acl []yt.ACE) string {
	formattedAcl, err := yson.MarshalFormat(acl, yson.FormatText)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("/usr/bin/yt set %s/@acl '%s'", path, string(formattedAcl))
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
		sidecar := corev1.Container{}
		if err := yaml.UnmarshalStrict([]byte(sidecarSpec), &sidecar); err != nil {
			return err
		}
		podSpec.Containers = append(podSpec.Containers, sidecar)
	}
	return nil
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
