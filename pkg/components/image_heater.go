package components

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"k8s.io/utils/ptr"

	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

type ImageHeater struct {
	component

	cfgen            *ytconfig.Generator
	getAllComponents func() []Component

	daemonSetList appsv1.DaemonSetList
	nameToHeater  map[string]*appsv1.DaemonSet
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
	err := ih.owner.ListObjects(ctx, &ih.daemonSetList,
		client.InNamespace(ih.labeller.GetNamespace()),
		client.MatchingLabels{consts.YTClusterLabelName: ih.labeller.GetClusterName()},
		client.HasLabels{consts.ImageHeaterLabelName})
	if err != nil {
		return err
	}
	ih.nameToHeater = make(map[string]*appsv1.DaemonSet)
	for i := range ih.daemonSetList.Items {
		obj := &ih.daemonSetList.Items[i]
		for _, name := range strings.Split(obj.Annotations[consts.ImageHeaterTargetsAnnotationName], " ") {
			ih.nameToHeater[name] = obj
		}
	}
	return nil
}

func (ih *ImageHeater) Exists() bool {
	return ih.owner.IsStatusConditionTrue(consts.ConditionImageHeaterReady)
}

func (ih *ImageHeater) NeedSync() bool {
	// Image heaters need no updates unless something have changed in spec.
	return !ih.owner.IsStatusConditionTrueAndObservedGeneration(consts.ConditionImageHeaterReady)
}

func (ih *ImageHeater) NeedUpdate() ComponentStatus {
	return ComponentStatus{}
}

func (ih *ImageHeater) Status(ctx context.Context) (ComponentStatus, error) {
	return ih.doSync(ctx, true)
}

func (ih *ImageHeater) GetHeaterStatus(component ytv1.Component) (ready bool, message string) {
	name := ih.labeller.ForComponent(component.Type, component.Name).GetImageHeaterName()
	if heater, found := ih.nameToHeater[name]; !found {
		return true, "no active image heater for " + name
	} else {
		return resources.GetDaemonSetStatus(heater)
	}
}

func (ih *ImageHeater) Sync(ctx context.Context) error {
	_, err := ih.doSync(ctx, false)
	return err
}

func (ih *ImageHeater) doSync(ctx context.Context, dry bool) (ComponentStatus, error) {
	// Fast path if nothing have changed since last completion.
	if !ih.NeedSync() {
		return ComponentStatusReadyAfter("Image heating is complete"), nil
	}

	logger := ctrllog.FromContext(ctx)
	hashToHeater := map[string]*resources.DaemonSet{}
	hashToNames := map[string][]string{}
	toApply := map[string]*resources.DaemonSet{}
	toDelete := map[string]*appsv1.DaemonSet{}

	// Build all required heaters.
	for _, component := range ih.getAllComponents() {
		name := component.GetLabeller().GetImageHeaterName()
		target := component.GetImageHeaterTarget()
		if target == nil {
			continue
		}

		heater, hash, err := ih.buildImageHeater(name, target)
		if err != nil {
			return ComponentStatusPending("Cannot build image heater"), err
		}
		if err := heater.Fetch(ctx); err != nil {
			return ComponentStatusPending("Cannot fetch image heater"), err
		}

		// Deduplicate daemon sets.
		if other, found := hashToHeater[hash]; !found {
			hashToHeater[hash] = heater
			hashToNames[hash] = []string{name}
		} else if heater.Name() < other.Name() && (heater.Exists() || !other.Exists()) {
			hashToHeater[hash] = heater
			hashToNames[hash] = append(hashToNames[hash], name)
			if other.Exists() {
				toDelete[other.Name()] = other.OldObject()
			}
		} else {
			hashToNames[hash] = append(hashToNames[hash], name)
			if heater.Exists() {
				toDelete[heater.Name()] = heater.OldObject()
			}
		}
	}

	// Collect required changes.
	for hash, heater := range hashToHeater {
		slices.Sort(hashToNames[hash])
		targets := strings.Join(hashToNames[hash], " ")
		metav1.SetMetaDataAnnotation(&heater.NewObject().ObjectMeta, consts.ImageHeaterTargetsAnnotationName, targets)
		toApply[heater.Name()] = heater
	}

	for i := range ih.daemonSetList.Items {
		ds := &ih.daemonSetList.Items[i]
		if _, keep := toApply[ds.Name]; !keep {
			toDelete[ds.Name] = ds
		}
	}

	// Apply changes in alphabetic order.
	if !dry {
		for _, name := range slices.Sorted(maps.Keys(toDelete)) {
			ds := toDelete[name]
			if err := ih.owner.DeleteObject(ctx, ds); err != nil {
				return ComponentStatusPending("Cannot delete " + ds.Name), err
			}
		}
	}

	allReady := true
	var allReadyMessage, notReadyMessage []string

	for _, name := range slices.Sorted(maps.Keys(toApply)) {
		heater := toApply[name]
		hash := heater.NewObject().Annotations[consts.InstanceHashAnnotationName]
		ready, message := heater.Status()
		if !heater.Exists() || hash != heater.OldObject().Annotations[consts.InstanceHashAnnotationName] {
			ready = false
			message = fmt.Sprintf("daemon set %v need sync", heater.Name())
			if !dry {
				if err := heater.Sync(ctx); err != nil {
					return ComponentStatusPending("Cannot create " + heater.Name()), err
				}
			}
		}
		logger.Info("Image heater status",
			"daemonset", heater.Name(),
			"hash", hash,
			"targets", hashToNames[hash],
			"ready", ready,
			"message", message,
		)
		if ready {
			allReadyMessage = append(allReadyMessage, message)
		} else {
			notReadyMessage = append(notReadyMessage, message)
			allReady = false
		}
	}

	if !allReady {
		return ComponentStatusPending(strings.Join(notReadyMessage, "\n")), nil
	}
	return ComponentStatusReadyAfter(strings.Join(allReadyMessage, "\n")), nil
}

