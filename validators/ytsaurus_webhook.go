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
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/api/validation"
	v1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/ptr"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apihelpers"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"

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
	r.Validate = r.validateYtsaurus
	return r
}

func (r *baseValidator) validateTransportSecurity(spec *ytv1.RPCTransportSpec, commonSpec *ytv1.CommonSpec, path *field.Path) Results {
	var results Results

	features := ptr.Deref(commonSpec.ClusterFeatures, ytv1.ClusterFeatures{})

	if spec == nil {
		spec = commonSpec.NativeTransport
	}

	if spec == nil {
		if features.SecureClusterTransports {
			results.Error(field.Required(path, "Secure cluster transport demands TLS setup"))
		}
		return results
	}

	secretPath := path.Child("tlsSecret")
	clientSecretPath := path.Child("tlsClientSecret")
	if spec.TLSSecret == nil && (spec.TLSRequired || !spec.TLSInsecure) {
		results.Error(field.Required(secretPath, "TLS certificate for native transport is required"))
	}
	if spec.TLSClientSecret == nil && (spec.TLSRequired && !spec.TLSInsecure) {
		results.Error(field.Required(clientSecretPath, "Client TLS certificate for native transport is required"))
	}

	if features.SecureClusterTransports {
		if !spec.TLSRequired {
			results.Error(field.Forbidden(path.Child("tlsRequired"), "Secure cluster transport demands TLS-only native transport"))
		}
		if spec.TLSInsecure {
			results.Error(field.Forbidden(path.Child("tlsInsecure"), "Secure cluster transport demands TLS certificate validation"))
		}
		if spec.TLSSecret == nil {
			results.Error(field.Required(secretPath, "Secure cluster transport demands TLS certificate for native transport"))
		}
		if spec.TLSClientSecret == nil {
			results.Error(field.Required(clientSecretPath, "Secure cluster transport demands client TLS certificate for native transport"))
		}
	}

	return results
}

func (r *ytsaurusValidator) validateDiscovery(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	results.Append(r.validateInstanceSpec(newYtsaurus.Spec.Discovery.InstanceSpec, &newYtsaurus.Spec.CommonSpec, field.NewPath("spec").Child("discovery")))

	return results
}

//nolint:cyclop,nestif //ok
func (r *ytsaurusValidator) validateMasterSpec(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus, mastersSpec, oldMastersSpec *ytv1.MastersSpec, path *field.Path) Results {
	var results Results

	results.Append(r.validateInstanceSpecWithTimbertruck(mastersSpec.InstanceSpec, &newYtsaurus.Spec.CommonSpec, mastersSpec.Timbertruck, path))
	results.Append(r.validateHostAddresses(newYtsaurus, mastersSpec, path))

	if apihelpers.FindFirstLocation(mastersSpec.Locations, ytv1.LocationTypeMasterChangelogs) == nil {
		results.Error(field.NotFound(path.Child("locations"), ytv1.LocationTypeMasterChangelogs))
	}

	if apihelpers.FindFirstLocation(mastersSpec.Locations, ytv1.LocationTypeMasterSnapshots) == nil {
		results.Error(field.NotFound(path.Child("locations"), ytv1.LocationTypeMasterSnapshots))
	}

	if oldMastersSpec != nil && oldMastersSpec.CellTag != mastersSpec.CellTag {
		results.Error(field.Invalid(path.Child("cellTag"), mastersSpec.CellTag, "Could not be changed"))
	}

	rolesPath := path.Child("roles")
	isPrimary := mastersSpec.CellTag == newYtsaurus.Spec.PrimaryMasters.CellTag
	isMulticell := len(newYtsaurus.Spec.SecondaryMasters) > 0
	cellRoles := UniqueValues[ytv1.MasterCellRole]{}
	results.Append(cellRoles.InsertAll(ytv1.GetMasterCellRoles(mastersSpec.Roles, isPrimary, isMulticell), rolesPath))
	cellRolesChanged := false

	if cellRoles.Count(ytv1.MasterCellRoleChunkHost, ytv1.MasterCellRoleDedicatedChunkHost) > 1 {
		results.Error(field.Forbidden(rolesPath, fmt.Sprintf("Master cell %v has role conflict: %v %v",
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
					results.Error(field.Forbidden(rolesPath, fmt.Sprintf("Cell %v role could not be removed: %v", mastersSpec.CellTag, role)))
				}
			}
		}
		if isPrimary && isMulticell && !wasMulticell && mastersSpec.Roles == nil {
			results.Error(field.Required(rolesPath, "Upgrade to multicell requires filling roles for primary cell"))
		}
	}

	newInstanceCount := mastersSpec.InstanceCount
	newMinReady := min(newInstanceCount, ptr.Deref(mastersSpec.MinReadyInstanceCount, newInstanceCount))
	if newInstanceCount > 0 && newMinReady <= newInstanceCount/2 && !newYtsaurus.Spec.EphemeralCluster {
		results.Error(field.Invalid(path.Child("minReadyInstanceCount"), newMinReady, "Must be bigger than half of instanceCount"))
	}
	if newMinReady > newInstanceCount {
		results.Error(field.Invalid(path.Child("minReadyInstanceCount"), newMinReady, "Cannot be bigger than instanceCount"))
	}

	instanceCountPath := path.Child("instanceCount")
	if oldMastersSpec == nil {
		if oldYtsaurus != nil && !isPrimary && newInstanceCount != 0 {
			results.Error(field.Forbidden(instanceCountPath, "New secondary master cell initially must have instanceCount = 0"))
		}
	} else {
		oldInstanceCount := oldMastersSpec.InstanceCount
		oldMinReady := min(oldInstanceCount, ptr.Deref(oldMastersSpec.MinReadyInstanceCount, oldInstanceCount))
		if !isPrimary && newInstanceCount != 0 && oldInstanceCount == 0 {
			if !slices.ContainsFunc(newYtsaurus.Status.UpdateStatus.MasterCellsMaintenance, func(info ytv1.MasterCellMaintenanceInfo) bool {
				return info.CellTag == mastersSpec.CellTag && info.Unregistered
			}) {
				results.Error(field.Invalid(instanceCountPath, mastersSpec.InstanceCount, "Master cell must be unregistered"))
			}
		}
		if newInstanceCount < 1 && oldInstanceCount > 0 {
			results.Error(field.Invalid(instanceCountPath, mastersSpec.InstanceCount, "Cannot remove last instance"))
		}
		if newInstanceCount/2 < oldInstanceCount-oldMinReady {
			results.Error(field.Invalid(instanceCountPath, mastersSpec.InstanceCount,
				"Cannot shrink without possibility of losing quorum, increase minReadyInstanceCount first"))
		}
		if newInstanceCount > oldMinReady*2-1 && oldInstanceCount > 1 {
			results.Error(field.Invalid(instanceCountPath, mastersSpec.InstanceCount,
				fmt.Sprintf("Cannot grow bigger than previous minReadyInstanceCount*2-1 (%d) in one step", oldMinReady*2-1)))
		}

		oldMaintenance := ptr.Deref(oldYtsaurus.Spec.ClusterMaintenance, ytv1.ClusterMaintenance{})
		newMaintenance := ptr.Deref(newYtsaurus.Spec.ClusterMaintenance, ytv1.ClusterMaintenance{})
		if oldMaintenance.Shutdown != ytv1.ClusterShutdownExceptMasters || newMaintenance.Shutdown != ytv1.ClusterShutdownExceptMasters {
			if oldInstanceCount != newInstanceCount {
				results.Error(field.Forbidden(instanceCountPath, "Could be changed only during master cells maintenance"))
			}
			if cellRolesChanged {
				results.Error(field.Forbidden(rolesPath, "Could be changed only during master cells maintenance"))
			}
		}
		if oldInstanceCount > 0 && cellRolesChanged && (oldMaintenance.AssignMasterCellsRoles || !newMaintenance.AssignMasterCellsRoles) {
			results.Error(field.Forbidden(rolesPath, "Could be changed only together with enabling assignMasterCellsRoles"))
		}
	}

	if mastersSpec.InstanceCount > 1 && !newYtsaurus.Spec.EphemeralCluster {
		if affinity := mastersSpec.Affinity; affinity == nil || affinity.PodAntiAffinity == nil {
			results.Error(field.Required(path.Child("affinity").Child("podAntiAffinity"),
				"Masters should be placed on different nodes"))
		}
	}

	results.Append(r.validateHydraPersistenceUploaderSpec(mastersSpec.HydraPersistenceUploader, mastersSpec.Locations, path))

	results.Append(r.validateSidecars(mastersSpec.Sidecars, path.Child("sidecars")))

	return results
}

