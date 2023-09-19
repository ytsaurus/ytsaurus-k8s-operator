package resources

import (
	corev1 "k8s.io/api/core/v1"
)

// TLSSecret represents mounted kubernetes.io/tls secret
// https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets
type TLSSecret struct {
	SecretName string
	VolumeName string
	MountPath  string
}

func NewTLSSecret(secretName string, volumeName string, mountPath string) *TLSSecret {
	return &TLSSecret{
		SecretName: secretName,
		VolumeName: volumeName,
		MountPath:  mountPath,
	}
}

func (t *TLSSecret) AddVolume(podSpec *corev1.PodSpec) {
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: t.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: t.SecretName,
			},
		},
	})
}

func (t *TLSSecret) AddVolumeMount(container *corev1.Container) {
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      t.VolumeName,
		MountPath: t.MountPath,
		ReadOnly:  true,
	})
}
