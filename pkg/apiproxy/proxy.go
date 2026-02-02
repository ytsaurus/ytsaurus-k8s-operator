package apiproxy

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/version"
)

type ControllerObject interface {
	client.Object

	GetStatusObservedGeneration() int64
	SetStatusObservedGeneration(generation int64)
	GetStatusConditions() []metav1.Condition
	SetStatusConditions(conditions []metav1.Condition)
}

type APIProxy interface {
	Client() client.Client

	FetchObject(ctx context.Context, name string, obj client.Object) error
	ListObjects(ctx context.Context, objList client.ObjectList, opts ...client.ListOption) error

	RecordWarning(reason, messageFmt string, args ...any)
	RecordNormal(reason, messageFmt string, args ...any)
	RecordAction(action string, subject client.Object)

	// IsObjectUpdated returns true if annotation of managed object is equal to generation of owner object.
	IsObjectUpdated(obj client.Object) bool

	SyncObject(ctx context.Context, oldObj, newObj client.Object) error
	DeleteObject(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error

	// SyncObservedGeneration confirms that current generation was observed.
	// Returns true if generation actually has been changed and status must be saved.
	SyncObservedGeneration() bool

	// SetStatusCondition also updates its own observed generation.
	SetStatusCondition(condition metav1.Condition)

	RemoveStatusCondition(conditionType string)

	GetStatusCondition(conditionType string) *metav1.Condition
	IsStatusConditionTrue(conditionType string) bool
	IsStatusConditionFalse(conditionType string) bool

	// Returns true if condition has met and controller object is not changed since then.
	IsStatusConditionTrueAndObservedGeneration(conditionType string) bool

	UpdateStatus(ctx context.Context) error
}

func NewAPIProxy(
	object ControllerObject,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme,
) APIProxy {
	return &apiProxy{
		object:   object,
		kind:     object.GetObjectKind().GroupVersionKind().Kind,
		client:   client,
		recorder: recorder,
		scheme:   scheme,
	}
}

type apiProxy struct {
	object   ControllerObject
	kind     string
	client   client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
}

func (c *apiProxy) getObjectKey(name string) types.NamespacedName {
	return types.NamespacedName{
		Name:      name,
		Namespace: c.object.GetNamespace(),
	}
}

func (c *apiProxy) Client() client.Client {
	return c.client
}

func (c *apiProxy) FetchObject(ctx context.Context, name string, obj client.Object) error {
	err := c.client.Get(ctx, c.getObjectKey(name), obj)
	if err == nil || !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (c *apiProxy) ListObjects(ctx context.Context, objList client.ObjectList, opts ...client.ListOption) error {
	err := c.client.List(ctx, objList, opts...)
	return err
}

func (c *apiProxy) RecordWarning(reason, messageFmt string, args ...any) {
	c.recorder.Eventf(c.object, corev1.EventTypeWarning, reason, messageFmt, args...)
}

func (c *apiProxy) RecordNormal(reason, messageFmt string, args ...any) {
	c.recorder.Eventf(c.object, corev1.EventTypeNormal, reason, messageFmt, args...)
}

func (c *apiProxy) RecordAction(reason string, subject client.Object) {
	gvk, err := apiutil.GVKForObject(subject, c.scheme)
	if err != nil {
		gvk.Kind = fmt.Sprintf("%T", subject)
	}
	c.recorder.Eventf(subject, corev1.EventTypeNormal, reason, "%s by %s %s", reason, c.kind, c.object.GetName())
	c.recorder.Eventf(c.object, corev1.EventTypeNormal, reason, "%s object %s %s", reason, gvk.Kind, subject.GetName())
}

func (c *apiProxy) IsObjectUpdated(obj client.Object) bool {
	return obj.GetAnnotations()[consts.ObservedGenerationAnnotationName] == fmt.Sprintf("%d", c.object.GetGeneration())
}

func (c *apiProxy) SyncObject(ctx context.Context, oldObj, newObj client.Object) error {
	if newObj.GetName() == "" {
		return fmt.Errorf("cannot sync uninitialized object, object type %T", oldObj)
	}

	{
		annotations := newObj.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string, 1)
		}
		annotations[consts.ObservedGenerationAnnotationName] = fmt.Sprintf("%d", c.object.GetGeneration())
		newObj.SetAnnotations(annotations)
	}

	if oldObj.GetResourceVersion() == "" {
		return c.createAndReferenceObject(ctx, newObj)
	}

	newObj.SetResourceVersion(oldObj.GetResourceVersion())

	// Preserve finalizers, for example "foregroundDeletion".
	newObj.SetFinalizers(oldObj.GetFinalizers())

	return c.updateObject(ctx, newObj)
}

