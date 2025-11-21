package resources

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	ytv1 "github.com/ytsaurus/ytsaurus-k8s-operator/api/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
)

// CABundle represents mounted configmap with trusted certificates
type CABundle struct {
	Source     ytv1.FileObjectReference
	VolumeName string
	MountPath  string
}

func NewCABundle(source ytv1.FileObjectReference, volumeName string, mountPath string) *CABundle {
	return &CABundle{
		Source:     source,
		VolumeName: volumeName,
		MountPath:  mountPath,
	}
}

func (t *CABundle) AddVolume(podSpec *corev1.PodSpec) {
	var items []corev1.KeyToPath
	if t.Source.Key != "" {
		items = []corev1.KeyToPath{{Key: t.Source.Key, Path: consts.CABundleFileName}}
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
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      t.VolumeName,
		MountPath: t.MountPath,
		ReadOnly:  true,
	})
}

type CABundleCertificate struct {
	key       string
	secret    *BaseManagedResource[*corev1.Secret]
	configMap *BaseManagedResource[*corev1.ConfigMap]
}

func NewCABundleCertificate(source ytv1.FileObjectReference, proxy apiproxy.APIProxy) *CABundleCertificate {
	cert := &CABundleCertificate{
		key: source.Key,
	}
	if cert.key == "" {
		cert.key = consts.CABundleFileName
	}
	switch source.Kind {
	case "", "ConfigMap":
		cert.configMap = &BaseManagedResource[*corev1.ConfigMap]{
			proxy:     proxy,
			name:      source.Name,
			oldObject: &corev1.ConfigMap{},
		}
	case "Secret":
		cert.secret = &BaseManagedResource[*corev1.Secret]{
			proxy:     proxy,
			name:      source.Name,
			oldObject: &corev1.Secret{},
		}
	}
	return cert
}

func (c *CABundleCertificate) Fetch(ctx context.Context) error {
	return Fetch(ctx, c.secret, c.configMap)
}

func (c *CABundleCertificate) Get() ([]byte, error) {
	if c.configMap != nil {
		object := c.configMap.oldObject
		if data, found := object.Data[c.key]; found {
			return []byte(data), nil
		}

		if data, found := object.BinaryData[c.key]; found {
			return data, nil
		}
		return nil, fmt.Errorf("ca bundle configmap %q has no %q", object.Name, c.key)
	}
	if c.secret != nil {
		object := c.secret.oldObject
		if data, found := object.Data[c.key]; found {
			return data, nil
		}
		return nil, fmt.Errorf("ca bundle secret %q has no %q", object.Name, c.key)
	}
	return nil, fmt.Errorf("unknown CA bundle source")
}
