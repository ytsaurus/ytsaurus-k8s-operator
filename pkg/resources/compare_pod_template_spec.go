package resources

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

func podTemplateSpecEqual(a, b corev1.PodTemplateSpec) bool {
	return podSpecEqual(a.Spec, b.Spec)
}

func podSpecEqual(a, b corev1.PodSpec) bool {
	// Containers
	if !containersEqual(a.Containers, b.Containers) {
		return false
	}

	// InitContainers
	if !containersEqual(a.InitContainers, b.InitContainers) {
		return false
	}

	// Volumes
	if !volumesEqual(a.Volumes, b.Volumes) {
		return false
	}

	// Node selector, tolerations, affinity
	if !mapsEqual(a.NodeSelector, b.NodeSelector) {
		return false
	}
	if !tolerationsEqual(a.Tolerations, b.Tolerations) {
		return false
	}
	if !affinityEqual(a.Affinity, b.Affinity) {
		return false
	}

	// Service account
	if a.ServiceAccountName != b.ServiceAccountName {
		return false
	}

	return true
}

func containersEqual(a, b []corev1.Container) bool {
	if len(a) != len(b) {
		return false
	}

	// Sort by container name for determinism
	a, b = sortPair(a, b, func(c corev1.Container) string { return c.Name })

	for i := range a {
		ca := a[i]
		cb := b[i]
		if ca.Name != cb.Name {
			return false
		}

		if ca.Image != cb.Image {
			return false
		}

		if !stringSlicesEqual(ca.Command, cb.Command) {
			return false
		}
		if !stringSlicesEqual(ca.Args, cb.Args) {
			return false
		}

		if !envsEqual(ca.Env, cb.Env) {
			return false
		}
		if !ResourceRequirementsEqual(ca.Resources, cb.Resources) {
			return false
		}

		if !volumeMountsEqual(ca.VolumeMounts, cb.VolumeMounts) {
			return false
		}
	}

	return true
}

func envsEqual(a, b []corev1.EnvVar) bool {
	if len(a) != len(b) {
		return false
	}
	// Sort by env name
	a, b = sortPair(a, b, func(e corev1.EnvVar) string { return e.Name })

	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
		if a[i].Value != b[i].Value {
			return false
		}
		if (a[i].ValueFrom == nil) != (b[i].ValueFrom == nil) {
			return false
		}
	}

	return true
}

func tolerationsEqual(a, b []corev1.Toleration) bool {
	if len(a) != len(b) {
		return false
	}
	// Sort by key
	a, b = sortPair(a, b, func(t corev1.Toleration) string { return t.Key })

	for i := range a {
		if a[i].Key != b[i].Key ||
			a[i].Operator != b[i].Operator ||
			a[i].Value != b[i].Value ||
			a[i].Effect != b[i].Effect {
			return false
		}
	}
	return true
}

func affinityEqual(a, b *corev1.Affinity) bool {
	if equal, shouldReturn := bothNilOrBothNotNil(a, b); shouldReturn {
		return equal
	}

	if !nodeAffinityEqual(a.NodeAffinity, b.NodeAffinity) {
		return false
	}
	return true
}

func nodeAffinityEqual(a, b *corev1.NodeAffinity) bool {
	if equal, shouldReturn := bothNilOrBothNotNil(a, b); shouldReturn {
		return equal
	}

	if equal, shouldReturn := bothNilOrBothNotNil(a.RequiredDuringSchedulingIgnoredDuringExecution, b.RequiredDuringSchedulingIgnoredDuringExecution); shouldReturn {
		return equal
	}

	aTerms := a.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	bTerms := b.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	return len(aTerms) == len(bTerms)
}

func volumeMountsEqual(a, b []corev1.VolumeMount) bool {
	if len(a) != len(b) {
		return false
	}
	a, b = sortPair(a, b, func(v corev1.VolumeMount) string { return v.Name })

	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
		if a[i].MountPath != b[i].MountPath {
			return false
		}
		if a[i].SubPath != b[i].SubPath {
			return false
		}
		if a[i].ReadOnly != b[i].ReadOnly {
			return false
		}
	}
	return true
}

