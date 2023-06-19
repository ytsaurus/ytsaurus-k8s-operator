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
	"reflect"

	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	// Set anti affinity for masters
	if r.Spec.PrimaryMasters.Affinity == nil {
		r.Spec.PrimaryMasters.Affinity = &corev1.Affinity{}
	}
	if r.Spec.PrimaryMasters.Affinity.PodAntiAffinity == nil {
		r.Spec.PrimaryMasters.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							consts.YTComponentLabelName: fmt.Sprintf("%s-%s", r.Name, "yt-master"),
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		}
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-ytsaurus,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=ytsaurus,verbs=create;update,versions=v1,name=vytsaurus.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Ytsaurus{}

func (r *Ytsaurus) validateProxies(spec YtsaurusSpec) field.ErrorList {
	var allErrs field.ErrorList

	httpRoles := make(map[string]bool)
	rpcRoles := make(map[string]bool)
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
		if _, exists := rpcRoles[rp.Role]; exists {
			allErrs = append(allErrs, field.Duplicate(field.NewPath("spec").Child("rpcProxies").Child("role"), rp.Role))
		}
		rpcRoles[rp.Role] = true
	}

	return allErrs
}

func (r *Ytsaurus) checkInstanceSpec(value reflect.Value, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	_, ok := value.Type().FieldByName("InstanceSpec")
	if ok {
		antiAffinity := value.FieldByName("EnableAntiAffinity")
		if !antiAffinity.IsNil() {
			allErrors = append(allErrors, field.Invalid(path.Child("EnableAntiAffinity"), antiAffinity.Interface(), "EnableAntiAffinity is deprecated, use Affinity instead"))
		}
	}
	return allErrors
}

func (r *Ytsaurus) checkEnableAntiAffinity(value reflect.Value, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	if value.Kind() == reflect.Slice {
		for i := 0; i < value.Len(); i++ {
			elem := value.Index(i)
			if elem.Kind() == reflect.Ptr && !elem.IsNil() {
				elem = elem.Elem()
			}
			allErrors = append(allErrors, r.checkInstanceSpec(elem, path.Index(i))...)
		}
	} else if value.Kind() == reflect.Struct {
		allErrors = append(allErrors, r.checkInstanceSpec(value, path)...)
	}

	return allErrors
}

func (r *Ytsaurus) validateInstanceSpecs() field.ErrorList {
	var allErrors field.ErrorList
	v := reflect.ValueOf(r.Spec)

	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}

	for i := 0; i < v.NumField(); i++ {
		fieldValue := v.Field(i)
		fieldType := v.Type().Field(i)

		if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {
			fieldValue = fieldValue.Elem()
		}

		allErrors = append(allErrors, r.checkEnableAntiAffinity(fieldValue, field.NewPath(fieldType.Name))...)
	}

	return allErrors

}

func (r *Ytsaurus) validateYtsaurus() error {
	var allErrs field.ErrorList

	allErrs = append(allErrs, r.validateProxies(r.Spec)...)
	allErrs = append(allErrs, r.validateInstanceSpecs()...)
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