func (ih *ImageHeater) buildImageHeater(name string, target *ImageHeaterTarget) (*resources.DaemonSet, string, error) {
	labeller := ih.labeller.ForComponent(consts.ImageHeaterType, name)
	ds := resources.NewDaemonSet(
		labeller.GetComponentShortName(),
		labeller,
		ih.ytsaurus,
	)
	obj := ds.Build()

	metav1.SetMetaDataLabel(&obj.ObjectMeta, consts.ImageHeaterLabelName, name)
	metav1.SetMetaDataLabel(&obj.Spec.Template.ObjectMeta, consts.ImageHeaterLabelName, name)

	obj.Spec.Template.Spec = corev1.PodSpec{
		// FIXME: Maybe better to not waste cluster IP addresses.
		// HostNetwork:                   true,
		EnableServiceLinks:            ptr.To(false),
		AutomountServiceAccountToken:  ptr.To(false),
		TerminationGracePeriodSeconds: ptr.To(int64(0)),

		ImagePullSecrets: target.ImagePullSecrets,
		Tolerations:      target.Tolerations,
		NodeSelector:     target.NodeSelector,
		Affinity: &corev1.Affinity{
			NodeAffinity: target.NodeAffinity,
		},
	}

	containers := make([]corev1.Container, 0, len(target.Images))
	for _, name := range slices.Sorted(maps.Keys(target.Images)) {
		containers = append(containers, corev1.Container{
			Name:            name,
			Image:           target.Images[name],
			ImagePullPolicy: corev1.PullAlways,
			Command:         []string{"sleep", "inf"},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("0"),
					corev1.ResourceMemory: resource.MustParse("0"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("32Mi"),
				},
			},
			SecurityContext: &corev1.SecurityContext{
				Privileged:               ptr.To(false),
				AllowPrivilegeEscalation: ptr.To(false),
				RunAsNonRoot:             ptr.To(true),
				RunAsUser:                ptr.To(int64(1000)),
				RunAsGroup:               ptr.To(int64(1000)),
				ReadOnlyRootFilesystem:   ptr.To(true),
			},
		})
	}
	obj.Spec.Template.Spec.Containers = containers

	// Instance hash covers images and node selectors.
	hash, err := resources.Hash(
		obj.Spec.Template.Spec.Tolerations,
		obj.Spec.Template.Spec.NodeSelector,
		obj.Spec.Template.Spec.Affinity,
		obj.Spec.Template.Spec.ImagePullSecrets,
		obj.Spec.Template.Spec.Containers,
	)
	if err != nil {
		return nil, "", err
	}

	metav1.SetMetaDataAnnotation(&obj.ObjectMeta, consts.InstanceHashAnnotationName, hash)

	return ds, hash, nil
}