func (r *ytsaurusValidator) validatePrimaryMasters(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	mastersSpec := &newYtsaurus.Spec.PrimaryMasters
	path := field.NewPath("spec").Child("primaryMasters")

	var oldMastersSpec *ytv1.MastersSpec
	if oldYtsaurus != nil {
		oldMastersSpec = &oldYtsaurus.Spec.PrimaryMasters
	}

	results.Append(r.validateMasterSpec(newYtsaurus, oldYtsaurus, mastersSpec, oldMastersSpec, path))

	if mastersSpec.InstanceCount < 1 {
		results.Error(field.Invalid(path.Child("instanceCount"), mastersSpec.InstanceCount, "Cannot be below 1"))
	}

	return results
}

func (r *ytsaurusValidator) validateSecondaryMasters(newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	cellTags := UniqueValues[uint16]{}
	cellTags.Insert(newYtsaurus.Spec.PrimaryMasters.CellTag, field.NewPath("spec").Child("primaryMasters").Child("cellTag"))

	// TODO(khlebnikov): Add option for RPC port to collocate primary and secondary masters in host network.
	hostAddresses := UniqueValues[string]{}
	results.Append(hostAddresses.InsertAll(newYtsaurus.Spec.PrimaryMasters.HostAddresses, field.NewPath("spec").Child("primaryMasters").Child("hostAddresses")))

	secondaryMastersPath := field.NewPath("spec").Child("secondaryMasters")
	for i := range newYtsaurus.Spec.SecondaryMasters {
		path := secondaryMastersPath.Index(i)
		mastersSpec := &newYtsaurus.Spec.SecondaryMasters[i]
		var oldMastersSpec *ytv1.MastersSpec
		if oldYtsaurus != nil && len(oldYtsaurus.Spec.SecondaryMasters) > i {
			oldMastersSpec = &oldYtsaurus.Spec.SecondaryMasters[i]
		}
		results.Append(r.validateMasterSpec(newYtsaurus, oldYtsaurus, mastersSpec, oldMastersSpec, path))
		results.Append(cellTags.Insert(mastersSpec.CellTag, path.Child("cellTag")))
		results.Append(hostAddresses.InsertAll(mastersSpec.HostAddresses, path.Child("hostAddresses")))
	}

	if cnt := len(cellTags) - 1; cnt > consts.MaxSecondaryMasterCells {
		results.Error(field.TooMany(secondaryMastersPath, cnt, consts.MaxSecondaryMasterCells))
	}
	for cellTag, path := range cellTags {
		if cellTag < consts.MinValidCellTag || cellTag > consts.MaxValidCellTag {
			results.Error(field.Invalid(path, cellTag, fmt.Sprintf("Cell tag must be in range %v..%v", consts.MinValidCellTag, consts.MaxValidCellTag)))
		}
	}

	if oldYtsaurus != nil {
		// It is ok to remove unborn secondary cells without instances from the end of the list.
		for i := len(newYtsaurus.Spec.SecondaryMasters); i < len(oldYtsaurus.Spec.SecondaryMasters); i++ {
			oldMastersSpec := &oldYtsaurus.Spec.SecondaryMasters[i]
			if oldMastersSpec.InstanceCount > 0 {
				path := field.NewPath("spec").Child("secondaryMasters").Index(i)
				results.Error(field.Forbidden(path, "Cannot remove cell with instances"))
			}
		}
	}

	return results
}

