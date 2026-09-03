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
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	v1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/components"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/version"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-cluster-ytsaurus-tech-v1-ytsaurus,mutating=false,failurePolicy=fail,sideEffects=None,groups=cluster.ytsaurus.tech,resources=ytsaurus,verbs=create;update,versions=v1,name=vytsaurus.kb.io,admissionReviewVersions=v1

type baseValidator struct{}

type ytsaurusValidator struct {
	customValidator[*ytv1.Ytsaurus]
	baseValidator
}

func NewYtsaurusValidator() *ytsaurusValidator {
	r := &ytsaurusValidator{}
	r.Object = &ytv1.Ytsaurus{}
	r.Validate = r.evaluateYtsaurusValidation
	return r
}

func oldCommonSpec(oldYtsaurus *ytv1.Ytsaurus) *ytv1.CommonSpec {
	if oldYtsaurus == nil {
		return nil
	}
	return &oldYtsaurus.Spec.CommonSpec
}

func (r *baseValidator) validateTransportSecurity(spec *ytv1.RPCTransportSpec, commonSpec *ytv1.CommonSpec, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	features := ptr.Deref(commonSpec.ClusterFeatures, ytv1.ClusterFeatures{})

	if spec == nil {
		spec = commonSpec.NativeTransport
	}

	if spec == nil {
		if features.SecureClusterTransports {
			allErrors = append(allErrors, field.Required(path, "Secure cluster transport demands TLS setup"))
		}
		return allErrors
	}

	secretPath := path.Child("tlsSecret")
	clientSecretPath := path.Child("tlsClientSecret")
	if spec.TLSSecret == nil && (spec.TLSRequired || !spec.TLSInsecure) {
		allErrors = append(allErrors, field.Required(secretPath, "TLS certificate for native transport is required"))
	}
	if spec.TLSClientSecret == nil && (spec.TLSRequired && !spec.TLSInsecure) {
		allErrors = append(allErrors, field.Required(clientSecretPath, "Client TLS certificate for native transport is required"))
	}

	if features.SecureClusterTransports {
		if !spec.TLSRequired {
			allErrors = append(allErrors, field.Forbidden(path.Child("tlsRequired"), "Secure cluster transport demands TLS-only native transport"))
		}
		if spec.TLSInsecure {
			allErrors = append(allErrors, field.Forbidden(path.Child("tlsInsecure"), "Secure cluster transport demands TLS certificate validation"))
		}
		if spec.TLSSecret == nil {
			allErrors = append(allErrors, field.Required(secretPath, "Secure cluster transport demands TLS certificate for native transport"))
		}
		if spec.TLSClientSecret == nil {
			allErrors = append(allErrors, field.Required(clientSecretPath, "Secure cluster transport demands client TLS certificate for native transport"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateDiscovery(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	var oldInstanceSpec *ytv1.InstanceSpec
	if oldYtsaurus != nil {
		oldInstanceSpec = &oldYtsaurus.Spec.Discovery.InstanceSpec
	}
	allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.Discovery.InstanceSpec, oldInstanceSpec,
		&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), field.NewPath("spec").Child("discovery"))...)

	return allErrors
}

//nolint:cyclop,nestif //ok
func (r *ytsaurusValidator) validateMasterSpec(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus, mastersSpec, oldMastersSpec *ytv1.MastersSpec, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	var oldInstanceSpec *ytv1.InstanceSpec
	if oldMastersSpec != nil {
		oldInstanceSpec = &oldMastersSpec.InstanceSpec
	}
	allErrors = append(allErrors, r.validateInstanceSpec(mastersSpec.InstanceSpec, oldInstanceSpec,
		&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)
	allErrors = append(allErrors, r.validateTimbertruckSpec(mastersSpec.Timbertruck, newYtsaurus.Spec.Timbertruck, mastersSpec.StructuredLoggers, mastersSpec.Locations, path)...)
	allErrors = append(allErrors, r.validateHostAddresses(newYtsaurus, mastersSpec, path)...)

	if ytv1.FindFirstLocation(mastersSpec.Locations, ytv1.LocationTypeMasterChangelogs) == nil {
		allErrors = append(allErrors, field.NotFound(path.Child("locations"), ytv1.LocationTypeMasterChangelogs))
	}

	if ytv1.FindFirstLocation(mastersSpec.Locations, ytv1.LocationTypeMasterSnapshots) == nil {
		allErrors = append(allErrors, field.NotFound(path.Child("locations"), ytv1.LocationTypeMasterSnapshots))
	}

	if oldMastersSpec != nil && oldMastersSpec.CellTag != mastersSpec.CellTag {
		allErrors = append(allErrors, field.Invalid(path.Child("cellTag"), mastersSpec.CellTag, "Could not be changed"))
	}

	rolesPath := path.Child("roles")
	isPrimary := mastersSpec.CellTag == newYtsaurus.Spec.PrimaryMasters.CellTag
	isMulticell := len(newYtsaurus.Spec.SecondaryMasters) > 0
	cellRoles := UniqueValues[ytv1.MasterCellRole]{}
	allErrors = append(allErrors, cellRoles.InsertAll(ytv1.GetMasterCellRoles(mastersSpec.Roles, isPrimary, isMulticell), rolesPath)...)
	cellRolesChanged := false

	if cellRoles.Count(ytv1.MasterCellRoleChunkHost, ytv1.MasterCellRoleDedicatedChunkHost) > 1 {
		allErrors = append(allErrors, field.Forbidden(rolesPath, fmt.Sprintf("Master cell %v has role conflict: %v %v",
			mastersSpec.CellTag, ytv1.MasterCellRoleChunkHost, ytv1.MasterCellRoleDedicatedChunkHost)))
	}

	if oldMastersSpec != nil && oldMastersSpec.InstanceCount > 0 {
		wasMulticell := len(oldYtsaurus.Spec.SecondaryMasters) > 0
		oldCellRoles := ytv1.GetMasterCellRoles(oldMastersSpec.Roles, isPrimary, wasMulticell)
		cellRolesChanged = len(cellRoles) != len(oldCellRoles)
		for _, role := range oldCellRoles {
			if _, found := cellRoles[role]; !found {
				cellRolesChanged = true
				if (role == ytv1.MasterCellRoleChunkHost || role == ytv1.MasterCellRoleDedicatedChunkHost) &&
					cellRoles.Count(ytv1.MasterCellRoleChunkHost, ytv1.MasterCellRoleDedicatedChunkHost) > 0 {
					// Allow interchangeable chunk host roles.
				} else {
					allErrors = append(allErrors, field.Forbidden(rolesPath, fmt.Sprintf("Cell %v role could not be removed: %v", mastersSpec.CellTag, role)))
				}
			}
		}
		if isPrimary && isMulticell && !wasMulticell && mastersSpec.Roles == nil {
			allErrors = append(allErrors, field.Required(rolesPath, "Upgrade to multicell requires filling roles for primary cell"))
		}
	}

	newInstanceCount := mastersSpec.InstanceCount
	newMinReady := min(newInstanceCount, ptr.Deref(mastersSpec.MinReadyInstanceCount, newInstanceCount))
	if newInstanceCount > 0 && newMinReady <= newInstanceCount/2 && !newYtsaurus.Spec.EphemeralCluster {
		allErrors = append(allErrors, field.Invalid(path.Child("minReadyInstanceCount"), newMinReady, "Must be bigger than half of instanceCount"))
	}
	if newMinReady > newInstanceCount {
		allErrors = append(allErrors, field.Invalid(path.Child("minReadyInstanceCount"), newMinReady, "Cannot be bigger than instanceCount"))
	}

	instanceCountPath := path.Child("instanceCount")
	if oldMastersSpec == nil {
		if oldYtsaurus != nil && !isPrimary && newInstanceCount != 0 {
			allErrors = append(allErrors, field.Forbidden(instanceCountPath, "New secondary master cell initially must have instanceCount = 0"))
		}
	} else {
		oldInstanceCount := oldMastersSpec.InstanceCount
		oldMinReady := min(oldInstanceCount, ptr.Deref(oldMastersSpec.MinReadyInstanceCount, oldInstanceCount))
		if !isPrimary && newInstanceCount != 0 && oldInstanceCount == 0 {
			if !slices.ContainsFunc(newYtsaurus.Status.UpdateStatus.MasterCellsMaintenance, func(info ytv1.MasterCellMaintenanceInfo) bool {
				return info.CellTag == mastersSpec.CellTag && info.Unregistered
			}) {
				allErrors = append(allErrors, field.Invalid(instanceCountPath, mastersSpec.InstanceCount, "Master cell must be unregistered"))
			}
		}
		if newInstanceCount < 1 && oldInstanceCount > 0 {
			allErrors = append(allErrors, field.Invalid(instanceCountPath, mastersSpec.InstanceCount, "Cannot remove last instance"))
		}
		if newInstanceCount/2 < oldInstanceCount-oldMinReady {
			allErrors = append(allErrors, field.Invalid(instanceCountPath, mastersSpec.InstanceCount,
				"Cannot shrink without possibility of losing quorum, increase minReadyInstanceCount first"))
		}
		if newInstanceCount > oldMinReady*2-1 && oldInstanceCount > 1 {
			allErrors = append(allErrors, field.Invalid(instanceCountPath, mastersSpec.InstanceCount,
				fmt.Sprintf("Cannot grow bigger than previous minReadyInstanceCount*2-1 (%d) in one step", oldMinReady*2-1)))
		}

		oldMaintenance := ptr.Deref(oldYtsaurus.Spec.ClusterMaintenance, ytv1.ClusterMaintenance{})
		newMaintenance := ptr.Deref(newYtsaurus.Spec.ClusterMaintenance, ytv1.ClusterMaintenance{})
		if oldMaintenance.Shutdown != ytv1.ClusterShutdownExceptMasters || newMaintenance.Shutdown != ytv1.ClusterShutdownExceptMasters {
			if oldInstanceCount != newInstanceCount {
				allErrors = append(allErrors, field.Forbidden(instanceCountPath, "Could be changed only during master cells maintenance"))
			}
			if cellRolesChanged {
				allErrors = append(allErrors, field.Forbidden(rolesPath, "Could be changed only during master cells maintenance"))
			}
		}
		if oldInstanceCount > 0 && cellRolesChanged && (oldMaintenance.AssignMasterCellsRoles || !newMaintenance.AssignMasterCellsRoles) {
			allErrors = append(allErrors, field.Forbidden(rolesPath, "Could be changed only together with enabling assignMasterCellsRoles"))
		}
	}

	if mastersSpec.InstanceCount > 1 && !newYtsaurus.Spec.EphemeralCluster {
		if affinity := mastersSpec.Affinity; affinity == nil || affinity.PodAntiAffinity == nil {
			allErrors = append(allErrors, field.Required(path.Child("affinity").Child("podAntiAffinity"),
				"Masters should be placed on different nodes"))
		}
	}

	allErrors = append(allErrors, r.validateHydraPersistenceUploaderSpec(mastersSpec.HydraPersistenceUploader, mastersSpec.Locations, path)...)

	allErrors = append(allErrors, r.validateSidecars(mastersSpec.Sidecars, path.Child("sidecars"))...)

	return allErrors
}

func (r *ytsaurusValidator) validatePrimaryMasters(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	mastersSpec := &newYtsaurus.Spec.PrimaryMasters
	path := field.NewPath("spec").Child("primaryMasters")

	var oldMastersSpec *ytv1.MastersSpec
	if oldYtsaurus != nil {
		oldMastersSpec = &oldYtsaurus.Spec.PrimaryMasters
	}

	allErrors = append(allErrors, r.validateMasterSpec(newYtsaurus, oldYtsaurus, mastersSpec, oldMastersSpec, path)...)

	if mastersSpec.InstanceCount < 1 {
		allErrors = append(allErrors, field.Invalid(path.Child("instanceCount"), mastersSpec.InstanceCount, "Cannot be below 1"))
	}

	return allErrors
}

func (r *ytsaurusValidator) validateSecondaryMasters(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	cellTags := UniqueValues[uint16]{}
	cellTags.Insert(newYtsaurus.Spec.PrimaryMasters.CellTag, field.NewPath("spec").Child("primaryMasters").Child("cellTag"))

	// TODO(khlebnikov): Add option for RPC port to collocate primary and secondary masters in host network.
	hostAddresses := UniqueValues[string]{}
	allErrors = append(allErrors, hostAddresses.InsertAll(newYtsaurus.Spec.PrimaryMasters.HostAddresses, field.NewPath("spec").Child("primaryMasters").Child("hostAddresses"))...)

	secondaryMastersPath := field.NewPath("spec").Child("secondaryMasters")
	for i := range newYtsaurus.Spec.SecondaryMasters {
		path := secondaryMastersPath.Index(i)
		mastersSpec := &newYtsaurus.Spec.SecondaryMasters[i]
		var oldMastersSpec *ytv1.MastersSpec
		if oldYtsaurus != nil && len(oldYtsaurus.Spec.SecondaryMasters) > i {
			oldMastersSpec = &oldYtsaurus.Spec.SecondaryMasters[i]
		}
		allErrors = append(allErrors, r.validateMasterSpec(newYtsaurus, oldYtsaurus, mastersSpec, oldMastersSpec, path)...)
		allErrors = append(allErrors, cellTags.Insert(mastersSpec.CellTag, path.Child("cellTag"))...)
		allErrors = append(allErrors, hostAddresses.InsertAll(mastersSpec.HostAddresses, path.Child("hostAddresses"))...)
	}

	if cnt := len(cellTags) - 1; cnt > consts.MaxSecondaryMasterCells {
		allErrors = append(allErrors, field.TooMany(secondaryMastersPath, cnt, consts.MaxSecondaryMasterCells))
	}
	for cellTag, path := range cellTags {
		if cellTag < consts.MinValidCellTag || cellTag > consts.MaxValidCellTag {
			allErrors = append(allErrors, field.Invalid(path, cellTag, fmt.Sprintf("Cell tag must be in range %v..%v", consts.MinValidCellTag, consts.MaxValidCellTag)))
		}
	}

	if oldYtsaurus != nil {
		// It is ok to remove unborn secondary cells without instances from the end of the list.
		for i := len(newYtsaurus.Spec.SecondaryMasters); i < len(oldYtsaurus.Spec.SecondaryMasters); i++ {
			oldMastersSpec := &oldYtsaurus.Spec.SecondaryMasters[i]
			if oldMastersSpec.InstanceCount > 0 {
				path := field.NewPath("spec").Child("secondaryMasters").Index(i)
				allErrors = append(allErrors, field.Forbidden(path, "Cannot remove cell with instances"))
			}
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateHostAddresses(newYtsaurus *ytv1.Ytsaurus, mastersSpec *ytv1.MastersSpec, fieldPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	hostAddressesFieldPath := fieldPath.Child("hostAddresses")
	if !ptr.Deref(mastersSpec.HostNetwork, ptr.Deref(newYtsaurus.Spec.HostNetwork, false)) && len(mastersSpec.HostAddresses) != 0 {
		allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("hostNetwork"),
			fmt.Sprintf("%s doesn't make sense without hostNetwork=true", hostAddressesFieldPath.String())))
	}

	if len(mastersSpec.HostAddresses) != 0 && len(mastersSpec.HostAddresses) != int(mastersSpec.InstanceCount) {
		instanceCountFieldPath := fieldPath.Child("instanceCount")
		allErrors = append(allErrors, field.Invalid(hostAddressesFieldPath, mastersSpec.HostAddresses,
			fmt.Sprintf("%s list length should be equal to %s", hostAddressesFieldPath.String(), instanceCountFieldPath.String())))
	}

	return allErrors
}

func (r *ytsaurusValidator) validateHTTPProxies(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	features := ptr.Deref(newYtsaurus.Spec.ClusterFeatures, ytv1.ClusterFeatures{})
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

		var oldInstanceSpec *ytv1.InstanceSpec
		if oldYtsaurus != nil && i < len(oldYtsaurus.Spec.HTTPProxies) {
			oldInstanceSpec = &oldYtsaurus.Spec.HTTPProxies[i].InstanceSpec
		}
		allErrors = append(allErrors, r.validateInstanceSpec(hp.InstanceSpec, oldInstanceSpec,
			&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)

		if features.HTTPProxyHaveHTTPSAddress && hp.Transport.HTTPSSecret == nil {
			allErrors = append(allErrors, field.Required(
				path.Child("transport").Child("httpsSecret"),
				"Cluster feature httpProxyHaveHttpsAddress requires HTTPS for all HTTP proxies",
			))
		}

		if features.SecureClusterTransports && !hp.Transport.DisableHTTP {
			allErrors = append(allErrors, field.Forbidden(
				path.Child("transport").Child("disableHttp"),
				"Secure cluster transport demands HTTPS-only proxies",
			))
		}
	}

	if !hasDefaultHTTPProxy {
		allErrors = append(allErrors, field.Required(
			field.NewPath("spec").Child("httpProxies"),
			fmt.Sprintf("HTTP proxy with `%s` role should exist", consts.DefaultHTTPProxyRole)))
	}

	return allErrors
}

func (r *ytsaurusValidator) validateRPCProxies(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	features := ptr.Deref(newYtsaurus.Spec.ClusterFeatures, ytv1.ClusterFeatures{})
	rpcRoles := make(map[string]bool)
	for i, rp := range newYtsaurus.Spec.RPCProxies {
		path := field.NewPath("spec").Child("rpcProxies").Index(i)
		if _, exists := rpcRoles[rp.Role]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("role"), rp.Role))
		}
		rpcRoles[rp.Role] = true

		var oldInstanceSpec *ytv1.InstanceSpec
		if oldYtsaurus != nil && i < len(oldYtsaurus.Spec.RPCProxies) {
			oldInstanceSpec = &oldYtsaurus.Spec.RPCProxies[i].InstanceSpec
		}
		allErrors = append(allErrors, r.validateInstanceSpec(rp.InstanceSpec, oldInstanceSpec,
			&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)

		transportPath := path.Child("transport")
		if rp.Transport.TLSRequired && rp.Transport.TLSSecret == nil {
			allErrors = append(allErrors, field.Required(transportPath.Child("tlsSecret"), "TLS-only RPC proxy requires certificate"))
		}

		if features.SecureClusterTransports {
			if !rp.Transport.TLSRequired {
				allErrors = append(allErrors, field.Required(transportPath.Child("tlsRequired"), "Secure cluster transport demands TLS-only RPC proxies"))
			}
			if rp.Transport.TLSInsecure {
				allErrors = append(allErrors, field.Forbidden(transportPath.Child("tlsInsecure"), "Secure cluster transport demands TLS certificate validation"))
			}
			if rp.Transport.TLSSecret == nil {
				allErrors = append(allErrors, field.Required(transportPath.Child("tlsSecret"), "Secure cluster transport demands RPC proxy certificate"))
			}
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateTCPProxies(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	tcpRoles := make(map[string]bool)
	for i, rp := range newYtsaurus.Spec.TCPProxies {
		path := field.NewPath("spec").Child("tcpProxies").Index(i)
		if _, exists := tcpRoles[rp.Role]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("role"), rp.Role))
		}
		tcpRoles[rp.Role] = true

		var oldInstanceSpec *ytv1.InstanceSpec
		if oldYtsaurus != nil && i < len(oldYtsaurus.Spec.TCPProxies) {
			oldInstanceSpec = &oldYtsaurus.Spec.TCPProxies[i].InstanceSpec
		}
		allErrors = append(allErrors, r.validateInstanceSpec(rp.InstanceSpec, oldInstanceSpec,
			&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)
	}

	return allErrors
}

func (r *ytsaurusValidator) validateDataNodes(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	names := make(map[string]bool)
	for i, dn := range newYtsaurus.Spec.DataNodes {
		path := field.NewPath("spec").Child("dataNodes").Index(i)

		if _, exists := names[dn.Name]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("name"), dn.Name))
		}
		names[dn.Name] = true

		var oldInstanceSpec *ytv1.InstanceSpec
		if oldYtsaurus != nil && i < len(oldYtsaurus.Spec.DataNodes) {
			oldInstanceSpec = &oldYtsaurus.Spec.DataNodes[i].InstanceSpec
		}
		allErrors = append(allErrors, r.validateInstanceSpec(dn.InstanceSpec, oldInstanceSpec,
			&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)

		if ytv1.FindFirstLocation(dn.Locations, ytv1.LocationTypeChunkStore) == nil {
			allErrors = append(allErrors, field.NotFound(path.Child("locations"), ytv1.LocationTypeChunkStore))
		}
	}

	return allErrors
}

func (r *baseValidator) validateHydraPersistenceUploaderSpec(
	hydraPersistenceUploader *ytv1.HydraPersistenceUploaderSpec, locations []ytv1.LocationSpec, parentPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	if hydraPersistenceUploader == nil {
		return allErrors
	}

	if hydraPersistenceUploader.Image == nil {
		allErrors = append(allErrors, field.Required(parentPath.Child("hydraPersistenceUploader", "image"), "hydraPersistenceUploader image is required"))
	}

	allErrors = append(allErrors, requireLocations(
		parentPath.Child("locations"),
		locations,
		"hydraPersistenceUploader",
		ytv1.LocationTypeMasterSnapshots,
		ytv1.LocationTypeMasterChangelogs,
	)...)

	return allErrors
}

// validateTimbertruckSpec validates timbertruck log delivery for a single component. Delivery is
// enabled either by a per-log enableDelivery flag (any component) or, for backward compatibility,
// by the mere presence of a component-level timbertruck spec (masters). The image may come from the
// component override or the cluster-wide spec.timbertruck.
func (r *baseValidator) validateTimbertruckSpec(
	componentTimbertruck *ytv1.TimbertruckSpec,
	commonTimbertruck *ytv1.TimbertruckSpec,
	structuredLoggers []ytv1.StructuredLoggerSpec,
	locations []ytv1.LocationSpec,
	parentPath *field.Path,
) field.ErrorList {
	var allErrors field.ErrorList

	anyPerLogDelivery := false
	for _, logger := range structuredLoggers {
		if logger.EnableDelivery != nil && *logger.EnableDelivery {
			anyPerLogDelivery = true
			break
		}
	}
	// Legacy master mode: a component-level timbertruck spec delivers all structured loggers.
	legacyMaster := componentTimbertruck != nil

	if !anyPerLogDelivery && !legacyMaster {
		return allErrors
	}

	hasImage := func(tt *ytv1.TimbertruckSpec) bool {
		return tt != nil && tt.Image != nil && *tt.Image != ""
	}
	if !hasImage(componentTimbertruck) && !hasImage(commonTimbertruck) {
		allErrors = append(allErrors, field.Required(parentPath.Child("timbertruck", "image"),
			"timbertruck image is required (set it here or in spec.timbertruck.image) when log delivery is enabled"))
	}
	if legacyMaster && len(structuredLoggers) == 0 {
		allErrors = append(allErrors, field.Required(parentPath.Child("structuredLoggers"),
			"structuredLoggers must be configured when timbertruck is enabled"))
	}
	allErrors = append(allErrors, requireLocations(
		parentPath.Child("locations"),
		locations,
		"timbertruck",
		ytv1.LocationTypeLogs,
	)...)
	return allErrors
}

func validateStructuredLoggers(structuredLoggers []ytv1.StructuredLoggerSpec, parentPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	for loggerIdx, logger := range structuredLoggers {
		loggerPath := parentPath.Child("structuredLoggers").Index(loggerIdx)

		if (logger.Category != nil) == (logger.CategoriesFilter != nil) {
			allErrors = append(allErrors, field.Invalid(loggerPath, logger.Name,
				"exactly one of category and categoriesFilter must be set"))
		}
		if logger.Category != nil && *logger.Category == "" {
			allErrors = append(allErrors, field.Required(loggerPath.Child("category"),
				"category must not be empty"))
		}
		// Without both, the rule ends up with no category restriction and matches every category.
		if filter := logger.CategoriesFilter; filter != nil && (filter.Type == "" || len(filter.Values) == 0) {
			allErrors = append(allErrors, field.Required(loggerPath.Child("categoriesFilter"),
				"categoriesFilter requires type and values"))
		}
	}

	return allErrors
}

func requireLocations(
	locationsPath *field.Path,
	locations []ytv1.LocationSpec,
	reason string,
	locationTypes ...ytv1.LocationType,
) field.ErrorList {
	var allErrors field.ErrorList
	for _, locationType := range locationTypes {
		if ytv1.FindFirstLocation(locations, locationType) == nil {
			allErrors = append(allErrors, field.Required(
				locationsPath,
				fmt.Sprintf("%s location must be configured for %s", locationType, reason),
			))
		}
	}
	return allErrors
}

// validateExtraTimbertruckComponents validates timbertruck log delivery for the server components
// that are not covered by a dedicated validateInstanceSpec call but can still opt into delivery via
// a per-log enableDelivery flag. It mirrors the runtime enumeration in components so the webhook and
// the operator agree on which components may deliver logs. componentTimbertruck is always nil here
// because only masters carry a component-level timbertruck spec.
func (r *ytsaurusValidator) validateExtraTimbertruckComponents(newYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList
	commonTimbertruck := newYtsaurus.Spec.Timbertruck
	specPath := field.NewPath("spec")

	validate := func(instanceSpec *ytv1.InstanceSpec, path *field.Path) {
		allErrors = append(allErrors, r.validateTimbertruckSpec(nil, commonTimbertruck,
			instanceSpec.StructuredLoggers, instanceSpec.Locations, path)...)
		allErrors = append(allErrors, validateStructuredLoggers(instanceSpec.StructuredLoggers, path)...)
	}

	if mc := newYtsaurus.Spec.MasterCaches; mc != nil {
		validate(&mc.InstanceSpec, specPath.Child("masterCaches"))
	}
	for i := range newYtsaurus.Spec.KafkaProxies {
		validate(&newYtsaurus.Spec.KafkaProxies[i].InstanceSpec, specPath.Child("kafkaProxies").Index(i))
	}
	if cp := newYtsaurus.Spec.CypressProxies; cp != nil {
		validate(&cp.InstanceSpec, specPath.Child("cypressProxies"))
	}
	if bc := newYtsaurus.Spec.BundleController; bc != nil {
		validate(&bc.InstanceSpec, specPath.Child("bundleController"))
	}
	if tb := newYtsaurus.Spec.TabletBalancer; tb != nil {
		validate(&tb.InstanceSpec, specPath.Child("tabletBalancers"))
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

func (r *ytsaurusValidator) validateExecNodes(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	names := make(map[string]bool)
	for i, en := range newYtsaurus.Spec.ExecNodes {
		path := field.NewPath("spec").Child("execNodes").Index(i)

		if _, exists := names[en.Name]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("name"), en.Name))
		}
		names[en.Name] = true

		var oldInstanceSpec *ytv1.InstanceSpec
		if oldYtsaurus != nil && i < len(oldYtsaurus.Spec.ExecNodes) {
			oldInstanceSpec = &oldYtsaurus.Spec.ExecNodes[i].InstanceSpec
		}
		allErrors = append(allErrors, r.validateInstanceSpec(en.InstanceSpec, oldInstanceSpec,
			&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)

		if ytv1.FindFirstLocation(en.Locations, ytv1.LocationTypeChunkCache) == nil {
			allErrors = append(allErrors, field.NotFound(path.Child("locations"), ytv1.LocationTypeChunkCache))
		}

		if ytv1.FindFirstLocation(en.Locations, ytv1.LocationTypeSlots) == nil {
			allErrors = append(allErrors, field.NotFound(path.Child("locations"), ytv1.LocationTypeSlots))
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

func (r *ytsaurusValidator) validateSchedulers(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.Schedulers != nil {
		path := field.NewPath("spec").Child("schedulers")
		var oldInstanceSpec *ytv1.InstanceSpec
		if oldYtsaurus != nil && oldYtsaurus.Spec.Schedulers != nil {
			oldInstanceSpec = &oldYtsaurus.Spec.Schedulers.InstanceSpec
		}
		allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.Schedulers.InstanceSpec, oldInstanceSpec,
			&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)

		if newYtsaurus.Spec.ControllerAgents == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("controllerAgents"),
				"schedulers doesn't make sense without controllerAgents"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateControllerAgents(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.ControllerAgents != nil {
		path := field.NewPath("spec").Child("controllerAgents")
		var oldInstanceSpec *ytv1.InstanceSpec
		if oldYtsaurus != nil && oldYtsaurus.Spec.ControllerAgents != nil {
			oldInstanceSpec = &oldYtsaurus.Spec.ControllerAgents.InstanceSpec
		}
		allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.ControllerAgents.InstanceSpec, oldInstanceSpec,
			&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)

		if newYtsaurus.Spec.Schedulers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("schedulers"),
				"controllerAgents doesn't make sense without schedulers"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateTabletNodes(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	names := make(map[string]bool)
	for i, tn := range newYtsaurus.Spec.TabletNodes {
		path := field.NewPath("spec").Child("tabletNodes").Index(i)

		if _, exists := names[tn.Name]; exists {
			allErrors = append(allErrors, field.Duplicate(path.Child("name"), tn.Name))
		}
		names[tn.Name] = true

		var oldInstanceSpec *ytv1.InstanceSpec
		if oldYtsaurus != nil && i < len(oldYtsaurus.Spec.TabletNodes) {
			oldInstanceSpec = &oldYtsaurus.Spec.TabletNodes[i].InstanceSpec
		}
		allErrors = append(allErrors, r.validateInstanceSpec(tn.InstanceSpec, oldInstanceSpec,
			&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)
	}

	return allErrors
}

func (r *ytsaurusValidator) validateChyt(_ *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	return allErrors
}

func (r *ytsaurusValidator) validateStrawberry(newYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.StrawberryController != nil {
		if newYtsaurus.Spec.Schedulers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("schedulers"),
				"schedulers are required for strawberry"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateQueryTrackers(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.QueryTrackers != nil {
		path := field.NewPath("spec").Child("queryTrackers")
		var oldInstanceSpec *ytv1.InstanceSpec
		if oldYtsaurus != nil && oldYtsaurus.Spec.QueryTrackers != nil {
			oldInstanceSpec = &oldYtsaurus.Spec.QueryTrackers.InstanceSpec
		}
		allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.QueryTrackers.InstanceSpec, oldInstanceSpec,
			&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)

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

func (r *ytsaurusValidator) validateQueueAgents(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.QueueAgents != nil {
		path := field.NewPath("spec").Child("queueAgents")
		var oldInstanceSpec *ytv1.InstanceSpec
		if oldYtsaurus != nil && oldYtsaurus.Spec.QueueAgents != nil {
			oldInstanceSpec = &oldYtsaurus.Spec.QueueAgents.InstanceSpec
		}
		allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.QueueAgents.InstanceSpec, oldInstanceSpec,
			&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)

		if len(newYtsaurus.Spec.TabletNodes) == 0 {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("tabletNodes"),
				"tabletNodes are required for queueAgents"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateSpyt(newYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList
	path := field.NewPath("spec").Child("spyt")

	if newYtsaurus.Spec.Spyt != nil {
		allErrors = append(allErrors, field.Invalid(path, newYtsaurus.Spec.Spyt,
			"spyt is deprecated here, use Spyt resource instead"))
	}

	return allErrors
}

func (r *ytsaurusValidator) validateYQLAgents(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if newYtsaurus.Spec.YQLAgents != nil {
		path := field.NewPath("spec").Child("YQLAgents")
		var oldInstanceSpec *ytv1.InstanceSpec
		if oldYtsaurus != nil && oldYtsaurus.Spec.YQLAgents != nil {
			oldInstanceSpec = &oldYtsaurus.Spec.YQLAgents.InstanceSpec
		}
		allErrors = append(allErrors, r.validateInstanceSpec(newYtsaurus.Spec.YQLAgents.InstanceSpec, oldInstanceSpec,
			&newYtsaurus.Spec.CommonSpec, oldCommonSpec(oldYtsaurus), path)...)

		if newYtsaurus.Spec.QueryTrackers == nil {
			allErrors = append(allErrors, field.Required(field.NewPath("spec").Child("queryTrackers"),
				"yqlAgents doesn't make sense without queryTrackers"))
		}
	}

	return allErrors
}

func (r *ytsaurusValidator) validateUi(newYtsaurus *ytv1.Ytsaurus) field.ErrorList {
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

func (r *baseValidator) validatePodSpec(podSpec *ytv1.PodSpec, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	allErrors = append(allErrors, v1validation.ValidateLabels(podSpec.PodLabels, path.Child("podLabels"))...)
	allErrors = append(allErrors, validation.ValidateAnnotations(podSpec.PodAnnotations, path.Child("podAnnotations"))...)

	return allErrors
}

func (r *baseValidator) validateImageUpdate(
	newImage *string,
	oldImage *string,
	commonSpec *ytv1.CommonSpec,
	oldCommonSpec *ytv1.CommonSpec,
	path *field.Path,
) field.ErrorList {
	var errs field.ErrorList
	downtime := ptr.Deref(commonSpec.ClusterMaintenance, ytv1.ClusterMaintenance{}).Downtime
	if downtime != ytv1.ClusterDowntimeMinor || (oldImage == nil && oldCommonSpec == nil) {
		return nil
	}
	newImg := ptr.Deref(newImage, oldCommonSpec.CoreImage)
	oldImg := ptr.Deref(oldImage, oldCommonSpec.CoreImage)
	if newImg == oldImg {
		return nil
	}
	newVersion, err := version.ParseYtsaurusImageVersion(newImg)
	if err != nil {
		errs = append(errs, field.Invalid(path, newImg, fmt.Sprintf("cannot parse new image version: %v", err)))
	}
	oldVersion, err := version.ParseYtsaurusImageVersion(oldImg)
	if err != nil {
		errs = append(errs, field.Invalid(path, oldImg, fmt.Sprintf("cannot parse old image version: %v", err)))
	}
	if oldVersion != nil && newVersion != nil {
		if newVersion.Major() != oldVersion.Major() || newVersion.Minor() != oldVersion.Minor() {
			errs = append(errs, field.Forbidden(path, fmt.Sprintf("image update from version %v to %v is incompatible with minor downtime", oldVersion, newVersion)))
		} else if newVersion.Patch() < oldVersion.Patch() {
			errs = append(errs, field.Forbidden(path, fmt.Sprintf("image downgrade from version %v to %v is incompatible with minor downtime", oldVersion, newVersion)))
		}
	}
	return errs
}

func (r *baseValidator) validateInstanceSpec(
	instanceSpec ytv1.InstanceSpec,
	oldInstanceSpec *ytv1.InstanceSpec,
	commonSpec *ytv1.CommonSpec,
	oldCommonSpec *ytv1.CommonSpec,
	path *field.Path,
) field.ErrorList {
	var allErrors field.ErrorList
	_ = oldInstanceSpec
	_ = oldCommonSpec

	allErrors = append(allErrors, r.validatePodSpec(&instanceSpec.PodSpec, path)...)

	allErrors = append(allErrors, validateStructuredLoggers(instanceSpec.StructuredLoggers, path)...)

	if instanceSpec.EnableAntiAffinity != nil {
		allErrors = append(allErrors, field.Invalid(path.Child("EnableAntiAffinity"), instanceSpec.EnableAntiAffinity,
			"EnableAntiAffinity is deprecated, use Affinity instead"))
	}

	allErrors = append(allErrors, r.validateTransportSecurity(instanceSpec.NativeTransport, commonSpec, path.Child("nativeTransport"))...)

	for mountIdx, volumeMount := range instanceSpec.VolumeMounts {
		if strings.HasSuffix(volumeMount.MountPath, "/") {
			allErrors = append(allErrors, field.Invalid(path.Child("volumeMounts").Index(mountIdx), volumeMount.MountPath,
				"mount path must not end with '/'"))
		}
		for _, previousMount := range instanceSpec.VolumeMounts[:mountIdx] {
			if previousMount.MountPath == volumeMount.MountPath ||
				strings.HasPrefix(previousMount.MountPath, volumeMount.MountPath+"/") {
				allErrors = append(allErrors, field.Invalid(path.Child("volumeMounts").Index(mountIdx), volumeMount.MountPath,
					fmt.Sprintf("volume mount completely covers previous volume mount %q", previousMount.MountPath)))
			}
		}
	}

	if instanceSpec.Locations != nil {
		for locationIdx, location := range instanceSpec.Locations {
			if strings.HasSuffix(location.Path, "/") {
				allErrors = append(allErrors, field.Invalid(path.Child("locations").Index(locationIdx), location,
					"location path must not end with '/'"))
			}
			if components.FindVolumeMountForPath(instanceSpec.VolumeMounts, location.Path) == nil {
				allErrors = append(allErrors, field.Invalid(path.Child("locations").Index(locationIdx), location,
					"location path is not in any volume mount"))
			}
		}
	}

	if oldInstanceSpec != nil {
		allErrors = append(allErrors, r.validateImageUpdate(instanceSpec.Image, oldInstanceSpec.Image, commonSpec, oldCommonSpec, path.Child("image"))...)
	}

	return allErrors
}

func (r *ytsaurusValidator) validateExistsYtsaurus(ctx context.Context, newYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	var ytsaurusList ytv1.YtsaurusList
	err := r.Client.List(ctx, &ytsaurusList, &client.ListOptions{Namespace: newYtsaurus.Namespace})
	r.Log.Info("validateExistsYTsaurus", "ytsaurusList", ytsaurusList)

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

func (r *baseValidator) validateCommonSpec(spec *ytv1.CommonSpec) field.ErrorList {
	var allErrors field.ErrorList
	path := field.NewPath("spec")

	if features := spec.ClusterFeatures; features != nil {
		if features.SecureClusterTransports {
			if !features.RPCProxyHavePublicAddress {
				allErrors = append(allErrors, field.Required(
					path.Child("clusterFeatures").Child("rpcProxyHavePublicAddress"),
					"Secure cluster transport demands public address for RPC proxies",
				))
			}
			if !features.HTTPProxyHaveHTTPSAddress {
				allErrors = append(allErrors, field.Required(
					path.Child("clusterFeatures").Child("httpProxyHaveHttpsAddress"),
					"Secure cluster transport demands HTTPS for HTTP proxies",
				))
			}
		}
	}

	allErrors = append(allErrors, validation.ValidateAnnotations(spec.ExtraPodAnnotations, path.Child("extraPodAnnotations"))...)
	allErrors = append(allErrors, r.validateTransportSecurity(spec.NativeTransport, spec, path.Child("nativeTransport"))...)

	return allErrors
}

func (r *baseValidator) validateUpdatePlan(newYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList
	planPath := field.NewPath("spec").Child("updatePlan")
	exclusiveClass := consts.ComponentClassUnspecified

	if newYtsaurus.Spec.UpdatePlan != nil {
		for i, entry := range newYtsaurus.Spec.UpdatePlan {
			entryPath := planPath.Index(i)
			if entry.Class != consts.ComponentClassUnspecified && (entry.Component.Type != "" || entry.Component.Name != "") {
				allErrors = append(allErrors, field.Invalid(entryPath.Child("component"), entry.Component, "Only one of component or class should be specified"))
			}
			if entry.Class == consts.ComponentClassUnspecified && entry.Component.Type == "" && entry.Component.Name == "" {
				allErrors = append(allErrors, field.Invalid(entryPath.Child("component"), entry.Component, "Either component or class should be specified"))
			}
			if entry.Component.Type != "" && !slices.Contains(consts.LocalComponentTypes, entry.Component.Type) {
				allErrors = append(allErrors, field.Invalid(entryPath.Child("component").Child("type"), entry.Component.Type, "Unknown component type"))
			}
			if exclusiveClass != consts.ComponentClassUnspecified {
				allErrors = append(allErrors, field.Invalid(entryPath, entry, fmt.Sprintf("Update plan already contains class %s", exclusiveClass)))
			}

			switch entry.Class {
			case consts.ComponentClassNothing, consts.ComponentClassEverything:
				exclusiveClass = entry.Class
				if i > 0 {
					allErrors = append(allErrors, field.Invalid(entryPath.Child("class"), entry.Class, "Should be first and only entry"))
				}
			case consts.ComponentClassStateless, consts.ComponentClassUnspecified:
			default:
				allErrors = append(allErrors, field.Invalid(entryPath.Child("class"), entry.Class, "Unknown class"))
			}

			allErrors = append(allErrors, validateUpdateModeForSelector(newYtsaurus, entry, entryPath.Child("updateMode"))...)
		}
	}

	return allErrors
}

func validateUpdateModeForSelector(newYtsaurus *ytv1.Ytsaurus, selector ytv1.ComponentUpdateSelector, path *field.Path) field.ErrorList {
	var errs field.ErrorList

	modeType := selector.GetUpdateStrategyType()
	bulkOnlyComponentTypes := map[consts.ComponentType]struct{}{
		consts.QueueAgentType:   {},
		consts.QueryTrackerType: {},
		consts.YqlAgentType:     {},
	}

	// strategy currently supported only for concrete components
	if selector.Class != consts.ComponentClassUnspecified && modeType != "" {
		errs = append(errs, field.Invalid(path.Child("strategy"), modeType, "strategy is supported only for specific components, not for component classes"))
		return errs
	}

	if selector.Class == consts.ComponentClassUnspecified {
		if selector.Component.Type == "" && selector.Strategy != nil {
			errs = append(errs, field.Invalid(path, selector.Strategy, "component.type must be set to use strategy"))
			return errs
		}
		// validate bulk-only restriction
		if _, bulkOnly := bulkOnlyComponentTypes[selector.Component.Type]; bulkOnly && modeType != "" && modeType != ytv1.ComponentUpdateModeTypeBulkUpdate {
			errs = append(errs, field.Invalid(path.Child("strategy"), modeType, fmt.Sprintf("%s supports only BulkUpdate mode", selector.Component.Type)))
			return errs
		}
	}

	downtime := ptr.Deref(newYtsaurus.Spec.ClusterMaintenance, ytv1.ClusterMaintenance{}).Downtime

	switch modeType {
	case ytv1.ComponentUpdateModeTypeBulkUpdate:
		if downtime == ytv1.ClusterDowntimeMinor {
			errs = append(errs, field.Forbidden(path, "Bulk update is incompatible with minor downtime"))
		}
		if selector.Strategy != nil {
			if selector.Strategy.RollingUpdate != nil {
				errs = append(errs, field.Invalid(path.Child("rollingUpdate"), selector.Strategy.RollingUpdate, "rolling configuration is not valid for BulkUpdate"))
			}
		}
	case ytv1.ComponentUpdateModeTypeRollingUpdate:
		if downtime == ytv1.ClusterDowntimeMajor {
			errs = append(errs, field.Forbidden(path, "Rolling update is incompatible with major downtime"))
		}
		if selector.Component.Type == "" {
			errs = append(errs, field.Invalid(path.Child("type"), modeType, "rolling update requires a concrete component selector"))
		}
		if selector.Component.Type == ytv1.DataNodeType && selector.Component.Name == "" && len(newYtsaurus.Spec.DataNodes) > 1 && selector.Concurrency == nil {
			errs = append(errs, field.Invalid(path.Child("concurrency"), modeType, "rolling update for several data node groups requires concurrency limit"))
		}

	case ytv1.ComponentUpdateModeTypeOnDelete:
		if downtime == ytv1.ClusterDowntimeMajor {
			errs = append(errs, field.Forbidden(path, "On-delete update is incompatible with major downtime"))
		}
		if selector.Component.Type == "" {
			errs = append(errs, field.Invalid(path.Child("type"), modeType, "onDelete update requires a concrete component selector"))
		}
	}

	return errs
}

func (r *ytsaurusValidator) validateYtsaurus(ctx context.Context, newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) field.ErrorList {
	var allErrors field.ErrorList

	if err := ValidateVersionConstraint(newYtsaurus.Spec.RequiresOperatorVersion); err != nil {
		allErrors = append(allErrors, err)
	}

	allErrors = append(allErrors, r.validateCommonSpec(&newYtsaurus.Spec.CommonSpec)...)
	allErrors = append(allErrors, r.validatePodSpec(&newYtsaurus.Spec.PodSpec, field.NewPath("spec"))...)
	allErrors = append(allErrors, r.validateDiscovery(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validatePrimaryMasters(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateSecondaryMasters(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateHTTPProxies(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateRPCProxies(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateTCPProxies(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateDataNodes(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateExecNodes(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateSchedulers(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateControllerAgents(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateTabletNodes(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateChyt(newYtsaurus)...)
	allErrors = append(allErrors, r.validateStrawberry(newYtsaurus)...)
	allErrors = append(allErrors, r.validateQueryTrackers(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateQueueAgents(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateSpyt(newYtsaurus)...)
	allErrors = append(allErrors, r.validateYQLAgents(newYtsaurus, oldYtsaurus)...)
	allErrors = append(allErrors, r.validateUi(newYtsaurus)...)
	allErrors = append(allErrors, r.validateExtraTimbertruckComponents(newYtsaurus)...)
	allErrors = append(allErrors, r.validateExistsYtsaurus(ctx, newYtsaurus)...)
	allErrors = append(allErrors, r.validateUpdatePlan(newYtsaurus)...)

	return allErrors
}

func (r *ytsaurusValidator) evaluateYtsaurusValidation(ctx context.Context, newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) (admission.Warnings, error) {
	if newYtsaurus == nil {
		return nil, nil
	}

	allErrors := r.validateYtsaurus(ctx, newYtsaurus, oldYtsaurus)
	if len(allErrors) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(ytv1.YtsaurusGVK.GroupKind(), newYtsaurus.Name, allErrors)
}
