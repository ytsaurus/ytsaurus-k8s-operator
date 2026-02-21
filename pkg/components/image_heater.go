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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

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
	imageHeaterHashPrefix          = "hash="
)

type imageHeaterTarget struct {
	name         string
	labeller     *labeller.Labeller
	images       []string
	nodeSelector map[string]string
	tolerations  []corev1.Toleration
}

type ImageHeater struct {
	component
	cfgen            *ytconfig.Generator
	getAllComponents func() []Component
}

func NewImageHeater(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, getAllComponents func() []Component) *ImageHeater {
	l := cfgen.GetComponentLabeller(consts.ImageHeaterType, "")
	return &ImageHeater{
		component:        newComponent(l, ytsaurus),
		cfgen:            cfgen,
		getAllComponents: getAllComponents,
	}
}

func (ih *ImageHeater) Fetch(ctx context.Context) error {
	return nil
}

func (ih *ImageHeater) Exists() bool {
	return true
}

func (ih *ImageHeater) NeedSync() bool {
	return false
}

func (ih *ImageHeater) NeedUpdate() ComponentStatus {
	return ComponentStatus{}
}

func (ih *ImageHeater) Status(ctx context.Context) (ComponentStatus, error) {
	return ih.doSync(ctx, true)
}

func (ih *ImageHeater) Sync(ctx context.Context) error {
	_, err := ih.doSync(ctx, false)
	return err
}

// doSync drives ImageHeater lifecycle:
// - returns NeedUpdate when a new preheat is required.
// - runs preheat only during UpdateStateWaitingForImageHeater or during cluster creation as step 0.
// - marks readiness via ImageHeaterReady condition when targets are heated.
func (ih *ImageHeater) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	var err error

	if status, handled, err := ih.runAtClusterCreationStage(ctx, dry); handled || err != nil {
		return status, err
	}

	// we need to ImageHeater appear in needUpdate list before we start updating
	if ih.ytsaurus.IsReadyToUpdate() {
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

	if ih.ytsaurus.GetUpdateState() != ytv1.UpdateStateWaitingForImageHeater {
		return ComponentStatusReadyAfter("Image heater idle"), nil
	}

	if ih.ytsaurus.IsUpdateStatusConditionTrue(consts.ConditionImageHeaterReady) {
		return ComponentStatusReadyAfter("Images already heated"), nil
	}

	targets, err := ih.buildTargets(ctx)
	if err != nil {
		return SimpleStatus(SyncStatusUpdating), err
	}
	targetsHash := imageHeaterTargetsHash(targets)

	if len(targets) == 0 {
		return ih.markImageHeaterReady(ctx, targetsHash, dry)
	}

	allReady, err := ih.syncTargets(ctx, targets, dry)
	if err != nil {
		return SimpleStatus(SyncStatusUpdating), err
	}

	if !allReady {
		return ComponentStatusUpdating("Image heater daemonsets are not ready"), nil
	}

	return ih.markImageHeaterReady(ctx, targetsHash, dry)
}

func (ih *ImageHeater) isInitState() bool {
	switch ih.ytsaurus.GetClusterState() {
	case ytv1.ClusterStateInitializing, ytv1.ClusterStateCreated, ytv1.ClusterState(""):
		return true
	default:
		return false
	}
}

