package ypatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"go.ytsaurus.tech/yt/go/yson"
)

type OrderedValue struct {
	Attributes OrderedMap
	Value      any
}

type OrderedMap struct {
	Values map[string]any
	Keys   []string
}

var (
	_ json.Marshaler   = (*OrderedValue)(nil)
	_ json.Unmarshaler = (*OrderedValue)(nil)

	_ yson.StreamMarshaler   = (*OrderedValue)(nil)
	_ yson.StreamUnmarshaler = (*OrderedValue)(nil)

	_ json.Marshaler   = (*OrderedMap)(nil)
	_ json.Unmarshaler = (*OrderedMap)(nil)

	_ yson.StreamMarshaler   = (*OrderedMap)(nil)
	_ yson.StreamUnmarshaler = (*OrderedMap)(nil)
)

func (m *OrderedMap) Get(key string) (any, bool) {
	value, ok := m.Values[key]
	return value, ok
}

func (m *OrderedMap) Set(key string, value any) {
	if m.Values == nil {
		m.Values = map[string]any{}
	}
	if _, ok := m.Values[key]; !ok {
		m.Keys = append(m.Keys, key)
	}
	m.Values[key] = value
}

func (m *OrderedMap) Delete(key string) (any, bool) {
	if value, ok := m.Values[key]; ok {
		delete(m.Values, key)
		if index := slices.Index(m.Keys, key); index >= 0 {
			m.Keys = slices.Delete(m.Keys, index, index+1)
		}
		return value, true
	}
	return nil, false
}

func (m *OrderedMap) GetValue(key string) (any, bool) {
	if value, found := m.Values[key]; found {
		if av, ok := value.(*OrderedValue); ok {
			value = av.Value
		}
		return value, true
	}
	return nil, false
}

func (m *OrderedMap) GetMap(key string) (*OrderedMap, bool) {
	value, ok := m.Values[key].(*OrderedMap)
	return value, ok
}

func (m *OrderedMap) Merge(x map[string]any) {
	for _, key := range slices.Sorted(maps.Keys(x)) {
		m.Set(key, x[key])
	}
}

