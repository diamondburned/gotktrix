package registry

import "sync"

// Value is the boxed type.
type Value struct {
	V interface{}
	_ [0]sync.Mutex
	m M
}

// Delete deletes the box itself from the containing map. The value is
// invalidated after the call finishes.
func (v *Value) Delete() {
	delete(v.m, v)
	v.V = nil
}

// gc thrashing, the game

// M is the map type.
type M map[*Value]struct{}

// Each iterates over the map.
func (m M) Each(f func(interface{})) {
	for v := range m {
		f(v.V)
	}
}

// Add adds the given interface and returns a new and unique box that identifies
// it.
func (m M) Add(v interface{}) *Value {
	b := &Value{V: v, m: m}
	m[b] = struct{}{}
	return b
}
