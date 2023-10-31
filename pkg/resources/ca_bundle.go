package resources

import (
	corev1 "k8s.io/api/core/v1"
)

// CABundle represents mounted configmap with trusted certificates
type CABundle struct {
	ConfigMapName string
	VolumeName    string
	MountPath     string
}

func NewCABundle(configMapName string, volumeName string, mountPath string) *CABundle {
	return &CABundle{
		ConfigMapName: configMapName,
		VolumeName:    volumeName,
		MountPath:     mountPath,
	}
}

func (t *CABundle) AddVolume(podSpec *corev1.PodSpec) {
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: t.VolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: t.ConfigMapName,
				},
			},
		},
	})
}

func (t *CABundle) AddVolumeMount(container *corev1.Container) {
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      t.VolumeName,
		MountPath: t.MountPath,
		ReadOnly:  true,
	})
}