func (m *OrderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	encoder := json.NewEncoder(&buf)
	for i, key := range m.Keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encoder.Encode(key); err != nil {
			return nil, err
		}
		buf.WriteByte(':')
		if err := encoder.Encode(m.Values[key]); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func (m *OrderedMap) UnmarshalJSON(b []byte) error {
	m.Values = map[string]any{}
	if err := json.Unmarshal(b, &m.Values); err != nil {
		return err
	}
	m.Keys = slices.Sorted(maps.Keys(m.Values))
	return nil
}

func (m *OrderedMap) MarshalYSON(w *yson.Writer) error {
	w.BeginMap()
	for _, key := range m.Keys {
		w.MapKeyString(key)
		w.Any(m.Values[key])
	}
	w.EndMap()
	return w.Err()
}

func (m *OrderedMap) UnmarshalYSON(r *yson.Reader) error {
	if err := expectEvent(r, yson.EventBeginMap); err != nil {
		return err
	}
	if err := m.unmarshalBody(r); err != nil {
		return err
	}
	if err := expectEvent(r, yson.EventEndMap); err != nil {
		return err
	}
	return nil
}

func (m *OrderedMap) unmarshalBody(r *yson.Reader) error {
	for {
		hasKey, err := r.NextKey()
		if err != nil {
			return err
		}
		if !hasKey {
			break
		}
		key := r.String()
		if value, err := decodeInto(r, m.Values[key]); err != nil {
			return err
		} else {
			m.Set(key, value)
		}
	}
	return nil
}

func (v *OrderedValue) GetMap() (*OrderedMap, bool) {
	value, ok := v.Value.(*OrderedMap)
	return value, ok
}

func (v *OrderedValue) MarshalJSON() ([]byte, error) {
	if v.Attributes.Values == nil {
		return json.Marshal(v.Value)
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	if _, err := buf.WriteString(`{"$attributes":`); err != nil {
		return nil, err
	}
	if err := encoder.Encode(v.Attributes); err != nil {
		return nil, err
	}
	if _, err := buf.WriteString(`,"$value":`); err != nil {
		return nil, err
	}
	if err := encoder.Encode(v.Value); err != nil {
		return nil, err
	}
	if _, err := buf.WriteString(`}`); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (v *OrderedValue) UnmarshalJSON(b []byte) error {
	var val any
	if err := json.Unmarshal(b, &val); err != nil {
		return err
	}
	if m, ok := val.(map[string]any); ok && len(m) == 2 {
		attributes, hasAttributes := m["$attributes"].(map[string]any)
		value, hasValue := m["$value"]
		if hasAttributes && hasValue {
			v.Attributes = OrderedMap{attributes, slices.Sorted(maps.Keys(attributes))}
			v.Value = value
			return nil
		}
	}
	v.Attributes = OrderedMap{}
	v.Value = val
	return nil
}

func (v *OrderedValue) MarshalYSON(w *yson.Writer) error {
	if v.Attributes.Values != nil {
		w.BeginAttrs()
		for _, key := range v.Attributes.Keys {
			w.MapKeyString(key)
			w.Any(v.Attributes.Values[key])
		}
		w.EndAttrs()
	}
	w.Any(v.Value)
	return w.Err()
}

func (v *OrderedValue) UnmarshalYSON(r *yson.Reader) error {
	event, err := r.Next(false)
	if err != nil {
		return err
	}
	if event == yson.EventBeginAttrs {
		if v.Attributes.Values == nil {
			v.Attributes.Values = make(map[string]any) // Preserve "<>".
		}
		err = v.Attributes.unmarshalBody(r)
		if err != nil {
			return err
		}
		if err := expectEvent(r, yson.EventEndAttrs); err != nil {
			return err
		}
	} else {
		r.Undo(event)
	}
	v.Value, err = decodeInto(r, v.Value)
	return err
}

func decodeAny(r *yson.Reader) (any, error) {
	event, err := r.Next(false)
	if err != nil {
		return nil, err
	}
	switch event {
	case yson.EventBeginAttrs:
		r.Undo(event)
		v := OrderedValue{}
		if err := v.UnmarshalYSON(r); err != nil {
			return nil, err
		}
		return &v, nil
	case yson.EventBeginMap:
		m := OrderedMap{}
		if err := m.unmarshalBody(r); err != nil {
			return nil, err
		}
		if err := expectEvent(r, yson.EventEndMap); err != nil {
			return nil, err
		}
		return &m, nil
	case yson.EventBeginList:
		var list []any
		for {
			hasItem, err := r.NextListItem()
			if err != nil {
				return nil, err
			}
			if !hasItem {
				break
			}
			item, err := decodeAny(r)
			if err != nil {
				return nil, err
			}
			list = append(list, item)
		}
		if err := expectEvent(r, yson.EventEndList); err != nil {
			return nil, err
		}
		return list, nil
	case yson.EventLiteral:
		switch r.Type() {
		case yson.TypeEntity:
			return nil, nil
		case yson.TypeBool:
			return r.Bool(), nil
		case yson.TypeString:
			return r.String(), nil
		case yson.TypeInt64:
			return r.Int64(), nil
		case yson.TypeUint64:
			return r.Uint64(), nil
		case yson.TypeFloat64:
			return r.Float64(), nil
		}
	}
	return nil, fmt.Errorf("unexpected YSON event %v", event)
}

func decodeInto(r *yson.Reader, value any) (any, error) {
	switch v := value.(type) {
	case *OrderedValue:
		err := v.UnmarshalYSON(r)
		return v, err
	case *OrderedMap:
		switch peekEvent(r) {
		case yson.EventBeginMap:
			err := v.UnmarshalYSON(r)
			return v, err
		case yson.EventBeginAttrs:
			av := OrderedValue{Value: value}
			err := av.UnmarshalYSON(r)
			return &av, err
		}
	}
	return decodeAny(r)
}

func peekEvent(r *yson.Reader) yson.Event {
	event, err := r.Next(false)
	if err != nil {
		return yson.EventEOF
	}
	r.Undo(event)
	return event
}

func expectEvent(r *yson.Reader, expected yson.Event) error {
	if event, err := r.Next(false); err != nil {
		return err
	} else if event != expected {
		return fmt.Errorf("expected YSON event %v, got %v", expected, event)
	}
	return nil
}
