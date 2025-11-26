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
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	v1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

// log is for logging in this package.
var ytsauruslog = logf.Log.WithName("ytsaurus-resource")

type baseValidator struct {
	Client client.Client
}

type ytsaurusValidator struct {
	baseValidator
}

func (r *Ytsaurus) SetupWebhookWithManager(mgr ctrl.Manager) error {
	validator := &ytsaurusValidator{
		baseValidator: baseValidator{
			Client: mgr.GetClient(),
		},
	}
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(validator).
		Complete()
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-ytsaurus,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=ytsaurus,verbs=create;update,versions=v1,name=vytsaurus.kb.io,admissionReviewVersions=v1

//////////////////////////////////////////////////

func (r *ytsaurusValidator) validateDiscovery(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.Discovery.InstanceSpec, field.NewPath("spec").Child("discovery"))...)

	return allErrors
}

func (r *ytsaurusValidator) validateMasterSpec(newYtsaurus *Ytsaurus, mastersSpec, oldMastersSpec *MastersSpec, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	allErrors = append(allErrors, r.validateInstanceSpec(mastersSpec.InstanceSpec, path)...)
	allErrors = append(allErrors, r.validateHostAddresses(newYtsaurus, mastersSpec, path)...)

	if FindFirstLocation(mastersSpec.Locations, LocationTypeMasterChangelogs) == nil {
		allErrors = append(allErrors, field.NotFound(path.Child("locations"), LocationTypeMasterChangelogs))
	}

	if FindFirstLocation(mastersSpec.Locations, LocationTypeMasterSnapshots) == nil {
		allErrors = append(allErrors, field.NotFound(path.Child("locations"), LocationTypeMasterSnapshots))
	}

	if oldMastersSpec != nil && oldMastersSpec.CellTag != mastersSpec.CellTag {
		allErrors = append(allErrors, field.Invalid(path.Child("cellTag"), mastersSpec.CellTag, "Could not be changed"))
	}

	if mastersSpec.InstanceCount > 1 && !newYtsaurus.Spec.EphemeralCluster {
		affinity := newYtsaurus.Spec.PrimaryMasters.Affinity
		if affinity == nil || affinity.PodAntiAffinity == nil {
			allErrors = append(allErrors, field.Required(path.Child("affinity").Child("podAntiAffinity"),
				"Masters should be placed on different nodes"))
		}
	}

	allErrors = append(allErrors, r.validateHydraPersistenceUploaderSpec(mastersSpec.HydraPersistenceUploader, path.Child("hydraPersistenceUploader"))...)

	allErrors = append(allErrors, r.validateTimbertruckSpec(mastersSpec.Timbertruck, mastersSpec.StructuredLoggers, mastersSpec.Locations, path)...)

	allErrors = append(allErrors, r.validateSidecars(mastersSpec.Sidecars, path.Child("sidecars"))...)

	return allErrors
}

func (r *ytsaurusValidator) validatePrimaryMasters(newYtsaurus, oldYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	mastersSpec := &newYtsaurus.Spec.PrimaryMasters
	path := field.NewPath("spec").Child("primaryMasters")

	var oldMastersSpec *MastersSpec
	if oldYtsaurus != nil {
		oldMastersSpec = &oldYtsaurus.Spec.PrimaryMasters
	}

	allErrors = append(allErrors, r.validateMasterSpec(newYtsaurus, mastersSpec, oldMastersSpec, path)...)

	return allErrors
}

func (r *ytsaurusValidator) validateSecondaryMasters(newYtsaurus, oldYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	for i := range newYtsaurus.Spec.SecondaryMasters {
		path := field.NewPath("spec").Child("secondaryMasters").Index(i)
		mastersSpec := &newYtsaurus.Spec.SecondaryMasters[i]
		var oldMastersSpec *MastersSpec
		if oldYtsaurus != nil && len(oldYtsaurus.Spec.SecondaryMasters) > i {
			oldMastersSpec = &oldYtsaurus.Spec.SecondaryMasters[i]
		}
		allErrors = append(allErrors, r.validateMasterSpec(newYtsaurus, mastersSpec, oldMastersSpec, path)...)
	}

	return allErrors
}

func (r *ytsaurusValidator) validateHostAddresses(newYtsaurus *Ytsaurus, mastersSpec *MastersSpec, fieldPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	hostAddressesFieldPath := fieldPath.Child("hostAddresses")
	if !ptr.Deref(mastersSpec.HostNetwork, newYtsaurus.Spec.HostNetwork) && len(mastersSpec.HostAddresses) != 0 {
		allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("hostNetwork"),
			fmt.Sprintf("%s doesn't make sense without hostNetwork=true", hostAddressesFieldPath.String())))
	}

	if len(mastersSpec.HostAddresses) != 0 && len(mastersSpec.HostAddresses) != int(mastersSpec.InstanceCount) {
		instanceCountFieldPath := fieldPath.Child("instanceCount")
		allErrors = append(allErrors, field.Invalid(hostAddressesFieldPath, newYtsaurus.Spec.PrimaryMasters.HostAddresses,
			fmt.Sprintf("%s list length shoud be equal to %s", hostAddressesFieldPath.String(), instanceCountFieldPath.String())))
	}

	return allErrors
}