func (r *ytsaurusValidator) validateHostAddresses(newYtsaurus *ytv1.Ytsaurus, mastersSpec *ytv1.MastersSpec, fieldPath *field.Path) Results {
	var results Results

	hostAddressesFieldPath := fieldPath.Child("hostAddresses")
	if !ptr.Deref(mastersSpec.HostNetwork, ptr.Deref(newYtsaurus.Spec.HostNetwork, false)) && len(mastersSpec.HostAddresses) != 0 {
		results.Error(field.Required(field.NewPath("spec").Child("hostNetwork"),
			fmt.Sprintf("%s doesn't make sense without hostNetwork=true", hostAddressesFieldPath.String())))
	}

	if len(mastersSpec.HostAddresses) != 0 && len(mastersSpec.HostAddresses) != int(mastersSpec.InstanceCount) {
		instanceCountFieldPath := fieldPath.Child("instanceCount")
		results.Error(field.Invalid(hostAddressesFieldPath, mastersSpec.HostAddresses,
			fmt.Sprintf("%s list length should be equal to %s", hostAddressesFieldPath.String(), instanceCountFieldPath.String())))
	}

	return results
}

func (r *ytsaurusValidator) validateHTTPProxies(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	features := ptr.Deref(newYtsaurus.Spec.ClusterFeatures, ytv1.ClusterFeatures{})
	httpRoles := make(map[string]bool)
	hasDefaultHTTPProxy := false
	for i, hp := range newYtsaurus.Spec.HTTPProxies {
		path := field.NewPath("spec").Child("httpProxies").Index(i)
		if _, exists := httpRoles[hp.Role]; exists {
			results.Error(field.Duplicate(path.Child("role"), hp.Role))
		}
		if hp.Role == consts.DefaultHTTPProxyRole {
			hasDefaultHTTPProxy = true
		}
		httpRoles[hp.Role] = true

		results.Append(r.validateInstanceSpec(hp.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))

		if features.HTTPProxyHaveHTTPSAddress && hp.Transport.HTTPSSecret == nil {
			results.Error(field.Required(
				path.Child("transport").Child("httpsSecret"),
				"Cluster feature httpProxyHaveHttpsAddress requires HTTPS for all HTTP proxies",
			))
		}

		if features.SecureClusterTransports && !hp.Transport.DisableHTTP {
			results.Error(field.Forbidden(
				path.Child("transport").Child("disableHttp"),
				"Secure cluster transport demands HTTPS-only proxies",
			))
		}
	}

	if !hasDefaultHTTPProxy {
		results.Error(field.Required(
			field.NewPath("spec").Child("httpProxies"),
			fmt.Sprintf("HTTP proxy with `%s` role should exist", consts.DefaultHTTPProxyRole)))
	}

	return results
}

func (r *ytsaurusValidator) validateRPCProxies(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	features := ptr.Deref(newYtsaurus.Spec.ClusterFeatures, ytv1.ClusterFeatures{})
	rpcRoles := make(map[string]bool)
	for i, rp := range newYtsaurus.Spec.RPCProxies {
		path := field.NewPath("spec").Child("rpcProxies").Index(i)
		if _, exists := rpcRoles[rp.Role]; exists {
			results.Error(field.Duplicate(path.Child("role"), rp.Role))
		}
		rpcRoles[rp.Role] = true

		results.Append(r.validateInstanceSpec(rp.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))

		transportPath := path.Child("transport")
		if rp.Transport.TLSRequired && rp.Transport.TLSSecret == nil {
			results.Error(field.Required(transportPath.Child("tlsSecret"), "TLS-only RPC proxy requires certificate"))
		}

		if features.SecureClusterTransports {
			if !rp.Transport.TLSRequired {
				results.Error(field.Required(transportPath.Child("tlsRequired"), "Secure cluster transport demands TLS-only RPC proxies"))
			}
			if rp.Transport.TLSInsecure {
				results.Error(field.Forbidden(transportPath.Child("tlsInsecure"), "Secure cluster transport demands TLS certificate validation"))
			}
			if rp.Transport.TLSSecret == nil {
				results.Error(field.Required(transportPath.Child("tlsSecret"), "Secure cluster transport demands RPC proxy certificate"))
			}
		}
	}

	return results
}

func (r *ytsaurusValidator) validateTCPProxies(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	tcpRoles := make(map[string]bool)
	for i, rp := range newYtsaurus.Spec.TCPProxies {
		path := field.NewPath("spec").Child("tcpProxies").Index(i)
		if _, exists := tcpRoles[rp.Role]; exists {
			results.Error(field.Duplicate(path.Child("role"), rp.Role))
		}
		tcpRoles[rp.Role] = true

		results.Append(r.validateInstanceSpec(rp.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))
	}

	return results
}

func (r *ytsaurusValidator) validateDataNodes(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	names := make(map[string]bool)
	for i, dn := range newYtsaurus.Spec.DataNodes {
		path := field.NewPath("spec").Child("dataNodes").Index(i)

		if _, exists := names[dn.Name]; exists {
			results.Error(field.Duplicate(path.Child("name"), dn.Name))
		}
		names[dn.Name] = true

		results.Append(r.validateInstanceSpec(dn.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))

		if apihelpers.FindFirstLocation(dn.Locations, ytv1.LocationTypeChunkStore) == nil {
			results.Error(field.NotFound(path.Child("locations"), ytv1.LocationTypeChunkStore))
		}
	}

	return results
}

