package apiproxy

import (
	"context"
	"fmt"

	ytv1 "github.com/YTsaurus/yt-k8s-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type APIProxy struct {
	ytsaurus *ytv1.Ytsaurus
	client   client.Client
	recorder record.EventRecorder
	scheme   *runtime.Scheme
}

func NewAPIProxy(
	ytsaurus *ytv1.Ytsaurus,
	client client.Client,
	recorder record.EventRecorder,
	scheme *runtime.Scheme) *APIProxy {
	return &APIProxy{
		ytsaurus: ytsaurus,
		client:   client,
		recorder: recorder,
		scheme:   scheme}
}

func (c *APIProxy) GetObjectKey(name string) types.NamespacedName {
	return types.NamespacedName{
		Name:      name,
		Namespace: c.ytsaurus.Namespace,
	}
}

func (c *APIProxy) Ytsaurus() *ytv1.Ytsaurus {
	return c.ytsaurus
}

func (c *APIProxy) FetchObject(ctx context.Context, name string, obj client.Object) error {
	err := c.client.Get(ctx, c.GetObjectKey(name), obj)
	if err == nil || !errors.IsNotFound(err) {
		return err
	}

	return nil
}

func (c *APIProxy) ListObjects(ctx context.Context, objList client.ObjectList, opts ...client.ListOption) error {
	err := c.client.List(ctx, objList, opts...)
	return err
}

func (c *APIProxy) RecordWarning(reason, message string) {
	c.recorder.Event(
		c.ytsaurus,
		corev1.EventTypeWarning,
		reason,
		message)
}

func (c *APIProxy) RecordNormal(reason, message string) {
	c.recorder.Event(
		c.ytsaurus,
		corev1.EventTypeNormal,
		reason,
		message)
}

func (c *APIProxy) IsStatusConditionTrue(condition string) bool {
	return meta.IsStatusConditionTrue(c.ytsaurus.Status.Conditions, condition)
}

func (c *APIProxy) SetStatusCondition(ctx context.Context, condition metav1.Condition) error {
	meta.SetStatusCondition(&c.ytsaurus.Status.Conditions, condition)
	return c.UpdateStatus(ctx)
}

func (c *APIProxy) SyncObject(ctx context.Context, oldObj, newObj client.Object) error {
	var err error
	if newObj.GetName() == "" {
		return fmt.Errorf("Cannot sync uninitialized object, object type %T", oldObj)
	}
	if oldObj.GetResourceVersion() == "" {
		err = c.createAndReferenceObject(ctx, newObj)
	} else {
		newObj.SetResourceVersion(oldObj.GetResourceVersion())
		err = c.updateObject(ctx, newObj)
	}

	return err
}

func (c *APIProxy) updateObject(ctx context.Context, obj client.Object) error {
	logger := log.FromContext(ctx)

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

func (c *APIProxy) createAndReferenceObject(ctx context.Context, obj client.Object) error {
	logger := log.FromContext(ctx)

	if err := ctrl.SetControllerReference(c.ytsaurus, obj, c.scheme); err != nil {
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

func (c *APIProxy) UpdateStatus(ctx context.Context) error {
	return c.client.Status().Update(ctx, c.ytsaurus)
}
