package components

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/yson"
	corev1 "k8s.io/api/core/v1"
)

type ConfigHelper struct {
	labeller *labeller.Labeller
	apiProxy *apiproxy.APIProxy

	generator ytconfig.GeneratorFunc

	overridesMap corev1.ConfigMap
	fileName     string

	configMap *resources.ConfigMap
}

func NewConfigHelper(
	labeller *labeller.Labeller,
	apiProxy *apiproxy.APIProxy,
	name, fileName string,
	generator ytconfig.GeneratorFunc) *ConfigHelper {
	return &ConfigHelper{
		labeller:  labeller,
		apiProxy:  apiProxy,
		generator: generator,
		fileName:  fileName,
		configMap: resources.NewConfigMap(name, labeller, apiProxy),
	}
}

func mergeMapsRecursively(dst, src map[string]interface{}) map[string]interface{} {
	for key, srcVal := range src {
		if dstVal, ok := dst[key]; ok {
			srcMap, srcMapOk := mapify(srcVal)
			dstMap, dstMapOk := mapify(dstVal)
			if srcMapOk && dstMapOk {
				srcVal = mergeMapsRecursively(dstMap, srcMap)
			}
		}
		dst[key] = srcVal
	}
	return dst
}

func mapify(i interface{}) (map[string]interface{}, bool) {
	value := reflect.ValueOf(i)
	if value.Kind() == reflect.Map {
		m := map[string]interface{}{}
		for _, k := range value.MapKeys() {
			m[k.String()] = value.MapIndex(k).Interface()
		}
		return m, true
	}
	return map[string]interface{}{}, false
}

func overrideYsonConfigs(base []byte, overrides []byte) ([]byte, error) {
	b := map[string]interface{}{}
	err := yson.Unmarshal(base, b)
	if err != nil {
		return base, err
	}

	o := map[string]interface{}{}
	err = yson.Unmarshal(overrides, o)
	if err != nil {
		return base, err
	}

	merged := mergeMapsRecursively(b, o)
	return yson.Marshal(merged)
}

func (h *ConfigHelper) GetFileName() string {
	return h.fileName
}

func (h *ConfigHelper) GetConfigMapName() string {
	return h.configMap.Name()
}

func (h *ConfigHelper) getConfig() []byte {
	if h.generator == nil {
		return nil
	}

	serializedConfig, err := h.generator()
	if err != nil {
		panic(err)
	}
	if h.overridesMap.GetResourceVersion() != "" {
		if value, ok := h.overridesMap.BinaryData[h.fileName]; ok {
			configWithOverrides, err := overrideYsonConfigs(serializedConfig, value)
			if err == nil {
				serializedConfig = configWithOverrides
			} else {
				// ToDo(psushin): better error handling.
				h.apiProxy.RecordWarning(
					"Reconciling",
					fmt.Sprintf("Failed to apply config overrides for %s, skipping: %s", h.fileName, err))
			}
		}
	}

	return serializedConfig
}

func (h *ConfigHelper) NeedSync() bool {
	// ToDo(psushin): there could be more sophisticated logic.
	return !resources.Exists(h.configMap)
}

func (h *ConfigHelper) Build() *corev1.ConfigMap {
	cm := h.configMap.Build()
	data := h.getConfig()
	if data != nil {
		cm.BinaryData[h.fileName] = data
	}

	return cm
}

func (h *ConfigHelper) Sync(ctx context.Context) error {
	if !h.NeedSync() {
		return nil
	}

	return h.configMap.Sync(ctx)
}

func (h *ConfigHelper) Fetch(ctx context.Context) error {
	if h.apiProxy.Ytsaurus().Spec.ConfigOverrides != nil {
		name := h.apiProxy.Ytsaurus().Spec.ConfigOverrides.Name
		err := h.apiProxy.FetchObject(ctx, name, &h.overridesMap)
		if err != nil {
			return err
		}
	}
	return h.configMap.Fetch(ctx)
}