func (r *baseValidator) validateHydraPersistenceUploaderSpec(
	hydraPersistenceUploader *ytv1.HydraPersistenceUploaderSpec, locations []ytv1.LocationSpec, parentPath *field.Path) Results {
	var results Results

	if hydraPersistenceUploader == nil {
		return results
	}

	if hydraPersistenceUploader.Image == nil {
		results.Error(field.Required(parentPath.Child("hydraPersistenceUploader", "image"), "hydraPersistenceUploader image is required"))
	}

	results.Append(requireLocations(
		parentPath.Child("locations"),
		locations,
		"hydraPersistenceUploader",
		ytv1.LocationTypeMasterSnapshots,
		ytv1.LocationTypeMasterChangelogs,
	))

	return results
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
) Results {
	var results Results

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
		return results
	}

	hasImage := func(tt *ytv1.TimbertruckSpec) bool {
		return tt != nil && tt.Image != nil && *tt.Image != ""
	}
	if !hasImage(componentTimbertruck) && !hasImage(commonTimbertruck) {
		results.Error(field.Required(parentPath.Child("timbertruck", "image"),
			"timbertruck image is required (set it here or in spec.timbertruck.image) when log delivery is enabled"))
	}
	if legacyMaster && len(structuredLoggers) == 0 {
		results.Error(field.Required(parentPath.Child("structuredLoggers"),
			"structuredLoggers must be configured when timbertruck is enabled"))
	}
	results.Append(requireLocations(
		parentPath.Child("locations"),
		locations,
		"timbertruck",
		ytv1.LocationTypeLogs,
	))
	return results
}

func validateLogRotationPolicy(policy *ytv1.LogRotationPolicy, path *field.Path) Results {
	var results Results

	if policy == nil {
		results.Warning(field.Required(path, "logs are not rotated by default"))
	} else if policy.MaxTotalSizeToKeep == nil {
		results.Warning(field.Required(path.Child("maxTotalSizeToKeep"), "total size limit is required"))
	} else if policy.MaxSegmentSize == nil {
		results.Warning(field.Required(path.Child("maxSegmentSize"), "maxTotalSizeToKeep does not work correctly without maxSegmentSize"))
	} else if policy.MaxSegmentSize.Cmp(*policy.MaxTotalSizeToKeep) > 0 {
		results.Warning(field.Invalid(path.Child("maxSegmentSize"), *policy.MaxSegmentSize, "bigger than maxTotalSizeToKeep"))
	} else if policy.MaxSegmentCountToKeep != nil && policy.MaxTotalSizeToKeep.Value() > *policy.MaxSegmentCountToKeep*policy.MaxSegmentSize.Value() {
		results.Warning(field.Invalid(path.Child("maxSegmentCountToKeep"), *policy.MaxSegmentCountToKeep, "lower than maxTotalSizeToKeep / maxSegmentSize"))
	}

	if policy != nil && policy.RotationPeriodMilliseconds != nil {
		if policy.MaxSegmentCountToKeep == nil {
			results.Warning(field.Required(path.Child("maxSegmentCountToKeep"), "rotationPeriodMilliseconds could swarm files without maxSegmentCountToKeep"))
		} else if *policy.MaxSegmentCountToKeep**policy.RotationPeriodMilliseconds < (time.Hour * 24).Milliseconds() {
			results.Warning(field.Invalid(path.Child("maxSegmentCountToKeep"), *policy.MaxSegmentCountToKeep, "maxSegmentCountToKeep * rotationPeriodMilliseconds < 24 hours"))
		}
	}

	return results
}

func validateLoggers(loggers []ytv1.TextLoggerSpec, path *field.Path, quota *resource.Quantity) Results {
	var results Results
	var hasWarningLevel bool
	var maxLogsTotalSizes []*resource.Quantity
	for i, logger := range loggers {
		loggerPath := path.Index(i)
		if logger.UseDateTimeSuffix && logger.UseTimestampSuffix {
			results.Error(field.Forbidden(loggerPath.Child("useDateTimeSuffix"), "must be one of useDateTimeSuffix or useTimestampSuffix"))
		}
		if logger.WriterType == ytv1.LogWriterTypeFile {
			results.Append(validateLogRotationPolicy(logger.RotationPolicy, loggerPath.Child("rotationPolicy")))
			if !logger.UseDateTimeSuffix {
				results.Warning(field.Required(loggerPath.Child("useDateTimeSuffix"), "useDateTimeSuffix is recommended"))
			}
		} else if logger.RotationPolicy != nil {
			results.Warning(field.Forbidden(loggerPath.Child("rotationPolicy"), "only for file writer"))
		}
		switch logger.MinLogLevel {
		case ytv1.LogLevelTrace, ytv1.LogLevelDebug:
			if logger.CategoriesFilter == nil ||
				(logger.CategoriesFilter.Type == ytv1.CategoriesFilterTypeExclude &&
					(!slices.Contains(logger.CategoriesFilter.Values, "Bus") || !slices.Contains(logger.CategoriesFilter.Values, "Concurrency"))) {
				results.Warning(field.Required(loggerPath.Child("categoriesFilter"), "debug level categories Bus and Concurrency are noisy as trace level"))
			}
			if logger.Compression == ytv1.LogCompressionNone || logger.Compression == "" {
				results.Warning(field.Required(loggerPath.Child("compression"), "compression is recommended for debug and trace levels"))
			}
			fallthrough
		case ytv1.LogLevelInfo, ytv1.LogLevelWarning:
			hasWarningLevel = true
		}
		if logger.WriterType == ytv1.LogWriterTypeFile {
			maxLogsTotalSizes = append(maxLogsTotalSizes, apihelpers.MaxLogsTotalSize(logger.RotationPolicy))
		}
	}
	if len(loggers) > 0 && !hasWarningLevel {
		results.Warning(field.Required(path, "at least warning level logging is recommended"))
	}
	if quota != nil {
		maxLogsTotalSize := apihelpers.SumQuantity(maxLogsTotalSizes...)
		if maxLogsTotalSize == nil || maxLogsTotalSize.Cmp(*quota) > 0 {
			results.Warning(field.Invalid(path, loggers, fmt.Sprintf("logs disk usage %v could exceed quota %v", maxLogsTotalSize, quota)))
		}
	}
	return results
}

