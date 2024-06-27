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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	ptr "k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ytsaurus/yt-k8s-operator/pkg/consts"
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

//////////////////////////////////////////////////

func (r *Ytsaurus) validateDiscovery(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	allErrors = append(allErrors, r.validateInstanceSpec(r.Spec.Discovery.InstanceSpec, field.NewPath("spec").Child("discovery"))...)

	return allErrors
}

func (r *Ytsaurus) validatePrimaryMasters(old *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	path := field.NewPath("spec").Child("primaryMasters")
	allErrors = append(allErrors, r.validateInstanceSpec(r.Spec.PrimaryMasters.InstanceSpec, path)...)
	allErrors = append(allErrors, r.validateHostAddresses(r.Spec.PrimaryMasters, path)...)

	if FindFirstLocation(r.Spec.PrimaryMasters.Locations, LocationTypeMasterChangelogs) == nil {
		allErrors = append(allErrors, field.NotFound(path.Child("locations"), LocationTypeMasterChangelogs))
	}

	if FindFirstLocation(r.Spec.PrimaryMasters.Locations, LocationTypeMasterSnapshots) == nil {
		allErrors = append(allErrors, field.NotFound(path.Child("locations"), LocationTypeMasterSnapshots))
	}

	if old != nil && old.Spec.PrimaryMasters.CellTag != r.Spec.PrimaryMasters.CellTag {
		allErrors = append(allErrors, field.Invalid(path.Child("cellTag"), r.Spec.PrimaryMasters.CellTag, "Could not be changed"))
	}

	return allErrors
}

func (r *Ytsaurus) validateSecondaryMasters(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	for i, sm := range r.Spec.SecondaryMasters {
		path := field.NewPath("spec").Child("secondaryMasters").Index(i)
		allErrors = append(allErrors, r.validateInstanceSpec(sm.InstanceSpec, path)...)
		allErrors = append(allErrors, r.validateHostAddresses(r.Spec.PrimaryMasters, path)...)
	}

	return allErrors
}

func (r *Ytsaurus) validateHostAddresses(masterSpec MastersSpec, fieldPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	hostAddressesFieldPath := fieldPath.Child("hostAddresses")
	if !ptr.BoolDeref(masterSpec.HostNetwork, r.Spec.HostNetwork) && len(masterSpec.HostAddresses) != 0 {
		allErrors = append(
			allErrors,
			field.Required(
				field.NewPath("spec").Child("hostNetwork"),
				fmt.Sprintf("%s doesn't make sense without hostNetwork=true", hostAddressesFieldPath.String()),
			),
		)
	}

	if len(masterSpec.HostAddresses) != 0 && len(masterSpec.HostAddresses) != int(masterSpec.InstanceCount) {
		instanceCountFieldPath := fieldPath.Child("instanceCount")
		allErrors = append(
			allErrors,
			field.Invalid(
				hostAddressesFieldPath,
				masterSpec.HostAddresses,
				fmt.Sprintf("%s list length shoud be equal to %s", hostAddressesFieldPath.String(), instanceCountFieldPath.String()),
			),
		)
	}

	return allErrors
}

func (r *Ytsaurus) validateHTTPProxies(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	httpRoles := make(map[string]bool)
	hasDefaultHTTPProxy := false
	for i, hp := range r.Spec.HTTPProxies {
		path := field.NewPath("spec").Child("httpProxies").Index(i)
		if _, exists := httpRoles[hp.Role]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("role"), hp.Role))
		}
		if hp.Role == consts.DefaultHTTPProxyRole {
			hasDefaultHTTPProxy = true
		}
		httpRoles[hp.Role] = true

		allErrors = append(allErrors, r.validateInstanceSpec(hp.InstanceSpec, path)...)
	}

	if !hasDefaultHTTPProxy {
		allErrors = append(allErrors, field.Required(
			field.NewPath("spec").Child("httpProxies"),
			fmt.Sprintf("HTTP proxy with `%s` role should exist", consts.DefaultHTTPProxyRole)))
	}

	return allErrors
}

func (r *Ytsaurus) validateRPCProxies(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	rpcRoles := make(map[string]bool)
	for i, rp := range r.Spec.RPCProxies {
		path := field.NewPath("spec").Child("rpcProxies").Index(i)
		if _, exists := rpcRoles[rp.Role]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("role"), rp.Role))
		}
		rpcRoles[rp.Role] = true

		allErrors = append(allErrors, r.validateInstanceSpec(rp.InstanceSpec, path)...)
	}

	return allErrors
}

