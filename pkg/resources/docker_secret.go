package resources

import (
	"path"

	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

// DockerSecret represents mounted kubernetes.io/dockerconfigjson secret
// https://kubernetes.io/docs/concepts/configuration/secret/#docker-config-secrets
type DockerSecret struct {
	SecretName string
	VolumeName string
	MountPath  string
}

func NewDockerSecret(secretName string, volumeName string, mountPath string) *DockerSecret {
	return &DockerSecret{
		SecretName: secretName,
		VolumeName: volumeName,
		MountPath:  mountPath,
	}
}

func (t *DockerSecret) AddVolume(podSpec *corev1.PodSpec) {
	if t == nil {
		return
	}
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: t.VolumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: t.SecretName,
				Items: []corev1.KeyToPath{{
					Key:  corev1.DockerConfigJsonKey,
					Path: consts.DockerSecretFileName,
					Mode: ptr.To(int32(0o400)),
				}},
			},
		},
	})
}

func (t *DockerSecret) AddVolumeMount(container *corev1.Container) {
	if t == nil {
		return
	}
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      t.VolumeName,
		MountPath: t.MountPath,
		ReadOnly:  true,
	})
}

func (t *DockerSecret) GetPath() string {
	if t == nil {
		return ""
	}
	return path.Join(t.MountPath, consts.DockerSecretFileName)
}
