package v1

import (
	"fmt"
	"reflect"

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
	return item, nil
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