func volumesEqual(a, b []corev1.Volume) bool {
	if len(a) != len(b) {
		return false
	}
	a, b = sortPair(a, b, func(v corev1.Volume) string { return v.Name })

	for i := range a {
		if a[i].Name != b[i].Name {
			return false
		}
		if !volumeSourceEqual(a[i].VolumeSource, b[i].VolumeSource) {
			return false
		}
	}
	return true
}

// volumeSourceEqual compares key fields of common volume source types
func volumeSourceEqual(a, b corev1.VolumeSource) bool {
	if !nilnessEqual(a.ConfigMap, b.ConfigMap) {
		return false
	}
	if a.ConfigMap != nil && a.ConfigMap.Name != b.ConfigMap.Name {
		return false
	}

	if !nilnessEqual(a.Secret, b.Secret) {
		return false
	}
	if a.Secret != nil && a.Secret.SecretName != b.Secret.SecretName {
		return false
	}

	if !nilnessEqual(a.PersistentVolumeClaim, b.PersistentVolumeClaim) {
		return false
	}
	if a.PersistentVolumeClaim != nil && a.PersistentVolumeClaim.ClaimName != b.PersistentVolumeClaim.ClaimName {
		return false
	}

	if !nilnessEqual(a.EmptyDir, b.EmptyDir) {
		return false
	}

	if !nilnessEqual(a.HostPath, b.HostPath) {
		return false
	}
	if a.HostPath != nil && a.HostPath.Path != b.HostPath.Path {
		return false
	}

	return true
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ResourceRequirementsEqual compares resource requirements, handling nil vs empty cases
func ResourceRequirementsEqual(oldResourceRequirements, newResourceRequirements corev1.ResourceRequirements) bool {
	// Compare requests
	if !ResourceListEqual(oldResourceRequirements.Requests, newResourceRequirements.Requests) {
		return false
	}

	// Compare limits
	if !ResourceListEqual(oldResourceRequirements.Limits, newResourceRequirements.Limits) {
		return false
	}

	return true
}

// ResourceListEqual compares resource lists
func ResourceListEqual(oldResourceList, newResourceList corev1.ResourceList) bool {
	// If both are nil or empty, they're equal
	if len(oldResourceList) == 0 && len(newResourceList) == 0 {
		return true
	}

	if len(oldResourceList) != len(newResourceList) {
		return false
	}

	// Compare each resource
	for key, newVal := range newResourceList {
		oldVal, exists := oldResourceList[key]
		if !exists || !oldVal.Equal(newVal) {
			return false
		}
	}

	return true
}

func bothNilOrBothNotNil[T any](a, b *T) (equal bool, shouldReturn bool) {
	if a == nil && b == nil {
		return true, true
	}
	if (a == nil) != (b == nil) {
		return false, true
	}
	return false, false
}

// nilnessEqual checks if both pointers have the same nilness (both nil or both not nil)
func nilnessEqual[T any](a, b *T) bool {
	return (a == nil) == (b == nil)
}

// sortPair sorts two slices using the provided key extractor for comparison.
func sortPair[T any, K ~string](a, b []T, keyFunc func(T) K) (aSorted, bSorted []T) {
	// Create copies to avoid mutating originals
	aCopy := make([]T, len(a))
	bCopy := make([]T, len(b))
	copy(aCopy, a)
	copy(bCopy, b)

	sort.Slice(aCopy, func(i, j int) bool { return keyFunc(aCopy[i]) < keyFunc(aCopy[j]) })
	sort.Slice(bCopy, func(i, j int) bool { return keyFunc(bCopy[i]) < keyFunc(bCopy[j]) })

	return aCopy, bCopy
}
