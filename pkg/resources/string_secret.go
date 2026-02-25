package resources

import (
	"context"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/consts"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (s *StringSecret) GetUserName() string {
	return s.oldObject.Annotations[consts.UserNameAnnotationName]
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

func (s *StringSecret) SetValue(key, value string) {
	s.newObject.Data[key] = []byte(value)
}

func (s *StringSecret) SetUserName(user string) {
	metav1.SetMetaDataAnnotation(&s.newObject.ObjectMeta, consts.UserNameAnnotationName, user)
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

func (s *StringSecret) Build() *corev1.Secret {
	s.newObject = &corev1.Secret{
		ObjectMeta: s.labeller.GetObjectMeta(s.name),
		Data:       map[string][]byte{},
		Type:       corev1.SecretTypeOpaque,
	}
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

func (s *StringSecret) Delete(ctx context.Context) error {
	return s.proxy.DeleteObject(ctx, s.oldObject)
}
