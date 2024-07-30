package components

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/BurntSushi/toml"

	"github.com/google/go-cmp/cmp"
	"go.ytsaurus.tech/yt/go/yson"
	corev1 "k8s.io/api/core/v1"

	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/apiproxy"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/labeller"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/resources"
	"github.com/ytsaurus/ytsaurus-k8s-operator/pkg/ytconfig"
)

const (
	JsPrologue string = "module.exports = "
)

type ConfigHelper struct {
	labeller *labeller.Labeller
	apiProxy apiproxy.APIProxy

	generators map[string]ytconfig.GeneratorDescriptor

	configOverrides *corev1.LocalObjectReference
	overridesMap    corev1.ConfigMap

	configMap *resources.ConfigMap
}

func NewConfigHelper(
	labeller *labeller.Labeller,
	apiProxy apiproxy.APIProxy,
	name string,
	configOverrides *corev1.LocalObjectReference,
	generators map[string]ytconfig.GeneratorDescriptor) *ConfigHelper {
	return &ConfigHelper{
		labeller:        labeller,
		apiProxy:        apiProxy,
		generators:      generators,
		configOverrides: configOverrides,
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

func (h *ConfigHelper) GetFileNames() []string {
	fileNames := []string{}
	for fileName := range h.generators {
		fileNames = append(fileNames, fileName)
	}
	return fileNames
}

func (h *ConfigHelper) GetConfigMapName() string {
	return h.configMap.Name()
}

func (h *ConfigHelper) getConfig(fileName string) ([]byte, error) {
	descriptor, ok := h.generators[fileName]
	if !ok {
		return nil, nil
	}

	serializedConfig, err := descriptor.F()
	if err != nil {
		h.apiProxy.RecordWarning(
			"Reconciling",
			fmt.Sprintf("Failed to build config %s: %s", fileName, err))
		return nil, err
	}

	if h.overridesMap.GetResourceVersion() != "" {
		overrideNames := []string{
			fileName,
			fmt.Sprintf("%s--%s", h.GetConfigMapName(), fileName),
		}
		for _, overrideName := range overrideNames {
			if value, ok := h.overridesMap.Data[overrideName]; ok {
				configWithOverrides, err := overrideYsonConfigs(serializedConfig, []byte(value))
				if err == nil {
					serializedConfig = configWithOverrides
				} else {
					// ToDo(psushin): better error handling.
					h.apiProxy.RecordWarning(
						"Reconciling",
						fmt.Sprintf("Failed to apply config override %s for %s, skipping: %s", overrideName, fileName, err))
				}
			}
		}
	}

	switch descriptor.Fmt {
	case ytconfig.ConfigFormatJson, ytconfig.ConfigFormatJsonWithJsPrologue:
		var config any
		err := yson.Unmarshal(serializedConfig, &config)
		if err != nil {
			return nil, err
		}
		serializedConfig, err = json.Marshal(config)
		if err != nil {
			return nil, err
		}
		if descriptor.Fmt == ytconfig.ConfigFormatJsonWithJsPrologue {
			serializedConfig = append([]byte(JsPrologue), serializedConfig...)
		}
	case ytconfig.ConfigFormatToml:
		var config any
		err := yson.Unmarshal(serializedConfig, &config)
		if err != nil {
			return nil, err
		}

		var buf bytes.Buffer
		err = toml.NewEncoder(&buf).Encode(config)
		if err != nil {
			return nil, err
		}
		serializedConfig = buf.Bytes()
	}

	return serializedConfig, nil
}

func (h *ConfigHelper) getCurrentConfigValue(fileName string) []byte {
	if !resources.Exists(h.configMap) {
		return nil
	}

	data, exists := h.configMap.OldObject().(*corev1.ConfigMap).Data[fileName]
	if !exists {
		return nil
	}
	return []byte(data)
}

func (h *ConfigHelper) NeedReload() (bool, error) {
	for fileName := range h.generators {
		newConfig, err := h.getConfig(fileName)
		if err != nil {
			return false, err
		}
		curConfig := h.getCurrentConfigValue(fileName)
		if !cmp.Equal(curConfig, newConfig) {
			h.apiProxy.RecordNormal(
				"Reconciliation",
				fmt.Sprintf("Config %s needs reload", fileName))
			return true, nil
		}
	}
	return false, nil
}

func (h *ConfigHelper) NeedInit() bool {
	return !resources.Exists(h.configMap)
}

func (h *ConfigHelper) Build() *corev1.ConfigMap {
	cm := h.configMap.Build()

	for fileName := range h.generators {
		data, err := h.getConfig(fileName)
		if err != nil {
			return nil
		}

		if data != nil {
			cm.Data[fileName] = string(data)
		}
	}

	return cm
}

func (h *ConfigHelper) Sync(ctx context.Context) error {
	needReload, err := h.NeedReload()
	if err != nil {
		needReload = false
	}
	if !h.NeedInit() && !needReload {
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

func (h *ConfigHelper) RemoveIfExists(ctx context.Context) error {
	if !resources.Exists(h.configMap) {
		return nil
	}

	return h.apiProxy.DeleteObject(
		ctx,
		h.configMap.OldObject(),
	)
}

func (h *ConfigHelper) Exists() bool {
	return resources.Exists(h.configMap)
}