func validateStructuredLoggers(structuredLoggers []ytv1.StructuredLoggerSpec, parentPath *field.Path) Results {
	var results Results

	for loggerIdx, logger := range structuredLoggers {
		loggerPath := parentPath.Child("structuredLoggers").Index(loggerIdx)

		if (logger.Category != nil) == (logger.CategoriesFilter != nil) {
			results.Error(field.Invalid(loggerPath, logger.Name,
				"exactly one of category and categoriesFilter must be set"))
		}
		if logger.Category != nil && *logger.Category == "" {
			results.Error(field.Required(loggerPath.Child("category"),
				"category must not be empty"))
		}
		// Without both, the rule ends up with no category restriction and matches every category.
		if filter := logger.CategoriesFilter; filter != nil && (filter.Type == "" || len(filter.Values) == 0) {
			results.Error(field.Required(loggerPath.Child("categoriesFilter"),
				"categoriesFilter requires type and values"))
		}
	}

	return results
}

func requireLocations(
	locationsPath *field.Path,
	locations []ytv1.LocationSpec,
	reason string,
	locationTypes ...ytv1.LocationType,
) Results {
	var results Results
	for _, locationType := range locationTypes {
		if apihelpers.FindFirstLocation(locations, locationType) == nil {
			results.Error(field.Required(
				locationsPath,
				fmt.Sprintf("%s location must be configured for %s", locationType, reason),
			))
		}
	}
	return results
}

func (r *ytsaurusValidator) validateMasterCaches(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus.Spec.MasterCaches != nil {
		path := field.NewPath("spec").Child("masterCaches")
		results.Append(r.validateInstanceSpec(newYtsaurus.Spec.MasterCaches.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))
	}

	return results
}

func (r *ytsaurusValidator) validateKafkaProxies(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	kafkaRoles := make(map[string]bool)
	for i, kp := range newYtsaurus.Spec.KafkaProxies {
		path := field.NewPath("spec").Child("kafkaProxies").Index(i)
		if _, exists := kafkaRoles[kp.Role]; exists {
			results.Error(field.Duplicate(path.Child("role"), kp.Role))
		}
		kafkaRoles[kp.Role] = true

		results.Append(r.validateInstanceSpec(kp.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))
	}

	return results
}

func (r *ytsaurusValidator) validateCypressProxies(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus.Spec.CypressProxies != nil {
		path := field.NewPath("spec").Child("cypressProxies")
		results.Append(r.validateInstanceSpec(newYtsaurus.Spec.CypressProxies.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))
	}

	return results
}

func (r *ytsaurusValidator) validateBundleController(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus.Spec.BundleController != nil {
		path := field.NewPath("spec").Child("bundleController")
		results.Append(r.validateInstanceSpec(newYtsaurus.Spec.BundleController.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))
	}

	return results
}

func (r *ytsaurusValidator) validateTabletBalancer(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus.Spec.TabletBalancer != nil {
		path := field.NewPath("spec").Child("tabletBalancers")
		results.Append(r.validateInstanceSpec(newYtsaurus.Spec.TabletBalancer.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))
	}

	return results
}

func (r *baseValidator) validateSidecars(sidecars []string, path *field.Path) Results {
	var results Results

	names := make(map[string]bool)
	for i, sidecarSpec := range sidecars {
		sidecar := corev1.Container{}
		if err := yaml.UnmarshalStrict([]byte(sidecarSpec), &sidecar); err != nil {
			results.Error(field.Invalid(path.Index(i), sidecarSpec, err.Error()))
		}
		if _, exists := names[sidecar.Name]; exists {
			results.Error(field.Duplicate(path.Index(i).Child("name"), sidecar.Name))
		}
		names[sidecar.Name] = true
	}

	return results
}

func (r *ytsaurusValidator) validateExecNodes(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	names := make(map[string]bool)
	for i, en := range newYtsaurus.Spec.ExecNodes {
		path := field.NewPath("spec").Child("execNodes").Index(i)

		if _, exists := names[en.Name]; exists {
			results.Error(field.Duplicate(path.Child("name"), en.Name))
		}
		names[en.Name] = true

		results.Append(r.validateInstanceSpec(en.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))

		if apihelpers.FindFirstLocation(en.Locations, ytv1.LocationTypeChunkCache) == nil {
			results.Error(field.NotFound(path.Child("locations"), ytv1.LocationTypeChunkCache))
		}

		if apihelpers.FindFirstLocation(en.Locations, ytv1.LocationTypeSlots) == nil {
			results.Error(field.NotFound(path.Child("locations"), ytv1.LocationTypeSlots))
		}

		// TODO: Think about job proxy logs quota estimation.
		results.Append(validateLoggers(en.JobProxyLoggers, path.Child("jobProxyLoggers"), nil))

		if en.InitContainers != nil {
			results.Append(r.validateSidecars(en.InitContainers, path.Child("initContainers")))
		}
		if en.Sidecars != nil {
			results.Append(r.validateSidecars(en.Sidecars, path.Child("sidecars")))
		}
	}

	if len(newYtsaurus.Spec.ExecNodes) > 0 {
		if newYtsaurus.Spec.Schedulers == nil {
			results.Error(field.Required(field.NewPath("spec").Child("schedulers"),
				"execNodes doesn't make sense without schedulers"))
		}
	}

	return results
}

func (r *ytsaurusValidator) validateSchedulers(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus.Spec.Schedulers != nil {
		path := field.NewPath("spec").Child("schedulers")
		results.Append(r.validateInstanceSpec(newYtsaurus.Spec.Schedulers.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))

		if newYtsaurus.Spec.ControllerAgents == nil {
			results.Error(field.Required(field.NewPath("spec").Child("controllerAgents"),
				"schedulers doesn't make sense without controllerAgents"))
		}
	}

	return results
}

