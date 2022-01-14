// Package registry contains a registry of callbacks.
package registry

import "sync"

// Value is the boxed type.
type Value struct {
	V interface{}
	_ [0]sync.Mutex
	r *Registry
}

// Delete deletes the box itself from the containing map. The value is
// invalidated after the call finishes.
func (v *Value) Delete() {
	delete(v.r.m, v)
	v.V = nil
}

// gc thrashing, the game

// M is the map type.
type Registry struct {
	m map[*Value]interface{}
}

// New creates a new M instance.
func New(cap int) Registry {
	return Registry{
		m: make(map[*Value]interface{}, cap),
	}
}

// IsEmpty returns true if the Registry is empty.
func (r *Registry) IsEmpty() bool { return len(r.m) == 0 }

// Each iterates over the map.
func (r *Registry) Each(f func(interface{}, interface{})) {
	r.EachValue(func(v *Value, meta interface{}) { f(v.V, meta) })
}

// EachValue iterates over the map and gives the raw Value.
func (r *Registry) EachValue(f func(*Value, interface{})) {
	for v, metadata := range r.m {
		f(v, metadata)
	}
}

// Add adds the given interface and returns a new and unique box that identifies
// it.
func (r *Registry) Add(v, meta interface{}) *Value {
	if r.m == nil {
		r.m = make(map[*Value]interface{})
	}

	b := &Value{V: v, r: r}
	r.m[b] = meta
	return b
}