func (ih *ImageHeater) runAtClusterCreationStage(ctx context.Context, dry bool) (ComponentStatus, bool, error) {
	// If image heater is enabled and cluster is initializing/created, we need to deploy it first.
	if !ih.ytsaurus.IsImageHeaterEnabled() || !ih.isInitState() {
		return ComponentStatus{}, false, nil
	}

	targets, err := ih.buildTargets(ctx)
	if err != nil {
		return SimpleStatus(SyncStatusUpdating), true, err
	}
	targetsHash := imageHeaterTargetsHash(targets)

	if len(targets) == 0 {
		status, err := ih.markImageHeaterReadyInit(ctx, targetsHash, dry)
		return status, true, err
	}

	allReady, err := ih.syncTargets(ctx, targets, dry)
	if err != nil {
		return SimpleStatus(SyncStatusUpdating), true, err
	}

	if !allReady {
		return ComponentStatusUpdating("Image heater daemonsets are not ready"), true, nil
	}

	status, err := ih.markImageHeaterReadyInit(ctx, targetsHash, dry)
	return status, true, err
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
	ConditionImageHeaterReady := meta.FindStatusCondition(
		ih.ytsaurus.GetResource().Status.UpdateStatus.Conditions, consts.ConditionImageHeaterReady,
	)

	imageHeaterHash := imageHeaterHashFromCondition(ConditionImageHeaterReady)
	if targetsHash != "" && imageHeaterHash == targetsHash {
		return false, nil
	}

	if ConditionImageHeaterReady != nil && ConditionImageHeaterReady.Status == metav1.ConditionTrue {
		ih.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    consts.ConditionImageHeaterReady,
			Status:  metav1.ConditionFalse,
			Reason:  "Update",
			Message: imageHeaterConditionMessage(targetsHash),
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
	if !hasNonImageHeaterUpdatingComponent(ih.ytsaurus.GetUpdatingComponents()) {
		return true
	}
	return IsUpdatingComponent(ih.ytsaurus, component)
}

func (ih *ImageHeater) markImageHeaterReady(ctx context.Context, targetsHash string, dry bool) (ComponentStatus, error) {
	ih.setImageHeaterReadyCondition(ctx, targetsHash, dry)
	return SimpleStatus(SyncStatusUpdating), nil
}

func (ih *ImageHeater) markImageHeaterReadyInit(ctx context.Context, targetsHash string, dry bool) (ComponentStatus, error) {
	ih.setImageHeaterReadyCondition(ctx, targetsHash, dry)
	return ComponentStatusReadyAfter("Image heater preheated"), nil
}

func (ih *ImageHeater) setImageHeaterReadyCondition(ctx context.Context, targetsHash string, dry bool) {
	if !dry {
		ih.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type:    consts.ConditionImageHeaterReady,
			Status:  metav1.ConditionTrue,
			Reason:  "Update",
			Message: imageHeaterConditionMessage(targetsHash),
		})
	}
}
func imageHeaterHashFromCondition(condition *metav1.Condition) string {
	if condition == nil || condition.Status != metav1.ConditionTrue {
		return ""
	}
	return imageHeaterHashFromMessage(condition.Message)
}

func imageHeaterConditionMessage(hash string) string {
	if hash == "" {
		return ""
	}
	return imageHeaterHashPrefix + hash
}

