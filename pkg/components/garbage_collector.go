package components

import (
	"context"
	"fmt"
	"slices"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

// GarbageCollector removes resources whose cluster components no longer exist.
// Like other component updates, collection requires selection by the update plan.
type GarbageCollector struct {
	virtualComponent

	getAllComponents func() []Component
	orphans          []orphanResource
}

type orphanResource struct {
	client.Object
	kind string
}

func (r orphanResource) String() string {
	return r.kind + "/" + r.GetName()
}

func NewGarbageCollector(cfgen *ytconfig.Generator, ytsaurus *apiproxy.Ytsaurus, getAllComponents func() []Component) *GarbageCollector {
	l := cfgen.GetComponentLabeller(consts.GarbageCollectorType, "")
	return &GarbageCollector{
		virtualComponent: virtualComponent{component: newComponent(l, ytsaurus)},
		getAllComponents: getAllComponents,
	}
}

func (gc *GarbageCollector) Fetch(ctx context.Context) error {
	gc.orphans = nil
	owner := gc.ytsaurus.GetResource()
	if owner.UID == "" || owner.DeletionTimestamp != nil {
		return nil
	}

	expected := make(map[string]struct{})
	for _, component := range gc.getAllComponents() {
		l := component.GetLabeller()
		expected[l.GetInstanceLabelValue(false)] = struct{}{}
		expected[l.GetInstanceLabelValue(true)] = struct{}{}
	}
	// Remove workloads before their services and configuration.
	lists := []struct {
		kind string
		list client.ObjectList
	}{
		{"StatefulSet", &appsv1.StatefulSetList{}},
		{"Deployment", &appsv1.DeploymentList{}},
		{"Service", &corev1.ServiceList{}},
		{"ConfigMap", &corev1.ConfigMapList{}},
		{"Secret", &corev1.SecretList{}},
	}
	var orphans []orphanResource
	for _, entry := range lists {
		if err := gc.owner.ListObjects(ctx, entry.list,
			client.InNamespace(owner.Namespace), client.HasLabels{consts.YTComponentLabelName},
		); err != nil {
			return err
		}
		items, err := meta.ExtractList(entry.list)
		if err != nil {
			return err
		}
		var resources []orphanResource
		for _, item := range items {
			object, ok := item.(client.Object)
			if !ok {
				return fmt.Errorf("invalid %s list object: %T", entry.kind, item)
			}
			label := object.GetLabels()[consts.YTComponentLabelName]
			if label == "" || !metav1.IsControlledBy(object, owner) {
				continue
			}
			if _, exists := expected[label]; !exists {
				resources = append(resources, orphanResource{Object: object, kind: entry.kind})
			}
		}
		slices.SortFunc(resources, func(a, b orphanResource) int { return strings.Compare(a.GetName(), b.GetName()) })
		orphans = append(orphans, resources...)
	}
	gc.orphans = orphans
	return nil
}

func (gc *GarbageCollector) Exists() bool {
	return true
}

func (gc *GarbageCollector) NeedSync() bool {
	return gc.ytsaurus.IsUpdating() && IsUpdatingComponent(gc.ytsaurus, gc) &&
		gc.ytsaurus.GetUpdateState() == ytv1.UpdateStateWaitingForPodsRemoval
}

func (gc *GarbageCollector) NeedUpdate() ComponentStatus {
	if len(gc.orphans) == 0 {
		return ComponentStatusReady()
	}
	names := make([]string, 0, len(gc.orphans))
	for _, resource := range gc.orphans {
		names = append(names, resource.String())
	}
	return ComponentStatusNeedUpdate("Orphan resources: %s", strings.Join(names, ", "))
}

func (gc *GarbageCollector) Sync(ctx context.Context, dry bool) (ComponentStatus, error) {
	if !gc.ytsaurus.IsUpdating() {
		return gc.NeedUpdate(), nil
	}
	if !gc.NeedSync() {
		return ComponentStatusReady(), nil
	}

	status := ComponentStatusUpdateStep("Waiting for removal of %d orphan resources", len(gc.orphans))
	condition := gc.labeller.GetPodsRemovedCondition()
	if len(gc.orphans) != 0 && gc.ytsaurus.IsUpdateStatusConditionTrue(condition) {
		// The flow checks completion after dry sync and can advance before real
		// sync runs. Reopen removal if another orphan appeared in the meantime.
		gc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
			Type: condition, Status: metav1.ConditionFalse,
			Reason: "OrphansDetected", Message: "Orphan resources still require removal",
		})
	}
	if dry {
		return status, nil
	}
	// Wait for ongoing deletion even if a new orphan sorts before it.
	for i := range gc.orphans {
		if gc.orphans[i].GetDeletionTimestamp() != nil {
			return status, nil
		}
	}
	if len(gc.orphans) != 0 {
		object := gc.orphans[0].Object
		uid, resourceVersion := object.GetUID(), object.GetResourceVersion()
		return status, gc.owner.DeleteObject(ctx, object,
			client.PropagationPolicy(metav1.DeletePropagationBackground),
			client.Preconditions{UID: &uid, ResourceVersion: &resourceVersion},
		)
	}
	gc.ytsaurus.SetUpdateStatusCondition(ctx, metav1.Condition{
		Type: condition, Status: metav1.ConditionTrue,
		Reason: "GarbageCollected", Message: "All orphan resources have been removed",
	})
	return status, nil
}