func (r *ytsaurusValidator) validateControllerAgents(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus.Spec.ControllerAgents != nil {
		path := field.NewPath("spec").Child("controllerAgents")
		results.Append(r.validateInstanceSpec(newYtsaurus.Spec.ControllerAgents.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))

		if newYtsaurus.Spec.Schedulers == nil {
			results.Error(field.Required(field.NewPath("spec").Child("schedulers"),
				"controllerAgents doesn't make sense without schedulers"))
		}
	}

	return results
}

func (r *ytsaurusValidator) validateTabletNodes(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	names := make(map[string]bool)
	for i, tn := range newYtsaurus.Spec.TabletNodes {
		path := field.NewPath("spec").Child("tabletNodes").Index(i)

		if _, exists := names[tn.Name]; exists {
			results.Error(field.Duplicate(path.Child("name"), tn.Name))
		}
		names[tn.Name] = true

		results.Append(r.validateInstanceSpec(tn.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))
	}

	return results
}

func (r *ytsaurusValidator) validateChyt(_ *ytv1.Ytsaurus) Results {
	var results Results

	return results
}

func (r *ytsaurusValidator) validateStrawberry(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus.Spec.StrawberryController != nil {
		path := field.NewPath("spec").Child("strawberry")
		results.Append(r.validatePodSpec(&newYtsaurus.Spec.StrawberryController.PodSpec, path))

		if newYtsaurus.Spec.Schedulers == nil {
			results.Error(field.Required(field.NewPath("spec").Child("schedulers"),
				"schedulers are required for strawberry"))
		}
	}

	return results
}

func (r *ytsaurusValidator) validateQueryTrackers(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus.Spec.QueryTrackers != nil {
		path := field.NewPath("spec").Child("queryTrackers")
		results.Append(r.validateInstanceSpec(newYtsaurus.Spec.QueryTrackers.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))

		if len(newYtsaurus.Spec.TabletNodes) == 0 {
			results.Error(field.Required(field.NewPath("spec").Child("tabletNodes"),
				"tabletNodes are required for queryTrackers"))
		}

		if newYtsaurus.Spec.Schedulers == nil {
			results.Error(field.Required(field.NewPath("spec").Child("schedulers"),
				"schedulers are required for queryTrackers"))
		}
	}

	return results
}

func (r *ytsaurusValidator) validateQueueAgents(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus.Spec.QueueAgents != nil {
		path := field.NewPath("spec").Child("queueAgents")
		results.Append(r.validateInstanceSpec(newYtsaurus.Spec.QueueAgents.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))

		if len(newYtsaurus.Spec.TabletNodes) == 0 {
			results.Error(field.Required(field.NewPath("spec").Child("tabletNodes"),
				"tabletNodes are required for queueAgents"))
		}
	}

	return results
}

func (r *ytsaurusValidator) validateSpyt(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results
	path := field.NewPath("spec").Child("spyt")

	if newYtsaurus.Spec.Spyt != nil {
		results.Error(field.Invalid(path, newYtsaurus.Spec.Spyt,
			"spyt is deprecated here, use Spyt resource instead"))
	}

	return results
}

func (r *ytsaurusValidator) validateYQLAgents(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus.Spec.YQLAgents != nil {
		path := field.NewPath("spec").Child("yqlAgents")
		results.Append(r.validateInstanceSpec(newYtsaurus.Spec.YQLAgents.InstanceSpec, &newYtsaurus.Spec.CommonSpec, path))

		if newYtsaurus.Spec.QueryTrackers == nil {
			results.Error(field.Required(field.NewPath("spec").Child("queryTrackers"),
				"yqlAgents doesn't make sense without queryTrackers"))
		}
	}

	return results
}

func (r *ytsaurusValidator) validateUi(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus.Spec.UI != nil {
		path := field.NewPath("spec").Child("ui")
		results.Append(r.validatePodSpec(&newYtsaurus.Spec.UI.PodSpec, path))
	}

	if newYtsaurus.Spec.UI != nil && newYtsaurus.Spec.UI.Secure {
		for i, hp := range newYtsaurus.Spec.HTTPProxies {
			if hp.Role != consts.DefaultHTTPProxyRole {
				continue
			}
			if hp.Transport.HTTPSSecret == nil {
				results.Error(field.Required(
					field.NewPath("spec", "httpProxies").Index(i).Child("transport", "httpsSecret"),
					fmt.Sprintf("configured HTTPS for proxy with `%s` role is required for ui.secure", consts.DefaultHTTPProxyRole)))
			}
			break
		}
	}

	return results
}

func (r *baseValidator) validatePodSpec(podSpec *ytv1.PodSpec, path *field.Path) Results {
	var results Results

	results.AppendErrors(v1validation.ValidateLabels(podSpec.PodLabels, path.Child("podLabels")))
	results.AppendErrors(validation.ValidateAnnotations(podSpec.PodAnnotations, path.Child("podAnnotations")))

	return results
}

func (r *baseValidator) validateInstanceSpec(instanceSpec ytv1.InstanceSpec, commonSpec *ytv1.CommonSpec, path *field.Path) Results {
	return r.validateInstanceSpecWithTimbertruck(instanceSpec, commonSpec, nil, path)
}

