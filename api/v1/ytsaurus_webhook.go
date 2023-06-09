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

package v1

import (
	"fmt"
	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var ytsauruslog = logf.Log.WithName("ytsaurus-resource")

func (r *Ytsaurus) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-cluster-ytsaurus-tech-v1-ytsaurus,mutating=true,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=ytsaurus,verbs=create;update,versions=v1,name=mytsaurus.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Ytsaurus{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Ytsaurus) Default() {
	ytsauruslog.Info("default", "name", r.Name)

	if r.Spec.PrimaryMasters.EnableAntiAffinity == nil {
		r.Spec.PrimaryMasters.EnableAntiAffinity = new(bool)
		*r.Spec.PrimaryMasters.EnableAntiAffinity = true
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-ytsaurus,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=ytsaurus,verbs=create;update,versions=v1,name=vytsaurus.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Ytsaurus{}

func (r *Ytsaurus) validateProxies(spec YtsaurusSpec) field.ErrorList {
	var allErrs field.ErrorList

	var httpRoles map[string]bool
	var rpcRoles map[string]bool
	hasDefaultHTTPProxy := false
	for _, hp := range spec.HTTPProxies {
		if _, exists := httpRoles[hp.Role]; exists {
			allErrs = append(allErrs, field.Duplicate(field.NewPath("spec").Child("httpProxies").Child("role"), hp.Role))
		}
		if hp.Role == consts.DefaultHTTPProxyRole {
			hasDefaultHTTPProxy = true
		}
		httpRoles[hp.Role] = true
	}

	if !hasDefaultHTTPProxy {
		allErrs = append(allErrs, field.Required(
			field.NewPath("spec").Child("httpProxies").Child("role"),
			fmt.Sprintf("HTTP proxy with `%s` role should exist", consts.DefaultHTTPProxyRole)))
	}

	for _, rp := range spec.RPCProxies {
		if _, exists := httpRoles[rp.Role]; exists {
			allErrs = append(allErrs, field.Duplicate(field.NewPath("spec").Child("rpcProxies").Child("role"), rp.Role))
		}
		rpcRoles[rp.Role] = true
	}

	return allErrs
}

func (r *Ytsaurus) validateYtsaurus() error {
	var allErrs field.ErrorList

	allErrs = append(allErrs, r.validateProxies(r.Spec)...)
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "cluster.ytsaurus.tech", Kind: "Ytsaurus"},
		r.Name,
		allErrs)
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Ytsaurus) ValidateCreate() error {
	ytsauruslog.Info("validate create", "name", r.Name)

	return r.validateYtsaurus()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Ytsaurus) ValidateUpdate(old runtime.Object) error {
	ytsauruslog.Info("validate update", "name", r.Name)

	return r.validateYtsaurus()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Ytsaurus) ValidateDelete() error {
	ytsauruslog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