func (c *apiProxy) DeleteObject(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	logger := log.FromContext(ctx)

	if err := c.client.Delete(ctx, obj, opts...); err != nil {
		// ToDo(psushin): take to the status.
		c.RecordWarning("Failure", "Failed to delete YT object %s: %s", obj.GetName(), err)
		logger.Error(err, "unable to delete YT object", "object_name", obj.GetName())
		return err
	}

	c.RecordAction("Delete", obj)
	logger.V(2).Info("deleted YT object", "object_name", obj.GetName())
	return nil
}

func (c *apiProxy) updateObject(ctx context.Context, obj client.Object) error {
	logger := log.FromContext(ctx)

	if err := ctrl.SetControllerReference(c.object, obj, c.scheme); err != nil {
		logger.Error(err, "unable to set controller reference", "object_name", obj.GetName())
		return err
	}

	if err := c.client.Update(ctx, obj); err != nil {
		// ToDo(psushin): take to the status.
		c.RecordWarning("Failure", "Failed to update YT object %s: %s", obj.GetName(), err)
		logger.Error(err, "unable to update YT object", "object_name", obj.GetName())
		return err
	}

	c.RecordAction("Update", obj)
	logger.V(2).Info("updated existing YT object", "object_name", obj.GetName())
	return nil
}

func (c *apiProxy) createAndReferenceObject(ctx context.Context, obj client.Object) error {
	logger := log.FromContext(ctx)

	if err := ctrl.SetControllerReference(c.object, obj, c.scheme); err != nil {
		logger.Error(err, "unable to set controller reference", "object_name", obj.GetName())
		return err
	}

	if err := c.client.Create(ctx, obj); err != nil {
		// ToDo(psushin): take to the status.
		c.RecordWarning("Failure", "Failed to create YT object %s: %s", obj.GetName(), err)
		logger.Error(err, "unable to create YT obj", "object_name", obj.GetName())
		return err
	}

	c.RecordAction("Create", obj)
	logger.V(2).Info("created and registered new YT object", "object_name", obj.GetName())
	return nil
}

func (c *apiProxy) UpdateStatus(ctx context.Context) error {
	c.updateOperatorVersion()
	return c.client.Status().Update(ctx, c.object)
}

func sortConditions(conditions []metav1.Condition) {
	slices.SortStableFunc(conditions, func(a, b metav1.Condition) int {
		statusOrder := []metav1.ConditionStatus{metav1.ConditionTrue, metav1.ConditionFalse, metav1.ConditionUnknown}
		if diff := cmp.Compare(slices.Index(statusOrder, a.Status), slices.Index(statusOrder, b.Status)); diff != 0 {
			return diff
		}
		return a.LastTransitionTime.Compare(b.LastTransitionTime.Time)
	})
}

func (c *apiProxy) SetStatusCondition(condition metav1.Condition) {
	condition.ObservedGeneration = c.object.GetGeneration()
	conditions := c.object.GetStatusConditions()
	meta.SetStatusCondition(&conditions, condition)
	sortConditions(conditions)
	c.object.SetStatusConditions(conditions)
}

func (c *apiProxy) RemoveStatusCondition(conditionType string) {
	conditions := c.object.GetStatusConditions()
	meta.RemoveStatusCondition(&conditions, conditionType)
	c.object.SetStatusConditions(conditions)
}

func (c *apiProxy) GetStatusCondition(conditionType string) *metav1.Condition {
	return meta.FindStatusCondition(c.object.GetStatusConditions(), conditionType)
}

func (c *apiProxy) IsStatusConditionTrue(conditionType string) bool {
	return meta.IsStatusConditionTrue(c.object.GetStatusConditions(), conditionType)
}

func (c *apiProxy) IsStatusConditionFalse(conditionType string) bool {
	return meta.IsStatusConditionFalse(c.object.GetStatusConditions(), conditionType)
}

func (c *apiProxy) IsStatusConditionTrueAndObservedGeneration(conditionType string) bool {
	cond := c.GetStatusCondition(conditionType)
	return cond != nil && cond.Status == metav1.ConditionTrue && cond.ObservedGeneration == c.object.GetGeneration()
}

func (c *apiProxy) updateOperatorVersion() bool {
	operatorVersion := version.GetVersion()
	conditions := c.object.GetStatusConditions()
	condition := meta.FindStatusCondition(conditions, consts.ConditionOperatorVersion)
	if condition != nil && condition.Message != operatorVersion {
		// Remove condition to update transition time.
		meta.RemoveStatusCondition(&conditions, consts.ConditionOperatorVersion)
	}
	changed := meta.SetStatusCondition(&conditions, metav1.Condition{
		Type:               consts.ConditionOperatorVersion,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: c.object.GetGeneration(),
		Reason:             "Observed",
		Message:            operatorVersion,
	})
	c.object.SetStatusConditions(conditions)
	return changed
}

func (c *apiProxy) SyncObservedGeneration() bool {
	updated := false
	if generation := c.object.GetGeneration(); c.object.GetStatusObservedGeneration() != generation {
		c.object.SetStatusObservedGeneration(generation)
		updated = true
	}
	if c.updateOperatorVersion() {
		updated = true
	}
	return updated
}
