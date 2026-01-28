package components

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

const (
	imageHeaterLabelValue          = "true"
	imageHeaterContainerNamePrefix = "image-heater"
	imagesHeatedHashPrefix         = "hash="
)

type imageHeaterTarget struct {
	name         string
	labeller     *labeller.Labeller
	images       []string
	nodeSelector map[string]string
	tolerations  []corev1.Toleration
}

type ImageHeater struct {
	localComponent
	cfgen            *ytconfig.Generator
	getAllComponents func() []Component
}

func NewImageHeater(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, getAllComponents func() []Component) *ImageHeater {
	l := cfgen.GetComponentLabeller(consts.ImageHeaterType, "")
	return &ImageHeater{
		localComponent:   newLocalComponent(l, ytsaurus),
		cfgen:            cfgen,
		getAllComponents: getAllComponents,
	}
}

func (ih *ImageHeater) Fetch(ctx context.Context) error {
	return nil
}

func (ih *ImageHeater) Status(ctx context.Context) (ComponentStatus, error) {
	return ih.doSync(ctx, true)
}

func (ih *ImageHeater) Sync(ctx context.Context) error {
	_, err := ih.doSync(ctx, false)
	return err
}

// doSync handles preheat creation and cleanup depending on update state.
func (ih *ImageHeater) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	// we need to ImageHeater appear in needUpdate list before we start updating
	if ytv1.IsReadyToUpdateClusterState(ih.ytsaurus.GetClusterState()) {
		needsPreheat, err := ih.needsPreheat(ctx)
		if err != nil {
			return SimpleStatus(SyncStatusUpdating), err
		}
		if needsPreheat {
			return SimpleStatus(SyncStatusNeedUpdate), nil
		}
	}

	if ih.ytsaurus.GetClusterState() != ytv1.ClusterStateUpdating {
		return ComponentStatusReadyAfter("Image heater idle"), nil
	}

	if ih.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForImagesHeated {
		if IsUpdatingComponent(ih.ytsaurus, ih) && ih.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval {
			hasDS, err := ih.cleanupDaemonSetsIfNeeded(ctx, dry)
			if err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}
			if hasDS {
				return ComponentStatusUpdating("Image heater daemonsets are still running"), nil
			}

			if !ih.ytsaurus.IsUpdateStatusConditionTrue(ih.labeller.GetPodsRemovingStartedCondition()) {
				setPodsRemovingStartedCondition(ctx, &ih.localComponent)
			}
			if !ih.ytsaurus.IsUpdateStatusConditionTrue(ih.labeller.GetPodsRemovedCondition()) {
				setPodsRemovedCondition(ctx, &ih.localComponent)
			}
			return SimpleStatus(SyncStatusUpdating), nil
		}

		hasDS, err := ih.cleanupDaemonSetsIfNeeded(ctx, dry)
		if err != nil {
			return SimpleStatus(SyncStatusUpdating), err
		}
		if dry && hasDS {
			return SimpleStatus(SyncStatusUpdating), nil
		}
		return ComponentStatusReadyAfter("Image heater idle"), nil
	}

	if ih.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionImagesHeated) {
		return ComponentStatusReadyAfter("Images already heated"), nil
	}

	targets, err := ih.buildTargets(ctx)
	if err != nil {
		return SimpleStatus(SyncStatusUpdating), err
	}
	targetsHash := imageHeaterTargetsHash(targets)

	if len(targets) == 0 {
		return ih.markImagesHeated(ctx, targetsHash, dry)
	}

	allReady, err := ih.syncTargets(ctx, targets, dry)
	if err != nil {
		return SimpleStatus(SyncStatusUpdating), err
	}

	if !allReady {
		return ComponentStatusUpdating("Image heater daemonsets are not ready"), nil
	}

	return ih.markImagesHeated(ctx, targetsHash, dry)
}