func (r *ytsaurusValidator) validateHTTPProxies(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	httpRoles := make(map[string]bool)
	hasDefaultHTTPProxy := false
	for i, hp := range newYtsaurus.Spec.HTTPProxies {
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

func (r *ytsaurusValidator) validateRPCProxies(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	rpcRoles := make(map[string]bool)
	for i, rp := range newYtsaurus.Spec.RPCProxies {
		path := field.NewPath("spec").Child("rpcProxies").Index(i)
		if _, exists := rpcRoles[rp.Role]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("role"), rp.Role))
		}
		rpcRoles[rp.Role] = true

		allErrors = append(allErrors, r.validateInstanceSpec(rp.InstanceSpec, path)...)
	}

	return allErrors
}

func (r *ytsaurusValidator) validateTCPProxies(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	tcpRoles := make(map[string]bool)
	for i, rp := range newYtsaurus.Spec.TCPProxies {
		path := field.NewPath("spec").Child("tcpProxies").Index(i)
		if _, exists := tcpRoles[rp.Role]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("role"), rp.Role))
		}
		tcpRoles[rp.Role] = true

		allErrors = append(allErrors, r.validateInstanceSpec(rp.InstanceSpec, path)...)
	}

	return allErrors
}

func (r *ytsaurusValidator) validateDataNodes(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	names := make(map[string]bool)
	for i, dn := range newYtsaurus.Spec.DataNodes {
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

func (r *baseValidator) validateHydraPersistenceUploaderSpec(
	hydraPersistenceUploader *HydraPersistenceUploaderSpec, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	if hydraPersistenceUploader != nil && hydraPersistenceUploader.Image == nil {
		allErrors = append(allErrors, field.Required(path.Child("image"), "hydraPersistenceUploader image is required"))
	}

	return allErrors
}

func (r *baseValidator) validateTimbertruckSpec(
	timbertruck *TimbertruckSpec, structuredLoggers []StructuredLoggerSpec, locations []LocationSpec, parentPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	if timbertruck != nil {
		if timbertruck.Image == nil {
			allErrors = append(allErrors, field.Required(parentPath.Child("timbertruck", "image"), "timbertruck image is required"))
		}
		if len(structuredLoggers) == 0 {
			allErrors = append(allErrors, field.Required(parentPath.Child("structuredLoggers"), "structuredLoggers must be configured when timbertruck is enabled"))
		}
		if logLocation := FindFirstLocation(locations, LocationTypeLogs); logLocation == nil {
			allErrors = append(allErrors, field.Required(parentPath.Child("locations"), "logs location must be configured when timbertruck is enabled"))
		}
	}
	return allErrors
}

func (r *baseValidator) validateSidecars(sidecars []string, path *field.Path) field.ErrorList {
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

func (r *ytsaurusValidator) validateExecNodes(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	names := make(map[string]bool)
	for i, en := range newYtsaurus.Spec.ExecNodes {
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
			allErrors = append(allErrors, r.validateSidecars(en.InitContainers, path.Child("initContainers"))...)
		}
		if en.Sidecars != nil {
			allErrors = append(allErrors, r.validateSidecars(en.Sidecars, path.Child("sidecars"))...)
		}
	}

	if len(newYtsaurus.Spec.ExecNodes) > 0 {
		if newYtsaurus.Spec.Schedulers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("schedulers"),
				"execNodes doesn't make sense without schedulers"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateSchedulers(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.Schedulers != nil {
		path := field.NewPath("spec").Child("schedulers")
		allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.Schedulers.InstanceSpec, path)...)

		if newYtsaurus.Spec.ControllerAgents == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("controllerAgents"),
				"schedulers doesn't make sense without controllerAgents"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateControllerAgents(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.ControllerAgents != nil {
		path := field.NewPath("spec").Child("controllerAgents")
		allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.ControllerAgents.InstanceSpec, path)...)

		if newYtsaurus.Spec.Schedulers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("schedulers"),
				"controllerAgents doesn't make sense without schedulers"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateTabletNodes(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	names := make(map[string]bool)
	for i, tn := range newYtsaurus.Spec.TabletNodes {
		path := field.NewPath("spec").Child("tabletNodes").Index(i)

		if _, exists := names[tn.Name]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("name"), tn.Name))
		}
		names[tn.Name] = true

		allErrors = append(allErrors, r.validateInstanceSpec(tn.InstanceSpec, path)...)
	}

	return allErrors
}

func (r *ytsaurusValidator) validateChyt(_ *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	return allErrors
}

func (r *ytsaurusValidator) validateStrawberry(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.StrawberryController != nil {
		if newYtsaurus.Spec.Schedulers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("schedulers"),
				"schedulers are required for strawberry"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateQueryTrackers(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.QueryTrackers != nil {
		path := field.NewPath("spec").Child("queryTrackers")
		allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.QueryTrackers.InstanceSpec, path)...)

		if len(newYtsaurus.Spec.TabletNodes) == 0 {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("tabletNodes"),
				"tabletNodes are required for queryTrackers"))
		}

		if newYtsaurus.Spec.Schedulers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("schedulers"),
				"schedulers are required for queryTrackers"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateQueueAgents(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.QueueAgents != nil {
		path := field.NewPath("spec").Child("queueAgents")
		allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.QueueAgents.InstanceSpec, path)...)

		if len(newYtsaurus.Spec.TabletNodes) == 0 {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("tabletNodes"),
				"tabletNodes are required for queueAgents"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateSpyt(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList
	path := field.NewPath("spec").Child("spyt")

	if newYtsaurus.Spec.Spyt != nil {
		allErrors = append(allErrors, field.Invalid(path, newYtsaurus.Spec.Spyt,
			"spyt is deprecated here, use Spyt resource instead"))
	}

	return allErrors
}

func (r *ytsaurusValidator) validateYQLAgents(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.YQLAgents != nil {
		path := field.NewPath("spec").Child("YQLAgents")
		allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.YQLAgents.InstanceSpec, path)...)

		if newYtsaurus.Spec.QueryTrackers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("queryTrackers"),
				"yqlAgents doesn't make sense without queryTrackers"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateUi(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.UI != nil && newYtsaurus.Spec.UI.Secure {
		for i, hp := range newYtsaurus.Spec.HTTPProxies {
			if hp.Role != consts.DefaultHTTPProxyRole {
				continue
			}
			if hp.Transport.HTTPSSecret == nil {
				allErrors = append(allErrors, field.Required(
					field.NewPath("spec", "httpProxies").Index(i).Child("transport", "httpsSecret"),
					fmt.Sprintf("configured HTTPS for proxy with `%s` role is required for ui.secure", consts.DefaultHTTPProxyRole)))
			}
			break
		}
	}

	return allErrors
}

func (r *baseValidator) validateInstanceSpec(instanceSpec InstanceSpec, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	allErrors = append(allErrors, v1validation.ValidateLabels(instanceSpec.PodLabels, path.Child("podLabels"))...)
	allErrors = append(allErrors, validation.ValidateAnnotations(instanceSpec.PodAnnotations, path.Child("podAnnotations"))...)

	if instanceSpec.EnableAntiAffinity != nil {
		allErrors = append(allErrors, field.Invalid(path.Child("EnableAntiAffinity"), instanceSpec.EnableAntiAffinity,
			"EnableAntiAffinity is deprecated, use Affinity instead"))
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
				allErrors = append(allErrors, field.Invalid(path.Child("locations").Index(locationIdx), location,
					"location path is not in any volume mount"))
			}
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateExistsYtsaurus(ctx context.Context, newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	var ytsaurusList YtsaurusList
	err := r.Client.List(ctx, &ytsaurusList, &client.ListOptions{Namespace: newYtsaurus.Namespace})
	ytsauruslog.Info("validateExistsYTsaurus", "ytsaurusList", ytsaurusList)

	if err != nil && !apierrors.IsNotFound(err) {
		allErrors = append(allErrors, field.InternalError(field.NewPath("k8sClient"), err))
		return allErrors
	}

	// if ytsaurus is already exists and it's the same one, it is an update operation
	if len(ytsaurusList.Items) == 1 && ytsaurusList.Items[0].Name == newYtsaurus.Name {
		return allErrors
	} else if len(ytsaurusList.Items) == 0 {
		// it's the creation operation for the first ytsaurus object
		return allErrors
	} else {
		allErrors = append(allErrors, field.Forbidden(field.NewPath("metadata").Child("namespace"),
			fmt.Sprintf("A Ytsaurus object already exists in the given namespace %s", newYtsaurus.Namespace)))
	}

	return allErrors
}

func (r *baseValidator) validateCommonSpec(spec *CommonSpec) field.ErrorList {
	var allErrors field.ErrorList
	path := field.NewPath("spec")

	allErrors = append(allErrors, validation.ValidateAnnotations(spec.ExtraPodAnnotations, path.Child("extraPodAnnotations"))...)

	return allErrors
}

func (r *baseValidator) validateUpdateSelectors(newYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList
	hasNothing := false

	if newYtsaurus.Spec.UpdatePlan != nil {
		if len(newYtsaurus.Spec.UpdatePlan) == 0 {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("updatePlan"), "updatePlan should not be empty"))
		}

		for i, updateSelector := range newYtsaurus.Spec.UpdatePlan {
			if updateSelector.Class != consts.ComponentClassUnspecified && (updateSelector.Component.Type != "" || updateSelector.Component.Name != "") {
				allErrors = append(allErrors, field.Invalid(field.NewPath("spec").Child("updatePlan").Index(i).Child("component"), updateSelector.Component, "Only one of component or class should be specified"))
			}
			if updateSelector.Class == consts.ComponentClassUnspecified && updateSelector.Component.Type == "" && updateSelector.Component.Name == "" {
				allErrors = append(allErrors, field.Invalid(field.NewPath("spec").Child("updatePlan").Index(i).Child("component"), updateSelector.Component, "Either component or class should be specified"))
			}
			if updateSelector.Class == consts.ComponentClassNothing {
				hasNothing = true
			}
		}
		if hasNothing && len(newYtsaurus.Spec.UpdatePlan) > 1 {
			allErrors = append(allErrors, field.Invalid(field.NewPath("spec").Child("updatePlan"), newYtsaurus.Spec.UpdatePlan, "updatePlan should contain only one Nothing class"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateYtsaurus(ctx context.Context, newYtsaurus, oldYtsaurus *Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	allErrors = append(allErrors, r.validateCommonSpec(&newYtsaurus.Spec.CommonSpec)...)
	allErrors = append(allErrors, r.validateDiscovery(newYtsaurus)...)
	allErrors = append(allErrors, r.validatePrimaryMasters(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateSecondaryMasters(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateHTTPProxies(newYtsaurus)...)
	allErrors = append(allErrors, r.validateRPCProxies(newYtsaurus)...)
	allErrors = append(allErrors, r.validateTCPProxies(newYtsaurus)...)
	allErrors = append(allErrors, r.validateDataNodes(newYtsaurus)...)
	allErrors = append(allErrors, r.validateExecNodes(newYtsaurus)...)
	allErrors = append(allErrors, r.validateSchedulers(newYtsaurus)...)
	allErrors = append(allErrors, r.validateControllerAgents(newYtsaurus)...)
	allErrors = append(allErrors, r.validateTabletNodes(newYtsaurus)...)
	allErrors = append(allErrors, r.validateChyt(newYtsaurus)...)
	allErrors = append(allErrors, r.validateStrawberry(newYtsaurus)...)
	allErrors = append(allErrors, r.validateQueryTrackers(newYtsaurus)...)
	allErrors = append(allErrors, r.validateQueueAgents(newYtsaurus)...)
	allErrors = append(allErrors, r.validateSpyt(newYtsaurus)...)
	allErrors = append(allErrors, r.validateYQLAgents(newYtsaurus)...)
	allErrors = append(allErrors, r.validateUi(newYtsaurus)...)
	allErrors = append(allErrors, r.validateExistsYtsaurus(ctx, newYtsaurus)...)
	allErrors = append(allErrors, r.validateUpdateSelectors(newYtsaurus)...)

	return allErrors
}

func (r *ytsaurusValidator) evaluateYtsaurusValidation(ctx context.Context, newYtsaurus, oldYtsaurus *Ytsaurus) (admission.Warnings, error) {
	allErrors := r.validateYtsaurus(ctx, newYtsaurus, oldYtsaurus)
	if len(allErrors) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(YtsaurusGVK.GroupKind(), newYtsaurus.Name, allErrors)
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ytsaurusValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	newYtsaurus, ok := obj.(*Ytsaurus)
	ytsauruslog.Info("validate create", "name", newYtsaurus.Name)
	if !ok {
		return nil, fmt.Errorf("expected a Ytsaurus but got a %T", obj)
	}
	return r.evaluateYtsaurusValidation(ctx, newYtsaurus, nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ytsaurusValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldYtsaurus, ok := oldObj.(*Ytsaurus)
	if !ok {
		return nil, fmt.Errorf("expected a Ytsaurus but got a %T", oldYtsaurus)
	}
	ytsauruslog.Info("validate update", "name", oldYtsaurus.Name)
	newYtsaurus, ok := newObj.(*Ytsaurus)
	if !ok {
		return nil, fmt.Errorf("expected a Ytsaurus but got a %T", newYtsaurus)
	}
	return r.evaluateYtsaurusValidation(ctx, newYtsaurus, oldYtsaurus)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ytsaurusValidator) ValidateDelete(_ context.Context, newObj runtime.Object) (admission.Warnings, error) {
	newYtsaurus, ok := newObj.(*Ytsaurus)
	if !ok {
		return nil, fmt.Errorf("expected a Ytsaurus but got a %T", newYtsaurus)
	}
	ytsauruslog.Info("validate delete", "name", newYtsaurus.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
