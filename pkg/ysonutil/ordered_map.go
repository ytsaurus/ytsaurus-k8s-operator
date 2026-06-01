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
	event, err := r.Next(true)
	if err != nil {
		return err
	}
	if event != yson.EventBeginMap {
		return fmt.Errorf("ysonutil: expected map, got event %v", event)
	}

	m.keys = m.keys[:0]
	if m.values == nil {
		m.values = make(map[string]any)
	} else {
		for k := range m.values {
			delete(m.values, k)
		}
	}

	for {
		ok, err := r.NextKey()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		key := r.String()

		raw, err := r.NextRawValue()
		if err != nil {
			return err
		}
		// Copy raw bytes because NextRawValue returns a slice valid only
		// until the next call to the reader.
		rawCopy := make([]byte, len(raw))
		copy(rawCopy, raw)

		var value any
		if err := yson.Unmarshal(rawCopy, &value); err != nil {
			return err
		}

		m.keys = append(m.keys, key)
		m.values[key] = value
	}

	return nil
}
