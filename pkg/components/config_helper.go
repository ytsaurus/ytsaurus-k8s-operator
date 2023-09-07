package components

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/ytsaurus/yt-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/yt-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/yt-k8s-operator/pkg/resources"
	"github.com/ytsaurus/yt-k8s-operator/pkg/ytconfig"
	"go.ytsaurus.tech/yt/go/yson"
	corev1 "k8s.io/api/core/v1"
)

type ConfigHelper struct {
	labeller *labeller.Labeller
	apiProxy apiproxy.APIProxy

	generator ytconfig.GeneratorFunc

	configOverrides *corev1.LocalObjectReference
	overridesMap    corev1.ConfigMap
	fileName        string

	configMap *resources.ConfigMap
}

func NewConfigHelper(
	labeller *labeller.Labeller,
	apiProxy apiproxy.APIProxy,
	name, fileName string,
	configOverrides *corev1.LocalObjectReference,
	generator ytconfig.GeneratorFunc) *ConfigHelper {
	return &ConfigHelper{
		labeller:        labeller,
		apiProxy:        apiProxy,
		generator:       generator,
		configOverrides: configOverrides,
		fileName:        fileName,
		configMap:       resources.NewConfigMap(name, labeller, apiProxy),
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
	err := yson.Unmarshal(base, &b)
	if err != nil {
		return base, err
	}

	o := map[string]interface{}{}
	err = yson.Unmarshal(overrides, &o)
	if err != nil {
		return base, err
	}

	merged := mergeMapsRecursively(b, o)
	return yson.MarshalFormat(merged, yson.FormatPretty)
}

func (h *ConfigHelper) GetFileName() string {
	return h.fileName
}

func (h *ConfigHelper) GetConfigMapName() string {
	return h.configMap.Name()
}

func (h *ConfigHelper) getConfig() ([]byte, error) {
	if h.generator == nil {
		return nil, nil
	}

	serializedConfig, err := h.generator()
	if err != nil {
		h.apiProxy.RecordWarning(
			"Reconciling",
			fmt.Sprintf("Failed to build config %s: %s", h.fileName, err))
		return nil, err
	}

	if h.overridesMap.GetResourceVersion() != "" {
		if value, ok := h.overridesMap.Data[h.fileName]; ok {
			configWithOverrides, err := overrideYsonConfigs(serializedConfig, []byte(value))
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

	return serializedConfig, nil
}

func (h *ConfigHelper) getCurrentConfigValue() []byte {
	if !resources.Exists(h.configMap) {
		return nil
	}

	data, exists := h.configMap.OldObject().(*corev1.ConfigMap).Data[h.fileName]
	if !exists {
		return nil
	}
	return []byte(data)
}

func (h *ConfigHelper) NeedReload() (bool, error) {
	newConfig, err := h.getConfig()
	if err != nil {
		return false, err
	}
	curConfig := h.getCurrentConfigValue()
	if cmp.Equal(curConfig, newConfig) {
		return false, nil
	}
	h.apiProxy.RecordNormal(
		"Reconciliation",
		fmt.Sprintf("Config %s needs reload", h.fileName))
	return true, nil
}

func (h *ConfigHelper) NeedSync() bool {
	if !resources.Exists(h.configMap) {
		return true
	}
	needReload, err := h.NeedReload()
	if err != nil {
		return false
	}
	return needReload

}

func (h *ConfigHelper) Build() *corev1.ConfigMap {
	cm := h.configMap.Build()
	data, err := h.getConfig()
	if err != nil {
		return nil
	}

	if data != nil {
		cm.Data[h.fileName] = string(data)
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
	if h.configOverrides != nil {
		name := h.configOverrides.Name
		err := h.apiProxy.FetchObject(ctx, name, &h.overridesMap)
		if err != nil {
			return err
		}
	}
	return h.configMap.Fetch(ctx)
}