func (r *baseValidator) validateInstanceSpecWithTimbertruck(instanceSpec ytv1.InstanceSpec, commonSpec *ytv1.CommonSpec, componentTimbertruck *ytv1.TimbertruckSpec, path *field.Path) Results {
	var results Results

	results.Append(r.validatePodSpec(&instanceSpec.PodSpec, path))

	results.Append(r.validateTimbertruckSpec(componentTimbertruck, commonSpec.Timbertruck, instanceSpec.StructuredLoggers, instanceSpec.Locations, path))

	var logsQuota *resource.Quantity
	if location := apihelpers.FindFirstLocation(instanceSpec.Locations, ytv1.LocationTypeLogs); location != nil {
		logsQuota, _ = apihelpers.FindQuotaForLocation(location, &instanceSpec)
	}
	results.Append(validateLoggers(instanceSpec.Loggers, path.Child("loggers"), logsQuota))

	results.Append(validateStructuredLoggers(instanceSpec.StructuredLoggers, path))

	if instanceSpec.EnableAntiAffinity != nil {
		results.Error(field.Invalid(path.Child("EnableAntiAffinity"), instanceSpec.EnableAntiAffinity,
			"EnableAntiAffinity is deprecated, use Affinity instead"))
	}

	results.Append(r.validateTransportSecurity(instanceSpec.NativeTransport, commonSpec, path.Child("nativeTransport")))

	for mountIdx, volumeMount := range instanceSpec.VolumeMounts {
		if strings.HasSuffix(volumeMount.MountPath, "/") {
			results.Error(field.Invalid(path.Child("volumeMounts").Index(mountIdx), volumeMount.MountPath,
				"mount path must not end with '/'"))
		}
		for _, previousMount := range instanceSpec.VolumeMounts[:mountIdx] {
			if previousMount.MountPath == volumeMount.MountPath ||
				strings.HasPrefix(previousMount.MountPath, volumeMount.MountPath+"/") {
				results.Error(field.Invalid(path.Child("volumeMounts").Index(mountIdx), volumeMount.MountPath,
					fmt.Sprintf("volume mount completely covers previous volume mount %q", previousMount.MountPath)))
			}
		}
	}

	if instanceSpec.Locations != nil {
		singletonTypes := []ytv1.LocationType{
			ytv1.LocationTypeLogs,
			ytv1.LocationTypeMasterChangelogs,
			ytv1.LocationTypeMasterSnapshots,
			ytv1.LocationTypeImageCache,
			ytv1.LocationTypeInitJobLogs,
		}
		seenTypes := map[ytv1.LocationType]bool{}
		for locationIdx, location := range instanceSpec.Locations {
			locationPath := path.Child("locations").Index(locationIdx)
			if strings.HasSuffix(location.Path, "/") {
				results.Error(field.Invalid(locationPath.Child("path"), location.Path, "location path must not end with '/'"))
			}
			if apihelpers.FindVolumeMountForPath(instanceSpec.VolumeMounts, location.Path) == nil {
				results.Error(field.Invalid(locationPath.Child("path"), location.Path, "location path is not in any volume mount"))
			}
			if slices.Contains(singletonTypes, location.LocationType) {
				if seenTypes[location.LocationType] {
					results.Error(field.Invalid(locationPath.Child("locationType"), location.LocationType, "duplicate location type"))
				}
			}
			seenTypes[location.LocationType] = true
		}
	}

	return results
}

func (r *ytsaurusValidator) validateExistsYtsaurus(ctx context.Context, newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	var ytsaurusList ytv1.YtsaurusList
	err := r.Client.List(ctx, &ytsaurusList, &client.ListOptions{Namespace: newYtsaurus.Namespace})
	r.Log.Info("validateExistsYTsaurus", "ytsaurusList", ytsaurusList)

	if err != nil && !apierrors.IsNotFound(err) {
		results.Error(field.InternalError(field.NewPath("k8sClient"), err))
		return results
	}

	// if ytsaurus is already exists and it's the same one, it is an update operation
	if len(ytsaurusList.Items) == 1 && ytsaurusList.Items[0].Name == newYtsaurus.Name {
		return results
	} else if len(ytsaurusList.Items) == 0 {
		// it's the creation operation for the first ytsaurus object
		return results
	} else {
		results.Error(field.Forbidden(field.NewPath("metadata").Child("namespace"),
			fmt.Sprintf("A Ytsaurus object already exists in the given namespace %s", newYtsaurus.Namespace)))
	}

	return results
}

func (r *baseValidator) validateCommonSpec(spec *ytv1.CommonSpec) Results {
	var results Results
	path := field.NewPath("spec")

	if features := spec.ClusterFeatures; features != nil {
		if features.SecureClusterTransports {
			if !features.RPCProxyHavePublicAddress {
				results.Error(field.Required(
					path.Child("clusterFeatures").Child("rpcProxyHavePublicAddress"),
					"Secure cluster transport demands public address for RPC proxies",
				))
			}
			if !features.HTTPProxyHaveHTTPSAddress {
				results.Error(field.Required(
					path.Child("clusterFeatures").Child("httpProxyHaveHttpsAddress"),
					"Secure cluster transport demands HTTPS for HTTP proxies",
				))
			}
		}
	}

	results.AppendErrors(validation.ValidateAnnotations(spec.ExtraPodAnnotations, path.Child("extraPodAnnotations")))
	results.Append(r.validateTransportSecurity(spec.NativeTransport, spec, path.Child("nativeTransport")))

	return results
}

