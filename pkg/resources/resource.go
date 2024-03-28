package resources

import (
	"context"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Resource interface {
	OldObject() client.Object
}

func Exists(r Resource) bool {
	return r.OldObject().GetResourceVersion() != ""
}

type Fetchable interface {
	Fetch(ctx context.Context) error
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

type Syncable interface {
	Sync(ctx context.Context) error
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
