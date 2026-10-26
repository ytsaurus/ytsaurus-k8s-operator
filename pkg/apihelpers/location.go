package apihelpers

import (
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"
)

func FindFirstLocation(locations []ytv1.LocationSpec, locationType ytv1.LocationType) *ytv1.LocationSpec {
	for i, location := range locations {
		if location.LocationType == locationType {
			return &locations[i]
		}
	}
	return nil
}

func FindAllLocations(locations []ytv1.LocationSpec, locationType ytv1.LocationType) []*ytv1.LocationSpec {
	result := make([]*ytv1.LocationSpec, 0)
	for i, location := range locations {
		if location.LocationType == locationType {
			result = append(result, &locations[i])
		}
	}
	return result
}

func FindVolumeMountForPath(volumeMounts []corev1.VolumeMount, path string) *corev1.VolumeMount {
	for i, mount := range slices.Backward(volumeMounts) {
		if path == mount.MountPath || strings.HasPrefix(path, mount.MountPath+"/") {
			return &volumeMounts[i]
		}
	}
	return nil
}

func findVolumeClaimTemplate(volumeName string, spec *ytv1.InstanceSpec) *ytv1.EmbeddedPersistentVolumeClaim {
	for _, claim := range spec.VolumeClaimTemplates {
		if claim.Name == volumeName {
			return &claim
		}
	}
	return nil
}

func findVolume(volumeName string, spec *ytv1.InstanceSpec) *ytv1.Volume {
	for _, volume := range spec.Volumes {
		if volume.Name == volumeName {
			return &volume
		}
	}
	return nil
}

func FindQuotaForLocation(location *ytv1.LocationSpec, spec *ytv1.InstanceSpec) (request *resource.Quantity, limit *resource.Quantity) {
	if quota := location.Quota; quota != nil {
		return quota, quota // NOTE: location quota overrides both storage request and limit.
	}

	mount := FindVolumeMountForPath(spec.VolumeMounts, location.Path)
	if mount == nil {
		return nil, nil
	}

	var resources *corev1.VolumeResourceRequirements
	if claim := findVolumeClaimTemplate(mount.Name, spec); claim != nil {
		resources = &claim.Spec.Resources
	} else if volume := findVolume(mount.Name, spec); volume != nil {
		if volume.EmptyDir != nil && volume.EmptyDir.SizeLimit != nil {
			val := volume.EmptyDir.SizeLimit
			return val, val
		}
		if volume.Ephemeral != nil && volume.Ephemeral.VolumeClaimTemplate != nil {
			resources = &volume.Ephemeral.VolumeClaimTemplate.Spec.Resources
		}
	}

	if resources != nil {
		if storage := resources.Requests.Storage(); storage != nil && !storage.IsZero() {
			request = storage
		}
		if storage := resources.Limits.Storage(); storage != nil && !storage.IsZero() {
			limit = storage
		}
		return request, limit
	}

	return nil, nil
}

func MinQuantity(values ...*resource.Quantity) (result *resource.Quantity) {
	for _, value := range values {
		if value != nil && (result == nil || value.Cmp(*result) < 0) {
			result = ptr.To(value.DeepCopy())
		}
	}
	return result
}

func SumQuantity(values ...*resource.Quantity) (result *resource.Quantity) {
	result = resource.NewQuantity(0, resource.BinarySI)
	for _, value := range values {
		if value == nil {
			result = nil
		} else if result != nil {
			result.Add(*value)
		}
	}
	return result
}

func MaxLogsTotalSize(policy *ytv1.LogRotationPolicy) (result *resource.Quantity) {
	if policy != nil {
		if policy.MaxSegmentSize != nil && policy.MaxSegmentCountToKeep != nil {
			result = ptr.To(policy.MaxSegmentSize.DeepCopy())
			result.Mul(*policy.MaxSegmentCountToKeep)
		}
		result = MinQuantity(result, policy.MaxTotalSizeToKeep)
	}
	return result
}
