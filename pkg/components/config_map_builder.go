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
)

const (
	JsPrologue string = "module.exports = "
)

type ConfigFormat string

const (
	ConfigFormatYson               = "yson"
	ConfigFormatJson               = "json"
	ConfigFormatJsonWithJsPrologue = "json_with_js_prologue"
	ConfigFormatToml               = "toml"
)

type ConfigGeneratorFunc func() ([]byte, error)

type ConfigGenerator struct {
	// Format is the desired serialization format for config map.
	// Note that conversion from YSON to Format (if needed) is performed as a very last
	// step of config generation pipeline.
	Format ConfigFormat
	// Generator must generate config in YSON.
	Generator ConfigGeneratorFunc
}

type ConfigMapBuilder struct {
	labeller *labeller.Labeller
	apiProxy apiproxy.APIProxy

	generators map[string]ConfigGenerator

	configOverrides *corev1.LocalObjectReference
	overridesMap    corev1.ConfigMap

	configMap *resources.ConfigMap
}

func NewConfigMapBuilder(
	labeller *labeller.Labeller,
	apiProxy apiproxy.APIProxy,
	name string,
	configOverrides *corev1.LocalObjectReference,
) *ConfigMapBuilder {
	return &ConfigMapBuilder{
		labeller:        labeller,
		apiProxy:        apiProxy,
		configOverrides: configOverrides,
		configMap:       resources.NewConfigMap(name, labeller, apiProxy),
	}
}

func (h *ConfigMapBuilder) AddGenerator(fileName string, format ConfigFormat, generator ConfigGeneratorFunc) {
	if h.generators == nil {
		h.generators = make(map[string]ConfigGenerator)
	}
	h.generators[fileName] = ConfigGenerator{
		Format:    format,
		Generator: generator,
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

func (h *ConfigMapBuilder) GetFileNames() []string {
	fileNames := []string{}
	for fileName := range h.generators {
		fileNames = append(fileNames, fileName)
	}
	return fileNames
}

func (h *ConfigMapBuilder) GetConfigMapName() string {
	return h.configMap.Name()
}

func (h *ConfigMapBuilder) getConfig(fileName string) ([]byte, error) {
	descriptor, ok := h.generators[fileName]
	if !ok {
		return nil, nil
	}

	serializedConfig, err := descriptor.Generator()
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

	switch descriptor.Format {
	case ConfigFormatJson, ConfigFormatJsonWithJsPrologue:
		var config any
		err := yson.Unmarshal(serializedConfig, &config)
		if err != nil {
			return nil, err
		}
		serializedConfig, err = json.Marshal(config)
		if err != nil {
			return nil, err
		}
		if descriptor.Format == ConfigFormatJsonWithJsPrologue {
			serializedConfig = append([]byte(JsPrologue), serializedConfig...)
		}
	case ConfigFormatToml:
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

func (h *ConfigMapBuilder) getCurrentConfigValue(fileName string) []byte {
	if !resources.Exists(h.configMap) {
		return nil
	}

	data, exists := h.configMap.OldObject().Data[fileName]
	if !exists {
		return nil
	}
	return []byte(data)
}

func (h *ConfigMapBuilder) NeedReload() (bool, error) {
	for fileName := range h.generators {
		newConfig, err := h.getConfig(fileName)
		if err != nil {
			return false, err
		}
		curConfig := h.getCurrentConfigValue(fileName)
		if !cmp.Equal(curConfig, newConfig) {
			if curConfig == nil {
				h.apiProxy.RecordNormal(
					"Reconciliation",
					fmt.Sprintf("Config %s needs creation", fileName))
			} else {
				configsDiff := cmp.Diff(string(curConfig), string(newConfig))
				h.apiProxy.RecordNormal(
					"Reconciliation",
					fmt.Sprintf("Config %s needs reload. Diff: %s", fileName, configsDiff))
			}
			return true, nil
		}
	}
	return false, nil
}

func (h *ConfigMapBuilder) NeedInit() bool {
	return !resources.Exists(h.configMap)
}

func (h *ConfigMapBuilder) Build() *corev1.ConfigMap {
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

func (h *ConfigMapBuilder) Sync(ctx context.Context) error {
	needReload, err := h.NeedReload()
	if err != nil {
		needReload = false
	}
	if !h.NeedInit() && !needReload {
		return nil
	}

	return h.configMap.Sync(ctx)
}

func (h *ConfigMapBuilder) Fetch(ctx context.Context) error {
	if h.configOverrides != nil {
		name := h.configOverrides.Name
		err := h.apiProxy.FetchObject(ctx, name, &h.overridesMap)
		if err != nil {
			return err
		}
	}
	return h.configMap.Fetch(ctx)
}

func (h *ConfigMapBuilder) RemoveIfExists(ctx context.Context) error {
	if !resources.Exists(h.configMap) {
		return nil
	}

	return h.apiProxy.DeleteObject(
		ctx,
		h.configMap.OldObject(),
	)
}

func (h *ConfigMapBuilder) Exists() bool {
	return resources.Exists(h.configMap)
}