func (r *Ytsaurus) validateTCPProxies(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	tcpRoles := make(map[string]bool)
	for i, rp := range r.Spec.TCPProxies {
		path := field.NewPath("spec").Child("tcpProxies").Index(i)
		if _, exists := tcpRoles[rp.Role]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("role"), rp.Role))
		}
		tcpRoles[rp.Role] = true

		allErrors = append(allErrors, r.validateInstanceSpec(rp.InstanceSpec, path)...)
	}

	return allErrors
}

func (r *Ytsaurus) validateDataNodes(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	names := make(map[string]bool)
	for i, dn := range r.Spec.DataNodes {
		path := field.NewPath("spec").Child("dataNodes").Index(i)

		if _, exists := names[dn.Name]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("name"), dn.Name))
		}
		names[dn.Name] = true

		allErrors = append(allErrors, r.validateInstanceSpec(dn.InstanceSpec, path)...)

		if FindFirstLocation(dn.Locations, LocationTypeChunkStore) == nil {
			allErrors = append(allErrors, field.NotFound(path.Child("locations"), LocationTypeChunkStore))
		}
	}

	return allErrors
}

func validateSidecars(sidecars []string, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	names := make(map[string]bool)
	for i, sidecarSpec := range sidecars {
		sidecar := corev1.Container{}
		if err := yaml.UnmarshalStrict([]byte(sidecarSpec), &sidecar); err != nil {
			allErrors = append(allErrors, field.Invalid(path.Index(i), sidecarSpec, err.Error()))
		}
		if _, exists := names[sidecar.Name]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Index(i).Child("name"), sidecar.Name))
		}
		names[sidecar.Name] = true
	}

	return allErrors
}

func (r *Ytsaurus) validateExecNodes(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	names := make(map[string]bool)
	for i, en := range r.Spec.ExecNodes {
		path := field.NewPath("spec").Child("execNodes").Index(i)

		if _, exists := names[en.Name]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("name"), en.Name))
		}
		names[en.Name] = true

		allErrors = append(allErrors, r.validateInstanceSpec(en.InstanceSpec, path)...)

		if FindFirstLocation(en.Locations, LocationTypeChunkCache) == nil {
			allErrors = append(allErrors, field.NotFound(path.Child("locations"), LocationTypeChunkCache))
		}

		if FindFirstLocation(en.Locations, LocationTypeSlots) == nil {
			allErrors = append(allErrors, field.NotFound(path.Child("locations"), LocationTypeSlots))
		}

		if en.InitContainers != nil {
			allErrors = append(allErrors, validateSidecars(en.InitContainers, path.Child("initContainers"))...)
		}
		if en.Sidecars != nil {
			allErrors = append(allErrors, validateSidecars(en.Sidecars, path.Child("sidecars"))...)
		}
	}

	if r.Spec.ExecNodes != nil && len(r.Spec.ExecNodes) > 0 {
		if r.Spec.Schedulers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("schedulers"), "execNodes doesn't make sense without schedulers"))
		}
	}

	return allErrors
}

func (r *Ytsaurus) validateSchedulers(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if r.Spec.Schedulers != nil {
		path := field.NewPath("spec").Child("schedulers")
		allErrors = append(allErrors, r.validateInstanceSpec(r.Spec.Schedulers.InstanceSpec, path)...)

		if r.Spec.ControllerAgents == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("controllerAgents"), "schedulers doesn't make sense without controllerAgents"))
		}
	}

	return allErrors
}

func (r *Ytsaurus) validateControllerAgents(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if r.Spec.ControllerAgents != nil {
		path := field.NewPath("spec").Child("controllerAgents")
		allErrors = append(allErrors, r.validateInstanceSpec(r.Spec.ControllerAgents.InstanceSpec, path)...)

		if r.Spec.Schedulers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("schedulers"), "controllerAgents doesn't make sense without schedulers"))
		}
	}

	return allErrors
}

func (r *Ytsaurus) validateTabletNodes(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	names := make(map[string]bool)
	for i, tn := range r.Spec.TabletNodes {
		path := field.NewPath("spec").Child("tabletNodes").Index(i)

		if _, exists := names[tn.Name]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("name"), tn.Name))
		}
		names[tn.Name] = true

		allErrors = append(allErrors, r.validateInstanceSpec(tn.InstanceSpec, path)...)
	}

	return allErrors
}

func (r *Ytsaurus) validateChyt(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	return allErrors
}

func (r *Ytsaurus) validateStrawberry(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if r.Spec.StrawberryController != nil {
		if r.Spec.Schedulers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("schedulers"), "schedulers are required for strawberry"))
		}
	}

	return allErrors
}

func (r *Ytsaurus) validateQueryTrackers(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if r.Spec.QueryTrackers != nil {
		path := field.NewPath("spec").Child("queryTrackers")
		allErrors = append(allErrors, r.validateInstanceSpec(r.Spec.QueryTrackers.InstanceSpec, path)...)

		if r.Spec.TabletNodes == nil || len(r.Spec.TabletNodes) == 0 {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("tabletNodes"), "tabletNodes are required for queryTrackers"))
		}

		if r.Spec.Schedulers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("schedulers"), "schedulers are required for queryTrackers"))
		}
	}

	return allErrors
}

