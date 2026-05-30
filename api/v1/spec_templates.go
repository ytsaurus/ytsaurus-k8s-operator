package v1

import (
	"fmt"
	"reflect"
	"strings"

	"dario.cat/mergo"
	"github.com/mohae/deepcopy"
)

type InstanceGroupTemplateError struct {
	List    string
	Index   int
	Field   string
	Value   string
	Message string
}

func (e *InstanceGroupTemplateError) Error() string {
	return fmt.Sprintf("%s[%d].%s: %s", e.List, e.Index, e.Field, e.Message)
}

func resolveInstanceGroupTemplates[T any](listName string, items []T) ([]T, error) {
	classes := make(map[string]int, len(items))
	resolved := make(map[int]T, len(items))
	state := make(map[int]bool, len(items))

	for i := range items {
		class := getTemplateField(items[i], "Class")
		if class == "" {
			continue
		}
		if j, ok := classes[class]; ok {
			return nil, &InstanceGroupTemplateError{
				List:    listName,
				Index:   i,
				Field:   "class",
				Value:   class,
				Message: fmt.Sprintf("duplicate template class %q, first defined at index %d", class, j),
			}
		}
		classes[class] = i
	}

	var resolve func(index int) (T, error)
	resolve = func(index int) (T, error) {
		if item, ok := resolved[index]; ok {
			return item, nil
		}
		if state[index] {
			return *new(T), &InstanceGroupTemplateError{
				List:    listName,
				Index:   index,
				Field:   "from",
				Value:   getTemplateField(items[index], "From"),
				Message: "cyclic template inheritance",
			}
		}

		state[index] = true
		item := items[index]
		from := getTemplateField(item, "From")
		if from != "" {
			baseIndex, ok := classes[from]
			if !ok {
				return *new(T), &InstanceGroupTemplateError{
					List:    listName,
					Index:   index,
					Field:   "from",
					Value:   from,
					Message: fmt.Sprintf("unknown template class %q", from),
				}
			}
			base, err := resolve(baseIndex)
			if err != nil {
				return *new(T), err
			}
			item, err = mergeInstanceGroupTemplate(base, item)
			if err != nil {
				return *new(T), &InstanceGroupTemplateError{
					List:    listName,
					Index:   index,
					Field:   "from",
					Value:   from,
					Message: err.Error(),
				}
			}
		} else {
			copied, ok := deepcopy.Copy(item).(T)
			if !ok {
				return *new(T), fmt.Errorf("failed to deep-copy %s[%d]", listName, index)
			}
			item = copied
		}

		state[index] = false
		resolved[index] = item
		return item, nil
	}

	out := make([]T, 0, len(items))
	for i := range items {
		item, err := resolve(i)
		if err != nil {
			return nil, err
		}
		if getTemplateField(items[i], "Class") != "" {
			continue
		}
		setTemplateField(&item, "Class", "")
		setTemplateField(&item, "From", "")
		out = append(out, item)
	}

	return out, nil
}

func mergeInstanceGroupTemplate[T any](base, overlay T) (T, error) {
	item, ok := deepcopy.Copy(overlay).(T)
	if !ok {
		return *new(T), fmt.Errorf("failed to deep-copy template")
	}
	if err := mergo.Merge(&item, deepcopy.Copy(base)); err != nil {
		return *new(T), err
	}
	if err := mergeNamedListFields(reflect.ValueOf(&item).Elem(), reflect.ValueOf(base), reflect.ValueOf(overlay)); err != nil {
		return *new(T), err
	}
	return item, nil
}

func mergeNamedListFields(dst, base, overlay reflect.Value) error {
	dst = indirectValue(dst)
	base = indirectValue(base)
	overlay = indirectValue(overlay)
	if !dst.IsValid() || !base.IsValid() || !overlay.IsValid() || dst.Kind() != reflect.Struct || base.Kind() != reflect.Struct || overlay.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < dst.NumField(); i++ {
		structField := dst.Type().Field(i)
		dstField := dst.Field(i)
		baseField := base.Field(i)
		overlayField := overlay.Field(i)

		if dstField.Kind() == reflect.Struct {
			if err := mergeNamedListFields(dstField, baseField, overlayField); err != nil {
				return err
			}
		}

		if dstField.Kind() != reflect.Slice || !dstField.CanSet() {
			continue
		}

		mergeKey := structField.Tag.Get("patchMergeKey")
		if mergeKey == "" || !strings.Contains(structField.Tag.Get("patchStrategy"), "merge") {
			continue
		}

		merged, err := mergeNamedSlice(baseField, overlayField, mergeKey)
		if err != nil {
			return fmt.Errorf("failed to merge %s: %w", structField.Name, err)
		}
		dstField.Set(merged)
	}

	return nil
}

