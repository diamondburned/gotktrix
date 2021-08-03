package prefs

import "sync/atomic"

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
