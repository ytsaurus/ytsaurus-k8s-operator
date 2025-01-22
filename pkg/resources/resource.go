package resources

import (
	"context"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type ResourceObject interface {
	client.Object
}

type Fetchable interface {
	Fetch(ctx context.Context) error
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
	Exists() bool
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
	return r.proxy.FetchObject(ctx, r.name, r.oldObject)
}

func (r *BaseManagedResource[T]) Exists() bool {
	return r.oldObject.GetResourceVersion() != ""
}

func (r *BaseManagedResource[T]) Sync(ctx context.Context) error {
	return r.proxy.SyncObject(ctx, r.oldObject, r.newObject)
}

func Exists[T ResourceObject](r ManagedResource[T]) bool {
	return r.Exists()
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
