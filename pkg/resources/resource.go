package resources

import (
	"context"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type ResourceObject interface {
	client.Object
}

type Fetchable interface {
	Fetch(ctx context.Context) error
	Exists() bool
}

type Syncable interface {
	Sync(ctx context.Context) error
}

type ManagedResource[T ResourceObject] interface {
	Fetchable
	Syncable

	Name() string
	OldObject() T
	NewObject() T
	Build() T
}

type BaseManagedResource[T ResourceObject] struct {
	proxy    apiproxy.APIProxy
	labeller *labeller.Labeller

	name      string
	oldObject T
	newObject T
}

func (r *BaseManagedResource[T]) OldObject() T {
	return r.oldObject
}

func (r *BaseManagedResource[T]) NewObject() T {
	return r.newObject
}

func (r *BaseManagedResource[T]) Name() string {
	return r.name
}

func (r *BaseManagedResource[T]) Fetch(ctx context.Context) error {
	r.oldObject.SetResourceVersion("")
	return r.proxy.FetchObject(ctx, r.name, r.oldObject)
}

func (r *BaseManagedResource[T]) Exists() bool {
	return r.oldObject.GetResourceVersion() != ""
}

func (r *BaseManagedResource[T]) IsAnnotationChanged(ann string) bool {
	oldVal, oldSet := r.oldObject.GetAnnotations()[ann]
	newVal, newSet := r.newObject.GetAnnotations()[ann]
	return oldSet != newSet || oldVal != newVal
}

func (r *BaseManagedResource[T]) IsUpdated() bool {
	return r.proxy.IsObjectUpdated(r.oldObject)
}

func (r *BaseManagedResource[T]) Sync(ctx context.Context) error {
	return r.proxy.SyncObject(ctx, r.oldObject, r.newObject)
}

func (r *BaseManagedResource[T]) DeleteForeground(ctx context.Context) error {
	return r.proxy.DeleteObject(ctx, r.oldObject, client.PropagationPolicy(metav1.DeletePropagationForeground))
}

func Fetch(ctx context.Context, objects ...Fetchable) error {
	for _, obj := range objects {
		if obj == nil || reflect.ValueOf(obj).IsNil() {
			continue
		}
		err := obj.Fetch(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func Exists(objects ...Fetchable) bool {
	for _, obj := range objects {
		if obj == nil || reflect.ValueOf(obj).IsNil() {
			continue
		}
		if !obj.Exists() {
			return false
		}
	}
	return true
}

func Sync(ctx context.Context, objects ...Syncable) error {
	for _, obj := range objects {
		if obj == nil || reflect.ValueOf(obj).IsNil() {
			continue
		}
		err := obj.Sync(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