func imageHeaterHashFromMessage(message string) string {
	if !strings.HasPrefix(message, imageHeaterHashPrefix) {
		return ""
	}
	return strings.TrimPrefix(message, imageHeaterHashPrefix)
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

func hasNonImageHeaterUpdatingComponent(components []ytv1.Component) bool {
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
		ih.ytsaurus,
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

// syncTargets reconciles image-heater DaemonSets for each target and reports readiness.
// A target is considered ready only when:
// - the DaemonSet exists and its template matches the desired images,
// - no heater pod is terminating,
// - heater pods contain exactly the desired images,
// - and DaemonSet status reports all pods updated/available.
func (ih *ImageHeater) syncTargets(ctx context.Context, targets []imageHeaterTarget, dry bool) (bool, error) {
	logger := log.FromContext(ctx)
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

		desiredImages, desiredImagesSet := imageHeaterDesiredImages(target.images)
		imageHeaterApplyDaemonSetSpec(ds, desiredImages)

		if err := imageHeaterSyncAndFetch(ctx, ds, dry); err != nil {
			return false, err
		}

		imageHeaterLogDaemonSetStatus(ctx, ds, target.name)

		if !ds.Exists() {
			logger.Info("Image heater daemonset not ready", "daemonset", target.name)
			allReady = false
			continue
		}

		currentImages := imageHeaterDaemonSetImages(ds.OldObject())
		if !imageHeaterImagesEqual(currentImages, desiredImages) {
			logger.Info("Image heater daemonset spec not yet applied",
				"daemonset", target.name,
				"currentImages", currentImages,
				"desiredImages", desiredImages,
			)
			allReady = false
			continue
		}

		podCheck, err := imageHeaterCheckPods(ctx, ds, desiredImagesSet)
		if err != nil {
			return false, err
		}
		if !podCheck.ok {
			logFields := []any{"daemonset", target.name}
			if podCheck.podName != "" {
				logFields = append(logFields, "pod", podCheck.podName)
			}
			if podCheck.image != "" {
				logFields = append(logFields, "image", podCheck.image)
			}
			logger.Info(podCheck.reason, logFields...)
			allReady = false
			continue
		}

		if !ds.ArePodsReady(ctx) {
			logger.Info("Image heater daemonset not ready", "daemonset", target.name)
			allReady = false
		}
	}

	return allReady, nil
}

type imageHeaterPodCheckResult struct {
	ok      bool
	reason  string
	podName string
	image   string
}

func imageHeaterDesiredImages(images []string) (desiredImages []string, imageSet map[string]struct{}) {
	desired := imageHeaterSortedImages(images)
	set := make(map[string]struct{}, len(desired))
	for _, image := range desired {
		set[image] = struct{}{}
	}
	return desired, set
}

func imageHeaterApplyDaemonSetSpec(ds *resources.DaemonSet, desiredImages []string) {
	dsSpec := ds.Build()
	dsSpec.Labels = ensureImageHeaterLabel(dsSpec.Labels)
	dsSpec.Spec.Template.Labels = ensureImageHeaterLabel(dsSpec.Spec.Template.Labels)
	containers := make([]corev1.Container, 0, len(desiredImages))
	for _, image := range desiredImages {
		containers = append(containers, imageHeaterContainer(image))
	}
	dsSpec.Spec.Template.Spec.Containers = containers
}

func imageHeaterSyncAndFetch(ctx context.Context, ds *resources.DaemonSet, dry bool) error {
	if dry {
		return nil
	}
	if err := ds.Sync(ctx); err != nil {
		return err
	}
	return ds.Fetch(ctx)
}

func imageHeaterLogDaemonSetStatus(ctx context.Context, ds *resources.DaemonSet, name string) {
	logger := log.FromContext(ctx)
	dsObj := ds.OldObject()
	if dsObj == nil {
		logger.Info("Image heater daemonset status unavailable", "daemonset", name)
		return
	}

	logger.Info("Image heater daemonset status",
		"daemonset", name,
		"exists", ds.Exists(),
		"generation", dsObj.Generation,
		"observedGeneration", dsObj.Status.ObservedGeneration,
		"desiredNumberScheduled", dsObj.Status.DesiredNumberScheduled,
		"updatedNumberScheduled", dsObj.Status.UpdatedNumberScheduled,
		"readyNumberScheduled", dsObj.Status.NumberReady,
		"numberAvailable", dsObj.Status.NumberAvailable,
		"numberUnavailable", dsObj.Status.NumberUnavailable,
	)
}

// imageHeaterCheckPods checks if the daemonset pods are ready and have the correct images.
func imageHeaterCheckPods(ctx context.Context, ds *resources.DaemonSet, desiredImagesSet map[string]struct{}) (imageHeaterPodCheckResult, error) {
	dsObj := ds.OldObject()
	if dsObj == nil || dsObj.Status.DesiredNumberScheduled == 0 {
		return imageHeaterPodCheckResult{ok: true}, nil
	}

	pods, err := ds.ListPods(ctx)
	if err != nil {
		return imageHeaterPodCheckResult{}, err
	}
	return imageHeaterValidatePods(pods, desiredImagesSet), nil
}

// imageHeaterValidatePods checks if ImageHeater's pods are not terminating and have the desired images.
func imageHeaterValidatePods(pods []corev1.Pod, desiredImagesSet map[string]struct{}) imageHeaterPodCheckResult {
	if len(pods) == 0 {
		return imageHeaterPodCheckResult{
			ok:     false,
			reason: "Image heater daemonset has no pods yet",
		}
	}

	for _, pod := range pods {
		if pod.DeletionTimestamp != nil {
			return imageHeaterPodCheckResult{
				ok:      false,
				reason:  "Image heater daemonset has terminating pod",
				podName: pod.Name,
			}
		}

		podImages := make(map[string]struct{})
		for _, container := range pod.Spec.Containers {
			if strings.HasPrefix(container.Name, imageHeaterContainerNamePrefix) {
				podImages[container.Image] = struct{}{}
			}
		}

		if len(podImages) == 0 {
			return imageHeaterPodCheckResult{
				ok:      false,
				reason:  "Image heater daemonset pod has no heater containers",
				podName: pod.Name,
			}
		}

		for image := range desiredImagesSet {
			if _, ok := podImages[image]; !ok {
				return imageHeaterPodCheckResult{
					ok:      false,
					reason:  "Image heater daemonset pod missing desired image",
					podName: pod.Name,
					image:   image,
				}
			}
		}

		for image := range podImages {
			if _, ok := desiredImagesSet[image]; !ok {
				return imageHeaterPodCheckResult{
					ok:      false,
					reason:  "Image heater daemonset pod has unexpected image",
					podName: pod.Name,
					image:   image,
				}
			}
		}
	}

	return imageHeaterPodCheckResult{ok: true}
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

func imageHeaterDaemonSetImages(ds *appsv1.DaemonSet) []string {
	if ds == nil {
		return nil
	}
	images := make([]string, 0, len(ds.Spec.Template.Spec.Containers))
	for _, container := range ds.Spec.Template.Spec.Containers {
		if strings.HasPrefix(container.Name, imageHeaterContainerNamePrefix) {
			images = append(images, container.Image)
		}
	}
	return imageHeaterSortedImages(images)
}

func imageHeaterImagesEqual(current, desired []string) bool {
	if len(current) != len(desired) {
		return false
	}
	for i := range current {
		if current[i] != desired[i] {
			return false
		}
	}
	return true
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