// needsPreheat checks if image preheating is needed based on update plan and current state
func (ih *ImageHeater) needsPreheat(ctx context.Context) (bool, error) {
	selectors := ih.ytsaurus.GetResource().Spec.UpdatePlan
	if len(selectors) == 0 || !updatePlanHasImageHeater(selectors) {
		return false, nil
	}

	targets, err := ih.buildTargets(ctx)
	if err != nil {
		return false, err
	}
	if len(targets) == 0 {
		return false, nil
	}

	targetsHash := imageHeaterTargetsHash(targets)
	conditionImagesHeated := meta.FindStatusCondition(
		ih.ytsaurus.GetResource().Status.UpdateStatus.Conditions, consts.ConditionImagesHeated,
	)

	heatedHash := imagesHeatedHashFromCondition(conditionImagesHeated)
	if targetsHash != "" && heatedHash == targetsHash {
		return false, nil
	}

	if conditionImagesHeated != nil && conditionImagesHeated.Status == metav1.ConditionTrue {
		ih.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    consts.ConditionImagesHeated,
			Status:  metav1.ConditionFalse,
			Reason:  "Update",
			Message: imagesHeatedHashMessage(targetsHash),
		})
	}
	return true, nil
}

func updatePlanHasImageHeater(selectors []ytv1.ComponentUpdateSelector) bool {
	for _, selector := range selectors {
		if selector.Component.Type == consts.ImageHeaterType {
			return true
		}
	}
	return false
}

func (ih *ImageHeater) shouldConsiderComponentForPreheat(component Component) bool {
	if component.GetType() == consts.ImageHeaterType {
		return false
	}
	if !hasNonImageHeaterComponent(ih.ytsaurus.GetUpdatingComponents()) {
		return true
	}
	return IsUpdatingComponent(ih.ytsaurus, component)
}

func (ih *ImageHeater) markImagesHeated(ctx context.Context, targetsHash string, dry bool) (ComponentStatus, error) {
	if !dry {
		ih.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    consts.ConditionImagesHeated,
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: imagesHeatedHashMessage(targetsHash),
		})
		if !hasNonImageHeaterComponent(ih.ytsaurus.GetUpdatingComponents()) {
			if err := ih.cleanupDaemonSets(ctx); err != nil {
				return SimpleStatus(SyncStatusUpdating), err
			}
		}
	}

	return SimpleStatus(SyncStatusUpdating), nil
}

func (ih *ImageHeater) cleanupDaemonSetsIfNeeded(ctx context.Context, dry bool) (bool, error) {
	if dry {
		return ih.hasDaemonSets(ctx)
	}
	if err := ih.cleanupDaemonSets(ctx); err != nil {
		return false, err
	}
	return ih.hasDaemonSets(ctx)
}

func imagesHeatedHashFromCondition(condition *metav1.Condition) string {
	if condition == nil || condition.Status != metav1.ConditionTrue {
		return ""
	}
	return imagesHeatedHashFromMessage(condition.Message)
}

func imagesHeatedHashMessage(hash string) string {
	if hash == "" {
		return ""
	}
	return imagesHeatedHashPrefix + hash
}

func imagesHeatedHashFromMessage(message string) string {
	if !strings.HasPrefix(message, imagesHeatedHashPrefix) {
		return ""
	}
	return strings.TrimPrefix(message, imagesHeatedHashPrefix)
}

func imageHeaterTargetsHash(targets []imageHeaterTarget) string {
	if len(targets) == 0 {
		return ""
	}

	groupedImages := make(map[string][]string)
	for _, target := range targets {
		groupKey := imageHeaterGroupKey(target.nodeSelector, target.tolerations)
		groupedImages[groupKey] = append(groupedImages[groupKey], target.images...)
	}

	keys := make([]string, 0, len(groupedImages))
	for key := range groupedImages {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString("|images:")
		for _, image := range imageHeaterSortedImages(groupedImages[key]) {
			if image == "" {
				continue
			}
			b.WriteString(image)
			b.WriteString(";")
		}
		b.WriteString("|")
	}

	sum := sha256.Sum256([]byte(b.String()))
	return fmt.Sprintf("%x", sum[:])
}

func hasNonImageHeaterComponent(components []ytv1.Component) bool {
	for _, component := range components {
		if component.Type != consts.ImageHeaterType {
			return true
		}
	}
	return false
}

