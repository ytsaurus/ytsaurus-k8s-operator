/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validators

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/go-logr/logr"
)

type Results struct {
	Errors   field.ErrorList
	Warnings field.ErrorList
}

func (r *Results) Append(results Results) {
	r.Errors = append(r.Errors, results.Errors...)
	r.Warnings = append(r.Warnings, results.Warnings...)
}

func (r *Results) Error(err *field.Error) {
	r.Errors = append(r.Errors, err)
}

func (r *Results) AppendErrors(errors field.ErrorList) {
	r.Errors = append(r.Errors, errors...)
}

func (r *Results) Warning(err *field.Error) {
	r.Warnings = append(r.Warnings, err)
}

// customValidator implements admission.CustomValidator
type customValidator[T client.Object] struct {
	Log       logr.Logger
	Client    client.Client
	Object    T
	nilObject T
	ObjectGVK schema.GroupVersionKind
	Validate  func(ctx context.Context, newObj, oldObj T) Results
}

func (r *customValidator[T]) SetupWebhookWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()

	object := r.Object
	r.Object = r.nilObject
	if gvk, err := r.Client.GroupVersionKindFor(object); err != nil {
		return err
	} else {
		r.ObjectGVK = gvk
	}

	r.Log = logf.Log.WithName(r.ObjectGVK.Kind + "-validator")
	return ctrl.NewWebhookManagedBy(mgr).
		For(object).
		WithValidator(r).
		Complete()
}

func (r *customValidator[T]) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	newObject, ok := obj.(T)
	if !ok {
		return nil, fmt.Errorf("expected a %v but got a %T", r.ObjectGVK, obj)
	}
	r.Log.Info("validate create", "name", newObject.GetName())
	return r.evaluateValidation(ctx, newObject, r.nilObject, newObject.GetName())
}

func (r *customValidator[T]) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldObject, ok := oldObj.(T)
	if !ok {
		return nil, fmt.Errorf("expected a %v but got a %T", r.ObjectGVK, oldObj)
	}
	newObject, ok := newObj.(T)
	if !ok {
		return nil, fmt.Errorf("expected a %v but got a %T", r.ObjectGVK, newObj)
	}
	r.Log.Info("validate update", "name", oldObject.GetName())
	return r.evaluateValidation(ctx, newObject, oldObject, newObject.GetName())
}

func (r *customValidator[T]) ValidateDelete(ctx context.Context, oldObj runtime.Object) (admission.Warnings, error) {
	oldObject, ok := oldObj.(T)
	if !ok {
		return nil, fmt.Errorf("expected a %v but got a %T", r.ObjectGVK, oldObj)
	}
	r.Log.Info("validate delete", "name", oldObject.GetName())
	return r.evaluateValidation(ctx, r.nilObject, oldObject, oldObject.GetName())
}

func (r *customValidator[T]) evaluateValidation(ctx context.Context, newObj, oldObj T, name string) (admission.Warnings, error) {
	results := r.Validate(ctx, newObj, oldObj)
	var warnings admission.Warnings
	for _, warning := range results.Warnings {
		warnings = append(warnings, warning.Error())
	}
	if len(results.Errors) == 0 {
		return warnings, nil
	}
	return warnings, apierrors.NewInvalid(r.ObjectGVK.GroupKind(), name, results.Errors)
}