func (r *Ytsaurus) validateQueueAgents(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if r.Spec.QueueAgents != nil {
		path := field.NewPath("spec").Child("queueAgents")
		allErrors = append(allErrors, r.validateInstanceSpec(r.Spec.QueueAgents.InstanceSpec, path)...)

		if r.Spec.TabletNodes == nil || len(r.Spec.TabletNodes) == 0 {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("tabletNodes"), "tabletNodes are required for queueAgents"))
		}
	}

	return allErrors
}

func (r *Ytsaurus) validateSpyt(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList
	path := field.NewPath("spec").Child("spyt")

	if r.Spec.Spyt != nil {
		allErrors = append(allErrors, field.Invalid(path, r.Spec.Spyt, "spyt is deprecated here, use Spyt resource instead"))
	}

	return allErrors
}

func (r *Ytsaurus) validateYQLAgents(*Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if r.Spec.YQLAgents != nil {
		path := field.NewPath("spec").Child("YQLAgents")
		allErrors = append(allErrors, r.validateInstanceSpec(r.Spec.YQLAgents.InstanceSpec, path)...)

		if r.Spec.QueryTrackers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("queryTrackers"), "yqlAgents doesn't make sense without queryTrackers"))
		}
	}

	return allErrors
}

//////////////////////////////////////////////////

func (r *Ytsaurus) validateInstanceSpec(instanceSpec InstanceSpec, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	if instanceSpec.EnableAntiAffinity != nil {
		allErrors = append(allErrors, field.Invalid(path.Child("EnableAntiAffinity"), instanceSpec.EnableAntiAffinity, "EnableAntiAffinity is deprecated, use Affinity instead"))
	}

	if instanceSpec.Locations != nil {
		for locationIdx, location := range instanceSpec.Locations {
			inVolumeMount := false
			for _, volumeMount := range instanceSpec.VolumeMounts {
				if strings.HasPrefix(location.Path, volumeMount.MountPath) {
					inVolumeMount = true
					break
				}
			}

			if !inVolumeMount {
				allErrors = append(allErrors, field.Invalid(path.Child("locations").Index(locationIdx), location, "location path is not in any volume mount"))
			}
		}
	}

	return allErrors
}

func (r *Ytsaurus) validateYtsaurus(old *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	allErrors = append(allErrors, r.validateDiscovery(old)...)
	allErrors = append(allErrors, r.validatePrimaryMasters(old)...)
	allErrors = append(allErrors, r.validateSecondaryMasters(old)...)
	allErrors = append(allErrors, r.validateHTTPProxies(old)...)
	allErrors = append(allErrors, r.validateRPCProxies(old)...)
	allErrors = append(allErrors, r.validateTCPProxies(old)...)
	allErrors = append(allErrors, r.validateDataNodes(old)...)
	allErrors = append(allErrors, r.validateExecNodes(old)...)
	allErrors = append(allErrors, r.validateSchedulers(old)...)
	allErrors = append(allErrors, r.validateControllerAgents(old)...)
	allErrors = append(allErrors, r.validateTabletNodes(old)...)
	allErrors = append(allErrors, r.validateChyt(old)...)
	allErrors = append(allErrors, r.validateStrawberry(old)...)
	allErrors = append(allErrors, r.validateQueryTrackers(old)...)
	allErrors = append(allErrors, r.validateQueueAgents(old)...)
	allErrors = append(allErrors, r.validateSpyt(old)...)
	allErrors = append(allErrors, r.validateYQLAgents(old)...)

	return allErrors
}

func (r *Ytsaurus) evaluateYtsaurusValidation(old *Ytsaurus) error {
	allErrors := r.validateYtsaurus(old)
	if len(allErrors) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "cluster.ytsaurus.tech", Kind: "Ytsaurus"},
		r.Name,
		allErrors)
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Ytsaurus) ValidateCreate() (admission.Warnings, error) {
	ytsauruslog.Info("validate create", "name", r.Name)

	return nil, r.evaluateYtsaurusValidation(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Ytsaurus) ValidateUpdate(oldObject runtime.Object) (admission.Warnings, error) {
	ytsauruslog.Info("validate update", "name", r.Name)
	old, ok := oldObject.(*Ytsaurus)
	if !ok {
		return nil, fmt.Errorf("expected a Ytsaurus but got a %T", oldObject)
	}
	return nil, r.evaluateYtsaurusValidation(old)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Ytsaurus) ValidateDelete() (admission.Warnings, error) {
	ytsauruslog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
