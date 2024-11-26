package apiproxy

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type OwningResource interface {
	client.Object
}

type TypedAPIProxy[T OwningResource] interface {
	Resource() T

	APIProxy() APIProxy

	Client() client.Client
	FetchObject(ctx context.Context, name string, obj client.Object) error
	ListObjects(ctx context.Context, objList client.ObjectList, opts ...client.ListOption) error
	RecordWarning(reason, message string)
	RecordNormal(reason, message string)
	SyncObject(ctx context.Context, oldObj, newObj client.Object) error
	DeleteObject(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error

	UpdateStatus(ctx context.Context) error
}

type APIProxy = TypedAPIProxy[OwningResource]

type ConditionManager interface {
	SetStatusCondition(condition metav1.Condition)
	IsStatusConditionTrue(conditionType string) bool
	IsStatusConditionFalse(conditionType string) bool
}

type UpdateConditionManager interface {
	SetUpdateStatusCondition(ctx context.Context, condition metav1.Condition)
	IsUpdateStatusConditionTrue(condition string) bool
}

func NewAPIProxy[T OwningResource](
	resource T,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme) apiProxy[T] {
	return apiProxy[T]{
		resource: resource,
		client:   client,
		recorder: recorder,
		scheme:   scheme,
	}
}

type apiProxy[T OwningResource] struct {
	resource T
	client   client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
}

var _ APIProxy = &apiProxy[OwningResource]{}

func (c *apiProxy[T]) Resource() T {
	return c.resource
}

func (c *apiProxy[T]) APIProxy() APIProxy {
	var r OwningResource = c.resource
	p := NewAPIProxy(r, c.client, c.recorder, c.scheme)
	return &p
}

func (c *apiProxy[T]) getObjectKey(name string) types.NamespacedName {
	return types.NamespacedName{
		Name:      name,
		Namespace: c.resource.GetNamespace(),
	}
}

func (c *apiProxy[T]) Client() client.Client {
	return c.client
}

func (c *apiProxy[T]) FetchObject(ctx context.Context, name string, obj client.Object) error {
	err := c.client.Get(ctx, c.getObjectKey(name), obj)
	if err == nil || !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (c *apiProxy[T]) ListObjects(ctx context.Context, objList client.ObjectList, opts ...client.ListOption) error {
	err := c.client.List(ctx, objList, opts...)
	return err
}

func (c *apiProxy[T]) RecordWarning(reason, message string) {
	c.recorder.Event(
		c.resource,
		corev1.EventTypeWarning,
		reason,
		message)
}

func (c *apiProxy[T]) RecordNormal(reason, message string) {
	c.recorder.Event(
		c.resource,
		corev1.EventTypeNormal,
		reason,
		message)
}

func (c *apiProxy[T]) SyncObject(ctx context.Context, oldObj, newObj client.Object) error {
	var err error
	if newObj.GetName() == "" {
		return fmt.Errorf("cannot sync uninitialized object, object type %T", oldObj)
	}
	if oldObj.GetResourceVersion() == "" {
		err = c.createAndReferenceObject(ctx, newObj)
	} else {
		newObj.SetResourceVersion(oldObj.GetResourceVersion())
		err = c.updateObject(ctx, newObj)
	}

	return err
}

func (c *apiProxy[T]) DeleteObject(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
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

func (c *apiProxy[T]) updateObject(ctx context.Context, obj client.Object) error {
	logger := log.FromContext(ctx)

	if err := ctrl.SetControllerReference(c.resource, obj, c.scheme); err != nil {
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

func (c *apiProxy[T]) createAndReferenceObject(ctx context.Context, obj client.Object) error {
	logger := log.FromContext(ctx)

	if err := ctrl.SetControllerReference(c.resource, obj, c.scheme); err != nil {
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

func (c *apiProxy[T]) UpdateStatus(ctx context.Context) error {
	return c.client.Status().Update(ctx, c.resource)
}
