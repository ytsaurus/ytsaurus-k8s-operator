package ysonutil

import (
	"fmt"

	"go.ytsaurus.tech/yt/go/yson"
)

// OrderedMap is a map[string]any that preserves insertion order.
// It implements yson.StreamMarshaler and yson.StreamUnmarshaler so that
// key order is maintained across YSON encode/decode cycles.
type OrderedMap struct {
	keys   []string
	values map[string]any
}

// NewOrderedMap returns an empty OrderedMap.
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{
		values: make(map[string]any),
	}
}

// Set adds or updates the value for key. New keys are appended to the end.
func (m *OrderedMap) Set(key string, value any) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Get returns the value associated with key and whether it was found.
func (m *OrderedMap) Get(key string) (any, bool) {
	v, ok := m.values[key]
	return v, ok
}

// Keys returns the keys in insertion order.
func (m *OrderedMap) Keys() []string {
	result := make([]string, len(m.keys))
	copy(result, m.keys)
	return result
}

// Len returns the number of entries.
func (m *OrderedMap) Len() int {
	return len(m.keys)
}

// MarshalYSON implements yson.StreamMarshaler.
// Keys are written in insertion order.
func (m *OrderedMap) MarshalYSON(w *yson.Writer) error {
	w.BeginMap()
	for _, k := range m.keys {
		w.MapKeyString(k)
		w.Any(m.values[k])
	}
	w.EndMap()
	return w.Err()
}

// UnmarshalYSON implements yson.StreamUnmarshaler.
// Keys are stored in the order they appear in the YSON stream.
func (m *OrderedMap) UnmarshalYSON(r *yson.Reader) error {
	m.keys = m.keys[:0]
	if m.values == nil {
		m.values = make(map[string]any)
	} else {
		for k := range m.values {
			delete(m.values, k)
		}
	}

	parsed, err := unmarshalValueFromReader(r)
	if err != nil {
		return err
	}

	ordered, ok := parsed.(*OrderedMap)
	if !ok {
		return fmt.Errorf("ysonutil: expected map, got %T", parsed)
	}

	m.keys = append(m.keys, ordered.keys...)
	for key, value := range ordered.values {
		m.values[key] = value
	}

	return nil
}

func unmarshalValueFromReader(r *yson.Reader) (any, error) {
	event, err := r.Next(false)
	if err != nil {
		return nil, err
	}

	if event == yson.EventBeginAttrs {
		attrs := make(map[string]any)
		for {
			ok, err := r.NextKey()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}

			key := r.String()
			value, err := unmarshalValueFromReader(r)
			if err != nil {
				return nil, err
			}
			attrs[key] = value
		}

		event, err = r.Next(false)
		if err != nil {
			return nil, err
		}
		if event != yson.EventEndAttrs {
			return nil, fmt.Errorf("ysonutil: expected end attrs, got %v", event)
		}

		value, err := unmarshalValueFromReader(r)
		if err != nil {
			return nil, err
		}

		return &yson.ValueWithAttrs{
			Attrs: attrs,
			Value: value,
		}, nil
	}

	switch event {
	case yson.EventBeginMap:
		result := NewOrderedMap()
		for {
			ok, err := r.NextKey()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}

			key := r.String()
			value, err := unmarshalValueFromReader(r)
			if err != nil {
				return nil, err
			}
			result.keys = append(result.keys, key)
			result.values[key] = value
		}

		event, err = r.Next(false)
		if err != nil {
			return nil, err
		}
		if event != yson.EventEndMap {
			return nil, fmt.Errorf("ysonutil: expected end map, got %v", event)
		}

		return result, nil
	case yson.EventBeginList:
		var result []any
		for {
			ok, err := r.NextListItem()
			if err != nil {
				return nil, err
			}
			if !ok {
				break
			}

			value, err := unmarshalValueFromReader(r)
			if err != nil {
				return nil, err
			}
			result = append(result, value)
		}

		event, err = r.Next(false)
		if err != nil {
			return nil, err
		}
		if event != yson.EventEndList {
			return nil, fmt.Errorf("ysonutil: expected end list, got %v", event)
		}

		return result, nil
	case yson.EventLiteral:
		switch r.Type() {
		case yson.TypeString:
			return r.String(), nil
		case yson.TypeInt64:
			return r.Int64(), nil
		case yson.TypeUint64:
			return r.Uint64(), nil
		case yson.TypeFloat64:
			return r.Float64(), nil
		case yson.TypeBool:
			return r.Bool(), nil
		case yson.TypeEntity:
			return nil, nil
		default:
			return nil, fmt.Errorf("ysonutil: unsupported literal type %v", r.Type())
		}
	default:
		return nil, fmt.Errorf("ysonutil: unexpected event %v", event)
	}
}
