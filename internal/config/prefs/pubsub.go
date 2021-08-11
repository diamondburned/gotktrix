package prefs

import (
	"sync"

	"github.com/diamondburned/gotk4/pkg/core/glib"
)

// Pubsub provides a simple publish-subscribe API. This instance is safe to use
// concurrently.
type Pubsub struct {
	funcs map[uint64]func()
	count uint64
	mu    sync.Mutex
}

// NewPubsub creates a new Pubsub instance.
func NewPubsub() *Pubsub {
	return &Pubsub{
		funcs: make(map[uint64]func()),
	}
}

// Publish publishes changes to all subscribe routines.
func (p *Pubsub) Publish() {
	glib.IdleAddPriority(glib.PriorityHighIdle, func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		for _, f := range p.funcs {
			f()
		}
	})
}

// Subscribe subscribes the given callback to changes. If rm is called, then the
// subscription is removed. The given callback will be called once in the
// receiving goroutine to signal a change. It is guaranteed for the callback to
// only be consistently called on that goroutine.
func (p *Pubsub) Subscribe(f func()) (rm func()) {
	p.mu.Lock()
	id := p.count
	p.funcs[id] = f
	p.count++
	p.mu.Unlock()

	glib.IdleAddPriority(glib.PriorityHighIdle, f)

	return func() {
		p.mu.Lock()
		delete(p.funcs, id)
		p.mu.Unlock()
	}
}

// Connect binds f to the lifetime of the given object.
func (p *Pubsub) Connect(obj glib.Objector, f func()) {
	var unsub func()
	obj.Connect("map", func() {
		unsub = p.Subscribe(f)
	})
	obj.Connect("destroy", func() {
		unsub()
	})
}
