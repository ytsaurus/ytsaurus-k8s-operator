package resources

import (
	"context"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	labeller2 "github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ConfigMap struct {
	name     string
	labeller *labeller2.Labeller
	apiProxy apiproxy.APIProxy

	oldObject corev1.ConfigMap
	newObject corev1.ConfigMap
}

func NewConfigMap(name string, labeller *labeller2.Labeller, apiProxy apiproxy.APIProxy) *ConfigMap {
	return &ConfigMap{
		name:     name,
		labeller: labeller,
		apiProxy: apiProxy,
	}
}

func (s *ConfigMap) OldObject() client.Object {
	return &s.oldObject
}

func (s *ConfigMap) Name() string {
	return s.name
}

func (s *ConfigMap) Sync(ctx context.Context) error {
	return s.apiProxy.SyncObject(ctx, &s.oldObject, &s.newObject)
}

func (s *ConfigMap) Build() *corev1.ConfigMap {
	s.newObject.ObjectMeta = s.labeller.GetObjectMeta(s.name)
	s.newObject.Data = make(map[string]string)
	return &s.newObject
}

func (s *ConfigMap) Fetch(ctx context.Context) error {
	return s.apiProxy.FetchObject(ctx, s.name, &s.oldObject)
}