// buildTargets groups images by scheduling constraints and collects unique images per group.
func (ih *ImageHeater) buildTargets(ctx context.Context) ([]imageHeaterTarget, error) {
	targetsByKey := make(map[string]*imageHeaterTarget)
	for _, component := range ih.getAllComponents() {
		if !ih.shouldConsiderComponentForPreheat(component) {
			continue
		}

		// PreheatSpec() of that interface delegates to the server, which resolves image and scheduling constraints.
		provider, ok := component.(PreheatSpecProvider)
		if !ok {
			continue
		}

		images, nodeSelector, tolerations := provider.PreheatSpec()
		if len(images) == 0 || images[0] == "" {
			continue
		}

		needsUpdate, err := ih.componentImageNeedsUpdate(ctx, component, images[0])
		if err != nil {
			return nil, err
		}
		if !needsUpdate {
			continue
		}

		groupKey := imageHeaterGroupKey(nodeSelector, tolerations)
		target := targetsByKey[groupKey]
		if target == nil {
			instanceGroup := imageHeaterInstanceGroup(groupKey)
			l := ih.cfgen.GetComponentLabeller(consts.ImageHeaterType, instanceGroup)
			target = &imageHeaterTarget{
				name:         imageHeaterDaemonSetName(l),
				labeller:     l,
				nodeSelector: nodeSelector,
				tolerations:  tolerations,
			}
			targetsByKey[groupKey] = target
		}

		for _, image := range images {
			if image == "" {
				continue
			}
			if !imageHeaterHasImage(target.images, image) {
				target.images = append(target.images, image)
			}
		}
	}

	keys := make([]string, 0, len(targetsByKey))
	for key := range targetsByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	targets := make([]imageHeaterTarget, 0, len(keys))
	for _, key := range keys {
		targets = append(targets, *targetsByKey[key])
	}
	return targets, nil
}

// componentImageNeedsUpdate checks if the StatefulSet pod template image differs from desired.
func (ih *ImageHeater) componentImageNeedsUpdate(ctx context.Context, component Component, desiredImage string) (bool, error) {
	l := ih.cfgen.GetComponentLabeller(component.GetType(), component.GetShortName())
	sts := resources.NewStatefulSet(
		l.GetServerStatefulSetName(),
		l,
		ih.ytsaurus.APIProxy(),
		&ih.ytsaurus.GetResource().Spec.CommonSpec,
	)
	if err := sts.Fetch(ctx); err != nil {
		return false, err
	}
	if !sts.Exists() {
		return true, nil
	}

	for _, container := range sts.OldObject().Spec.Template.Spec.Containers {
		if container.Name == consts.YTServerContainerName {
			return container.Image != desiredImage, nil
		}
	}

	return true, nil
}

// syncTargets applies daemonsets per target and reports when all are ready.
// it might be several containers per daemonset if there are several images to preheat
func (ih *ImageHeater) syncTargets(ctx context.Context, targets []imageHeaterTarget, dry bool) (bool, error) {
	allReady := true

	for _, target := range targets {

		ds := resources.NewDaemonSet(
			target.name,
			target.labeller,
			ih.ytsaurus,
			target.tolerations,
			target.nodeSelector,
		)
		if err := ds.Fetch(ctx); err != nil {
			return false, err
		}

		dsSpec := ds.Build()
		dsSpec.Labels = ensureImageHeaterLabel(dsSpec.Labels)
		dsSpec.Spec.Template.Labels = ensureImageHeaterLabel(dsSpec.Spec.Template.Labels)
		images := imageHeaterSortedImages(target.images)
		containers := make([]corev1.Container, 0, len(images))
		for _, image := range images {
			containers = append(containers, imageHeaterContainer(image))
		}
		dsSpec.Spec.Template.Spec.Containers = containers

		if !dry {
			if err := ds.Sync(ctx); err != nil {
				return false, err
			}
			if err := ds.Fetch(ctx); err != nil {
				return false, err
			}
		}

		if !ds.Exists() || !ds.ArePodsReady(ctx) {
			allReady = false
		}
	}

	return allReady, nil
}

