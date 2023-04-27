package resources

import (
	"context"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type StringSecret struct {
	name       string
	reconciler *labeller.Labeller
	apiProxy   *apiproxy.APIProxy

	oldObject corev1.Secret
	newObject corev1.Secret
}

func NewStringSecret(name string, reconciler *labeller.Labeller, apiProxy *apiproxy.APIProxy) *StringSecret {
	return &StringSecret{
		name:       name,
		reconciler: reconciler,
		apiProxy:   apiProxy,
	}
}

func (s *StringSecret) OldObject() client.Object {
	return &s.oldObject
}

func (s *StringSecret) Name() string {
	return s.name
}

func (s *StringSecret) Sync(ctx context.Context) error {
	return s.apiProxy.SyncObject(ctx, &s.oldObject, &s.newObject)
}

func (s *StringSecret) GetValue(key string) (string, bool) {
	v, ok := s.oldObject.Data[key]
	if !ok {
		return "", ok
	}
	return string(v), ok
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
	s.newObject.ObjectMeta = s.reconciler.GetObjectMeta(s.name)
	s.newObject.Type = corev1.SecretTypeOpaque
	return &s.newObject
}

func (s *StringSecret) NeedSync(key, value string) bool {
	if !Exists(s) {
		return true
	}

	v, ok := s.GetValue(key)
	if !ok {
		return true
	}

	if value == "" {
		return false
	}

	return value != string(v)
}

func (s *StringSecret) Fetch(ctx context.Context) error {
	return s.apiProxy.FetchObject(ctx, s.name, &s.oldObject)
}
