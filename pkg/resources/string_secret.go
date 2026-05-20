package resources

import (
	"path"

	"k8s.io/utils/ptr"

	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
)

type StringSecret struct {
	BaseManagedResource[*corev1.Secret]
}

func NewStringSecret(name string, reconciler *labeller.Labeller, apiProxy apiproxy.APIProxy) *StringSecret {
	return &StringSecret{
		BaseManagedResource: BaseManagedResource[*corev1.Secret]{
			proxy:     apiProxy,
			labeller:  reconciler,
			name:      name,
			oldObject: &corev1.Secret{},
			newObject: &corev1.Secret{},
		},
	}
}

func (s *StringSecret) GetValue(key string) (string, bool) {
	if value, ok := s.oldObject.Data[key]; ok {
		return string(value), true
	}
	// Fake k8s client does not move StringData into Data.
	if value, ok := s.oldObject.StringData[key]; ok {
		return value, true
	}
	return "", false
}

func (s *StringSecret) GetEnvSource() corev1.EnvFromSource {
	return corev1.EnvFromSource{
		SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: s.Name(),
			},
		},
	}
}

func (s *StringSecret) GetTokenVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: s.Name(),
				Items: []corev1.KeyToPath{{
					Key:  consts.TokenSecretKey,
					Path: consts.TokenFileName,
					Mode: ptr.To(int32(0o400)),
				}},
			},
		},
	}
}

func (s *StringSecret) GetTokenVolumeMount(name string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      name,
		MountPath: path.Join(consts.SecretsMountBase, name),
		ReadOnly:  true,
	}
}

func (s *StringSecret) Build() *corev1.Secret {
	s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)
	s.newObject.Type = corev1.SecretTypeOpaque
	return s.newObject
}

func (s *StringSecret) NeedSync(key, value string) bool {
	if !s.Exists() {
		return true
	}

	v, ok := s.GetValue(key)
	if !ok {
		return true
	}

	if value == "" {
		return false
	}

	return value != v
}
