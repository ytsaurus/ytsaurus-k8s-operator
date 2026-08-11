package ypatch

import (
	"fmt"
	"slices"

	"go.ytsaurus.tech/yt/go/yson"
)

type OrderedValue struct {
	Attrs OrderedMap
	Value any
}

type OrderedMap struct {
	Values map[string]any
	Keys   []string
}

var (
	_ yson.StreamMarshaler   = (*OrderedValue)(nil)
	_ yson.StreamUnmarshaler = (*OrderedValue)(nil)

	_ yson.StreamMarshaler   = (*OrderedMap)(nil)
	_ yson.StreamUnmarshaler = (*OrderedMap)(nil)
)

func (m *OrderedMap) Get(key string) (any, bool) {
	value, ok := m.Values[key]
	return value, ok
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

func (v *OrderedValue) MarshalYSON(w *yson.Writer) error {
	if v.Attrs.Values != nil {
		w.BeginAttrs()
		for _, key := range v.Attrs.Keys {
			w.MapKeyString(key)
			w.Any(v.Attrs.Values[key])
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
		if v.Attrs.Values == nil {
			v.Attrs.Values = make(map[string]any) // Preserve "<>".
		}
		err = v.Attrs.unmarshalBody(r)
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
