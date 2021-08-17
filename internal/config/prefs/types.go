package prefs

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
)

// Bool is a preference property of type boolean.
type Bool struct {
	Pubsub
	PropMeta
	v uint32
}

// NewBool creates a new boolean with the given default value and properties.
func NewBool(v bool, prop PropMeta) *Bool {
	prop.validate()

	b := &Bool{
		Pubsub:   *NewPubsub(),
		PropMeta: prop,

		v: boolToUint32(v),
	}

	registerProp(b)
	return b
}

// Publish publishes the new boolean.
func (b *Bool) Publish(v bool) {
	atomic.StoreUint32(&b.v, boolToUint32(v))
	b.Pubsub.Publish()
}

// Value loads the internal boolean.
func (b *Bool) Value() bool {
	return atomic.LoadUint32(&b.v) != 0
}

func boolToUint32(b bool) (u uint32) {
	if b {
		u = 1
	}
	return
}

// String is a preference property of type string.
type String struct {
	Pubsub
	StringMeta
	val string
	mut sync.Mutex
}

// StringMeta is the metadata of a string.
type StringMeta struct {
	PropMeta
	Validate func(string) error
}

// NewString creates a new String instance.
func NewString(def string, prop StringMeta) *String {
	l := &String{
		Pubsub:     *NewPubsub(),
		StringMeta: prop,

		val: def,
	}

	if prop.Validate != nil {
		if err := prop.Validate(def); err != nil {
			log.Panicf("default value %q fails validation: %v", def, err)
		}
	}

	registerProp(l)

	return l
}

// Publish publishes the new string value. An error is returned and nothing is
// published if the string fails the verifier.
func (s *String) Publish(v string) error {
	if s.Validate != nil {
		if err := s.Validate(v); err != nil {
			return err
		}
	}

	s.mut.Lock()
	s.val = v
	s.mut.Unlock()

	s.Pubsub.Publish()
	return nil
}

// Value returns the internal string value.
func (s *String) Value() string {
	s.mut.Lock()
	defer s.mut.Unlock()

	return s.val
}

// EnumList is a preference property of type stringer.
type EnumList struct {
	Pubsub
	EnumListMeta
	opts map[fmt.Stringer]struct{}
	val  fmt.Stringer
	mut  sync.RWMutex
}

// EnumListMeta is the metadata of an EnumList.
type EnumListMeta struct {
	PropMeta
	Options []fmt.Stringer
}

// NewEnumList creates a new EnumList instance.
func NewEnumList(def fmt.Stringer, prop EnumListMeta) *EnumList {
	l := &EnumList{
		Pubsub:       *NewPubsub(),
		EnumListMeta: prop,

		opts: make(map[fmt.Stringer]struct{}, len(prop.Options)),
		val:  def,
	}

	for _, opt := range prop.Options {
		l.opts[opt] = struct{}{}
	}

	if !l.IsValid(def) {
		log.Panicf("invalid default value %q, possible: %q.", def, l.Options)
	}

	registerProp(l)

	return l
}

// Publish publishes the new value. If the value isn't within Options, then the
// method will panic.
func (l *EnumList) Publish(v fmt.Stringer) {
	if !l.IsValid(v) {
		log.Panicf("publishing invalid value %q, possible: %q.", v, l.Options)
	}

	l.mut.Lock()
	l.val = v
	l.mut.Unlock()

	l.Pubsub.Publish()
}

// Value gets the current enum value.
func (l *EnumList) Value() fmt.Stringer {
	l.mut.RLock()
	defer l.mut.RUnlock()

	return l.val
}

// IsValid returns true if the given value is a valid enum value.
func (l *EnumList) IsValid(v fmt.Stringer) bool {
	_, ok := l.opts[v]
	return ok
}
