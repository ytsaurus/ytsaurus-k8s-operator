package resources

import (
	"path"
	"slices"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

// CABundle represents mounted configmap with trusted certificates
type CABundle struct {
	Source     ytv1.FileObjectReference
	VolumeName string
	MountPath  string
	FileName   string
}

func NewCABundle(source *ytv1.FileObjectReference) *CABundle {
	if source == nil {
		return nil
	}
	return &CABundle{
		Source:     *source,
		VolumeName: consts.CABundleVolumeName,
		MountPath:  consts.CABundleMountPoint,
		FileName:   consts.CABundleFileName,
	}
}

func NewCARootBundle(source *ytv1.FileObjectReference) *CABundle {
	if source == nil {
		return nil
	}
	return &CABundle{
		Source:     *source,
		VolumeName: consts.CARootBundleVolumeName,
		MountPath:  consts.CARootBundleMountPoint,
		FileName:   consts.CARootBundleFileName,
	}
}

func (t *CABundle) AddVolume(podSpec *corev1.PodSpec) {
	if t == nil {
		return
	}

	items := slices.Clone(t.Source.Items)
	if len(items) == 0 && t.Source.Key != "" {
		items = []corev1.KeyToPath{{Key: t.Source.Key, Path: t.FileName}}
	}

	switch t.Source.Kind {
	case "", "ConfigMap":
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: t.VolumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: t.Source.Name,
					},
					Items: items,
				},
			},
		})
	case "Secret":
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: t.VolumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: t.Source.Name,
					Items:      items,
				},
			},
		})
	}
}

func (t *CABundle) AddVolumeMount(container *corev1.Container) {
	if t == nil {
		return
	}
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      t.VolumeName,
		MountPath: t.MountPath,
		ReadOnly:  true,
	})
}

func (t *CABundle) AddContainerEnv(container *corev1.Container) {
	if t == nil {
		return
	}
	container.Env = append(container.Env,
		corev1.EnvVar{
			Name:  consts.SSLCertFileKey,
			Value: path.Join(t.MountPath, t.FileName),
		},
		corev1.EnvVar{
			Name:  consts.SSLCertDirKey,
			Value: t.MountPath,
		},
		corev1.EnvVar{
			Name:  consts.RequestsCABundleKey,
			Value: path.Join(t.MountPath, t.FileName),
		},
	)
}