func mergeNamedSlice(base, overlay reflect.Value, mergeKey string) (reflect.Value, error) {
	base = indirectValue(base)
	overlay = indirectValue(overlay)

	switch {
	case !overlay.IsValid():
		return base, nil
	case !base.IsValid():
		return overlay, nil
	case overlay.Len() == 0:
		return base, nil
	case base.Len() == 0:
		return overlay, nil
	}

	overlayByKey := make(map[string]reflect.Value, overlay.Len())
	for i := 0; i < overlay.Len(); i++ {
		key, err := getMergeKey(overlay.Index(i), mergeKey)
		if err != nil {
			return reflect.Value{}, err
		}
		overlayByKey[key] = overlay.Index(i)
	}

	result := reflect.MakeSlice(base.Type(), 0, base.Len()+overlay.Len())
	used := make(map[string]struct{}, overlay.Len())
	for i := 0; i < base.Len(); i++ {
		key, err := getMergeKey(base.Index(i), mergeKey)
		if err != nil {
			return reflect.Value{}, err
		}
		if value, ok := overlayByKey[key]; ok {
			result = reflect.Append(result, value)
			used[key] = struct{}{}
			continue
		}
		result = reflect.Append(result, base.Index(i))
	}

	for i := 0; i < overlay.Len(); i++ {
		key, err := getMergeKey(overlay.Index(i), mergeKey)
		if err != nil {
			return reflect.Value{}, err
		}
		if _, ok := used[key]; ok {
			continue
		}
		result = reflect.Append(result, overlay.Index(i))
	}

	return result, nil
}

func getMergeKey(value reflect.Value, mergeKey string) (string, error) {
	value = indirectValue(value)
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return "", fmt.Errorf("merge-by-key requires struct list items")
	}
	field := findMergeKeyField(value, mergeKey)
	if !field.IsValid() || field.Kind() != reflect.String {
		return "", fmt.Errorf("merge key %q not found", mergeKey)
	}
	return field.String(), nil
}

func findMergeKeyField(value reflect.Value, mergeKey string) reflect.Value {
	value = indirectValue(value)
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	valueType := value.Type()
	for i := 0; i < value.NumField(); i++ {
		structField := valueType.Field(i)
		field := value.Field(i)
		if structField.Anonymous {
			if nested := findMergeKeyField(field, mergeKey); nested.IsValid() {
				return nested
			}
		}

		jsonName := strings.Split(structField.Tag.Get("json"), ",")[0]
		if jsonName == mergeKey || strings.EqualFold(structField.Name, mergeKey) {
			return field
		}
	}
	return reflect.Value{}
}

func indirectValue(value reflect.Value) reflect.Value {
	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

func (s *YtsaurusSpec) ResolveInstanceGroupTemplates() error {
	var err error
	if s.HTTPProxies, err = resolveInstanceGroupTemplates("httpProxies", s.HTTPProxies); err != nil {
		return err
	}
	if s.RPCProxies, err = resolveInstanceGroupTemplates("rpcProxies", s.RPCProxies); err != nil {
		return err
	}
	if s.TCPProxies, err = resolveInstanceGroupTemplates("tcpProxies", s.TCPProxies); err != nil {
		return err
	}
	if s.KafkaProxies, err = resolveInstanceGroupTemplates("kafkaProxies", s.KafkaProxies); err != nil {
		return err
	}
	if s.DataNodes, err = resolveInstanceGroupTemplates("dataNodes", s.DataNodes); err != nil {
		return err
	}
	if s.ExecNodes, err = resolveInstanceGroupTemplates("execNodes", s.ExecNodes); err != nil {
		return err
	}
	if s.TabletNodes, err = resolveInstanceGroupTemplates("tabletNodes", s.TabletNodes); err != nil {
		return err
	}
	return nil
}

func getTemplateField(item any, fieldName string) string {
	value := reflect.ValueOf(item)
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	field := findTemplateField(value, fieldName)
	if !field.IsValid() || field.Kind() != reflect.String {
		return ""
	}
	return field.String()
}

func setTemplateField(item any, fieldName, value string) {
	current := reflect.ValueOf(item)
	if current.Kind() != reflect.Pointer {
		return
	}
	current = current.Elem()
	field := findTemplateField(current, fieldName)
	if !field.IsValid() || !field.CanSet() || field.Kind() != reflect.String {
		return
	}
	field.SetString(value)
}

func findTemplateField(value reflect.Value, fieldName string) reflect.Value {
	if !value.IsValid() {
		return reflect.Value{}
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}
	}
	if field := value.FieldByName(fieldName); field.IsValid() {
		return field
	}
	valueType := value.Type()
	for i := range value.NumField() {
		structField := valueType.Field(i)
		if !structField.Anonymous {
			continue
		}
		if field := findTemplateField(value.Field(i), fieldName); field.IsValid() {
			return field
		}
	}
	return reflect.Value{}
}