// cleanupDaemonSets removes image-heater daemonsets.
func (ih *ImageHeater) cleanupDaemonSets(ctx context.Context) error {
	dsList, err := ih.listImageHeaterDaemonSets(ctx)
	if err != nil {
		return err
	}

	for i := range dsList.Items {
		ds := &dsList.Items[i]
		if err := ih.ytsaurus.APIProxy().DeleteObject(ctx, ds); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	return nil
}

// hasDaemonSets returns true if any image-heater daemonsets exist.
func (ih *ImageHeater) hasDaemonSets(ctx context.Context) (bool, error) {
	dsList, err := ih.listImageHeaterDaemonSets(ctx)
	if err != nil {
		return false, err
	}
	return len(dsList.Items) > 0, nil
}

// listImageHeaterDaemonSets returns all image-heater daemonsets in the namespace.
func (ih *ImageHeater) listImageHeaterDaemonSets(ctx context.Context) (*appsv1.DaemonSetList, error) {
	dsList := &appsv1.DaemonSetList{}
	listOptions := []client.ListOption{
		client.InNamespace(ih.ytsaurus.GetResource().GetNamespace()),
		client.MatchingLabels(map[string]string{
			consts.ImageHeaterLabelName: imageHeaterLabelValue,
			consts.YTClusterLabelName:   ih.ytsaurus.GetResource().GetName(),
		}),
	}
	if err := ih.ytsaurus.APIProxy().ListObjects(ctx, dsList, listOptions...); err != nil {
		return nil, err
	}
	return dsList, nil
}

// imageHeaterInstanceGroup returns a stable instance group derived from the group key.
func imageHeaterInstanceGroup(groupKey string) string {
	sum := sha256.Sum256([]byte(groupKey))
	return fmt.Sprintf("group-%x", sum[:6])
}

// imageHeaterDaemonSetName builds a daemonset name consistent with labeller settings.
func imageHeaterDaemonSetName(l *labeller.Labeller) string {
	name := l.GetFullComponentLabel()
	if !l.UseShortNames {
		name = fmt.Sprintf("%s-%s", name, l.ResourceName)
	}
	return name
}

// imageHeaterContainer builds a container that just pulls and holds the image.
func imageHeaterContainer(image string) corev1.Container {
	return corev1.Container{
		Name:    imageHeaterContainerName(image),
		Image:   image,
		Command: []string{"sleep", "inf"},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
		},
	}
}

// ensureImageHeaterLabel injects the image-heater label into the map.
func ensureImageHeaterLabel(labels map[string]string) map[string]string {
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[consts.ImageHeaterLabelName] = imageHeaterLabelValue
	return labels
}

// imageHeaterHasImage checks whether the image is already present in the list.
func imageHeaterHasImage(images []string, image string) bool {
	for _, entry := range images {
		if entry == image {
			return true
		}
	}
	return false
}

// imageHeaterSortedImages returns a sorted copy of the image list.
func imageHeaterSortedImages(images []string) []string {
	if len(images) == 0 {
		return nil
	}
	unique := make([]string, len(images))
	copy(unique, images)
	sort.Strings(unique)
	return unique
}

// imageHeaterContainerName creates a stable container name derived from the image.
func imageHeaterContainerName(image string) string {
	sum := sha256.Sum256([]byte(image))
	return fmt.Sprintf("%s-%x", imageHeaterContainerNamePrefix, sum[:6])
}

// imageHeaterGroupKey produces a deterministic key for scheduling constraints.
func imageHeaterGroupKey(nodeSelector map[string]string, tolerations []corev1.Toleration) string {
	var b strings.Builder

	if len(nodeSelector) > 0 {
		keys := make([]string, 0, len(nodeSelector))
		for key := range nodeSelector {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteString("nodeSelector:")
		for _, key := range keys {
			b.WriteString(key)
			b.WriteString("=")
			b.WriteString(nodeSelector[key])
			b.WriteString(";")
		}
	}

	if len(tolerations) > 0 {
		tols := make([]string, 0, len(tolerations))
		for _, tol := range tolerations {
			tols = append(tols, imageHeaterTolerationKey(tol))
		}
		sort.Strings(tols)
		b.WriteString("tolerations:")
		for _, tol := range tols {
			b.WriteString(tol)
			b.WriteString(";")
		}
	}

	return b.String()
}

// imageHeaterTolerationKey normalizes tolerations into a stable key string.
func imageHeaterTolerationKey(tol corev1.Toleration) string {
	seconds := "nil"
	if tol.TolerationSeconds != nil {
		seconds = strconv.FormatInt(*tol.TolerationSeconds, 10)
	}

	operator := string(tol.Operator)
	if operator == "" {
		operator = string(corev1.TolerationOpEqual)
	}

	return strings.Join([]string{
		tol.Key,
		operator,
		tol.Value,
		string(tol.Effect),
		seconds,
	}, "|")
}
