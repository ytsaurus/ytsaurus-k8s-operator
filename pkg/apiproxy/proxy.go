package apiproxy

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/version"
)

type APIProxy interface {
	Client() client.Client
	FetchObject(ctx context.Context, name string, obj client.Object) error
	ListObjects(ctx context.Context, objList client.ObjectList, opts ...client.ListOption) error
	RecordWarning(reason, message string)
	RecordNormal(reason, message string)

	// IsObjectUpdated returns true if annotation of managed object is equal to generation of owner object.
	IsObjectUpdated(obj client.Object) bool
	SyncObject(ctx context.Context, oldObj, newObj client.Object) error
	DeleteObject(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error

	UpdateStatus(ctx context.Context) error
	UpdateOperatorVersion(conditions *[]metav1.Condition) bool
}

type ConditionManager interface {
	SetStatusCondition(condition metav1.Condition)
	IsStatusConditionTrue(conditionType string) bool
	IsStatusConditionFalse(conditionType string) bool
}

func NewAPIProxy(
	object client.Object,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme) APIProxy {
	return &apiProxy{
		object:   object,
		client:   client,
		recorder: recorder,
		scheme:   scheme,
	}
}

type apiProxy struct {
	object   client.Object
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

func (c *apiProxy) RecordWarning(reason, message string) {
	c.recorder.Event(
		c.object,
		corev1.EventTypeWarning,
		reason,
		message)
}

func (c *apiProxy) RecordNormal(reason, message string) {
	c.recorder.Event(
		c.object,
		corev1.EventTypeNormal,
		reason,
		message)
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

	if err := c.updateObject(ctx, newObj); err != nil {
		if apierrors.IsNotFound(err) {
			newObj.SetResourceVersion("")
			return c.createAndReferenceObject(ctx, newObj)
		}
		return err
	}
	return nil
}

func (c *apiProxy) DeleteObject(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	logger := log.FromContext(ctx)

	if err := c.client.Delete(ctx, obj, opts...); err != nil {
		// ToDo(psushin): take to the status.
		c.RecordWarning(
			"Reconciliation",
			fmt.Sprintf("Failed to delete YT object %s: %s", obj.GetName(), err))
		logger.Error(err, "unable to delete YT object", "object_name", obj.GetName())
		return err
	}

	c.RecordNormal(
		"Reconciliation",
		fmt.Sprintf("Deleted YT object %s", obj.GetName()))
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
		c.RecordWarning(
			"Reconciliation",
			fmt.Sprintf("Failed to update YT object %s: %s", obj.GetName(), err))
		logger.Error(err, "unable to update YT object", "object_name", obj.GetName())
		return err
	}

	c.RecordNormal(
		"Reconciliation",
		fmt.Sprintf("Updated YT object %s", obj.GetName()))
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
		c.RecordWarning(
			"Reconciliation",
			fmt.Sprintf("Failed to create YT object %s: %s", obj.GetName(), err))
		logger.Error(err, "unable to create YT obj", "object_name", obj.GetName())
		return err
	}
	c.RecordNormal(
		"Reconciliation",
		fmt.Sprintf("Created YT object %s (%T)", obj.GetName(), obj))
	logger.V(2).Info("created and registered new YT object", "object_name", obj.GetName())
	return nil
}

func (c *apiProxy) UpdateStatus(ctx context.Context) error {
	return c.client.Status().Update(ctx, c.object)
}

func (c *apiProxy) UpdateOperatorVersion(conditions *[]metav1.Condition) bool {
	operatorVersion := version.GetVersion()
	condition := meta.FindStatusCondition(*conditions, consts.ConditionOperatorVersion)
	if condition != nil && condition.Message != operatorVersion {
		// Remove condition to update transition time.
		meta.RemoveStatusCondition(conditions, consts.ConditionOperatorVersion)
	}
	return meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               consts.ConditionOperatorVersion,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: c.object.GetGeneration(),
		Reason:             "Observed",
		Message:            operatorVersion,
	})
}
