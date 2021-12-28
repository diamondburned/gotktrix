package prefs

import (
	"encoding/json"
	"errors"
	"log"
	"sync"
	"sync/atomic"

	"golang.org/x/text/message"
)

// ErrInvalidAnyType is returned by a preference property if it has the wrong
// type.
var ErrInvalidAnyType = errors.New("incorrect value type")

// Bool is a preference property of type boolean.
type Bool struct {
	Pubsub
	PropMeta
	v uint32
}

// NewBool creates a new boolean with the given default value and properties.
func NewBool(v bool, prop PropMeta) *Bool {
	validateMeta(prop)

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

func (b *Bool) MarshalJSON() ([]byte, error) { return json.Marshal(b.Value()) }

func (b *Bool) UnmarshalJSON(blob []byte) error {
	var v bool
	if err := json.Unmarshal(blob, &v); err != nil {
		return err
	}
	b.Publish(v)
	return nil
}

// AnyValue implements Prop.
func (b *Bool) AnyValue() interface{} { return b.Value() }

// AnyPublish implements Prop.
func (b *Bool) AnyPublish(v interface{}) error {
	bv, ok := v.(bool)
	if !ok {
		return ErrInvalidAnyType
	}
	b.Publish(bv)
	return nil
}

func boolToUint32(b bool) (u uint32) {
	if b {
		u = 1
	}
	return
}

// Int is a preference property of type int.
type Int struct {
	Pubsub
	IntMeta
	v int32
}

// IntMeta wraps PropMeta for Int.
type IntMeta struct {
	Name        message.Reference
	Section     message.Reference
	Description message.Reference
	Min         int
	Max         int
	Slider      bool
}

// Meta returns the PropMeta for IntMeta. It implements Prop.
func (m IntMeta) Meta() PropMeta {
	return PropMeta{
		Name:        m.Name,
		Section:     m.Section,
		Description: m.Description,
	}
}

// NewInt creates a new int(32) with the given default value and properties.
func NewInt(v int, meta IntMeta) *Int {
	validateMeta(meta.Meta())

	b := &Int{
		Pubsub:  *NewPubsub(),
		IntMeta: meta,

		v: int32(v),
	}

	registerProp(b)
	return b
}

// Publish publishes the new int.
func (i *Int) Publish(v int) {
	atomic.StoreInt32(&i.v, int32(v))
	i.Pubsub.Publish()
}

// Value loads the internal int.
func (i *Int) Value() int {
	return int(atomic.LoadInt32(&i.v))
}

func (i *Int) MarshalJSON() ([]byte, error) { return json.Marshal(i.Value()) }

func (i *Int) UnmarshalJSON(b []byte) error {
	var v int
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	i.Publish(v)
	return nil
}

// StringMeta is the metadata of a string.
type StringMeta struct {
	Name        message.Reference
	Section     message.Reference
	Description message.Reference
	Placeholder message.Reference
	Validate    func(string) error
	Multiline   bool
}

// Meta returns the PropMeta for StringMeta. It implements Prop.
func (m StringMeta) Meta() PropMeta {
	return PropMeta{
		Name:        m.Name,
		Section:     m.Section,
		Description: m.Description,
	}
}

// String is a preference property of type string.
type String struct {
	Pubsub
	StringMeta
	val string
	mut sync.Mutex
}

// NewString creates a new String instance.
func NewString(def string, prop StringMeta) *String {
	validateMeta(prop.Meta())

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

func (s *String) MarshalJSON() ([]byte, error) { return json.Marshal(s.Value()) }

func (s *String) UnmarshalJSON(blob []byte) error {
	var v string
	if err := json.Unmarshal(blob, &v); err != nil {
		return err
	}
	s.Publish(v)
	return nil
}

/*
// EnumList is a preference property of type stringer.
type EnumList struct {
	Pubsub
	EnumListMeta
	opts []string
	val  fmt.Stringer
	mut  sync.RWMutex
}

// EnumListMeta is the metadata of an EnumList.
type EnumListMeta struct {
	PropMeta
	Options []fmt.Stringer
}

// EnumString is a string type for EnumList.
type EnumString string

// String returns itself.
func (s EnumString) String() string { return string(s) }

// NewEnumList creates a new EnumList instance.
func NewEnumList(def fmt.Stringer, prop EnumListMeta) *EnumList {
	l := &EnumList{
		Pubsub:       *NewPubsub(),
		EnumListMeta: prop,

		opts: make([]string, len(prop.Options)),
		val:  def,
	}

	for i, opt := range prop.Options {
		l.opts[i] = opt.String()
	}

	if !l.IsValid(def) {
		log.Panicf("invalid default value %q, possible: %q.", def, l.Options)
	}

	registerProp(l)

	return l
}

// PossibleValueStrings returns the possible enum values as strings.
func (l *EnumList) PossibleValueStrings() []string {
	return l.opts
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

func (l *EnumList) MarshalJSON() ([]byte, error) {
	return json.Marshal(l.Value().String())
}

func (l *EnumList) UnmarshalJSON(blob []byte) error {
	l.mut.RLock()
	// TODO: refactor this once generics come out
	t := reflect.TypeOf(l.val)
	l.mut.Unlock()

	vptr := reflect.New(t)

	if err := json.Unmarshal(blob, vptr.Interface()); err != nil {
		return err
	}

	// source type is a stringerr
	v := vptr.Elem().Interface().(fmt.Stringer)

	if !l.IsValid(v) {
		return fmt.Errorf("enum %q is not a known values", v.String())
	}

	l.Publish(v)
	return nil
}

// IsValid returns true if the given value is a valid enum value.
func (l *EnumList) IsValid(v fmt.Stringer) bool {
	return l.isValid(v.String())
}

func (l *EnumList) isValid(str string) bool {
	for _, opt := range l.opts {
		if opt == str {
			return true
		}
	}
	return false
}
*/