func (r *baseValidator) validateUpdatePlan(newYtsaurus *ytv1.Ytsaurus) Results {
	var results Results
	planPath := field.NewPath("spec").Child("updatePlan")
	exclusiveClass := consts.ComponentClassUnspecified

	if newYtsaurus.Spec.UpdatePlan != nil {
		for i, entry := range newYtsaurus.Spec.UpdatePlan {
			entryPath := planPath.Index(i)
			if entry.Class != consts.ComponentClassUnspecified && (entry.Component.Type != "" || entry.Component.Name != "") {
				results.Error(field.Invalid(entryPath.Child("component"), entry.Component, "Only one of component or class should be specified"))
			}
			if entry.Class == consts.ComponentClassUnspecified && entry.Component.Type == "" && entry.Component.Name == "" {
				results.Error(field.Invalid(entryPath.Child("component"), entry.Component, "Either component or class should be specified"))
			}
			if entry.Component.Type != "" && !slices.Contains(consts.LocalComponentTypes, entry.Component.Type) {
				results.Error(field.Invalid(entryPath.Child("component").Child("type"), entry.Component.Type, "Unknown component type"))
			}
			if exclusiveClass != consts.ComponentClassUnspecified {
				results.Error(field.Invalid(entryPath, entry, fmt.Sprintf("Update plan already contains class %s", exclusiveClass)))
			}

			switch entry.Class {
			case consts.ComponentClassNothing, consts.ComponentClassEverything:
				exclusiveClass = entry.Class
				if i > 0 {
					results.Error(field.Invalid(entryPath.Child("class"), entry.Class, "Should be first and only entry"))
				}
			case consts.ComponentClassStateless, consts.ComponentClassUnspecified:
			default:
				results.Error(field.Invalid(entryPath.Child("class"), entry.Class, "Unknown class"))
			}

			results.Append(validateUpdateModeForSelector(newYtsaurus, entry, entryPath.Child("updateMode")))
		}
	}

	return results
}

func validateUpdateModeForSelector(newYtsaurus *ytv1.Ytsaurus, selector ytv1.ComponentUpdateSelector, path *field.Path) Results {
	var results Results

	modeType := selector.GetUpdateStrategyType()
	bulkOnlyComponentTypes := map[consts.ComponentType]struct{}{
		consts.QueueAgentType:   {},
		consts.QueryTrackerType: {},
		consts.YqlAgentType:     {},
	}

	// strategy currently supported only for concrete components
	if selector.Class != consts.ComponentClassUnspecified && modeType != "" {
		results.Error(field.Invalid(path.Child("strategy"), modeType, "strategy is supported only for specific components, not for component classes"))
		return results
	}

	if selector.Class == consts.ComponentClassUnspecified {
		if selector.Component.Type == "" && selector.Strategy != nil {
			results.Error(field.Invalid(path, selector.Strategy, "component.type must be set to use strategy"))
			return results
		}
		// validate bulk-only restriction
		if _, bulkOnly := bulkOnlyComponentTypes[selector.Component.Type]; bulkOnly && modeType != "" && modeType != ytv1.ComponentUpdateModeTypeBulkUpdate {
			results.Error(field.Invalid(path.Child("strategy"), modeType, fmt.Sprintf("%s supports only BulkUpdate mode", selector.Component.Type)))
			return results
		}
	}

	switch modeType {
	case ytv1.ComponentUpdateModeTypeBulkUpdate:
		if selector.Strategy != nil {
			if selector.Strategy.RollingUpdate != nil {
				results.Error(field.Invalid(path.Child("rollingUpdate"), selector.Strategy.RollingUpdate, "rolling configuration is not valid for BulkUpdate"))
			}
		}
	case ytv1.ComponentUpdateModeTypeRollingUpdate:
		if selector.Component.Type == "" {
			results.Error(field.Invalid(path.Child("type"), modeType, "rolling update requires a concrete component selector"))
		}

		if selector.Component.Type == ytv1.DataNodeType && selector.Component.Name == "" && len(newYtsaurus.Spec.DataNodes) > 1 && selector.Concurrency == nil {
			results.Error(field.Invalid(path.Child("concurrency"), modeType, "rolling update for several data node groups requires concurrency limit"))
		}

	case ytv1.ComponentUpdateModeTypeOnDelete:
		if selector.Component.Type == "" {
			results.Error(field.Invalid(path.Child("type"), modeType, "onDelete update requires a concrete component selector"))
		}
	}

	return results
}

func (r *ytsaurusValidator) validateYtsaurus(ctx context.Context, newYtsaurus, oldYtsaurus *ytv1.Ytsaurus) Results {
	var results Results

	if newYtsaurus == nil {
		return results
	}

	if err := ValidateVersionConstraint(newYtsaurus.Spec.RequiresOperatorVersion); err != nil {
		results.Error(err)
	}

	results.Append(r.validateCommonSpec(&newYtsaurus.Spec.CommonSpec))
	results.Append(r.validatePodSpec(&newYtsaurus.Spec.PodSpec, field.NewPath("spec")))
	results.Append(r.validateDiscovery(newYtsaurus))
	results.Append(r.validatePrimaryMasters(newYtsaurus, oldYtsaurus))
	results.Append(r.validateSecondaryMasters(newYtsaurus, oldYtsaurus))
	results.Append(r.validateMasterCaches(newYtsaurus))
	results.Append(r.validateHTTPProxies(newYtsaurus))
	results.Append(r.validateRPCProxies(newYtsaurus))
	results.Append(r.validateTCPProxies(newYtsaurus))
	results.Append(r.validateKafkaProxies(newYtsaurus))
	results.Append(r.validateDataNodes(newYtsaurus))
	results.Append(r.validateExecNodes(newYtsaurus))
	results.Append(r.validateSchedulers(newYtsaurus))
	results.Append(r.validateControllerAgents(newYtsaurus))
	results.Append(r.validateTabletNodes(newYtsaurus))
	results.Append(r.validateChyt(newYtsaurus))
	results.Append(r.validateStrawberry(newYtsaurus))
	results.Append(r.validateQueryTrackers(newYtsaurus))
	results.Append(r.validateQueueAgents(newYtsaurus))
	results.Append(r.validateCypressProxies(newYtsaurus))
	results.Append(r.validateBundleController(newYtsaurus))
	results.Append(r.validateTabletBalancer(newYtsaurus))
	results.Append(r.validateSpyt(newYtsaurus))
	results.Append(r.validateYQLAgents(newYtsaurus))
	results.Append(r.validateUi(newYtsaurus))
	results.Append(r.validateExistsYtsaurus(ctx, newYtsaurus))
	results.Append(r.validateUpdatePlan(newYtsaurus))

	return results
}
