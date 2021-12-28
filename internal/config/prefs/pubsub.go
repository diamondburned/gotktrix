package prefs

import (
	"sync"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type funcBox struct{ f func() }

// Pubsub provides a simple publish-subscribe API. This instance is safe to use
// concurrently.
type Pubsub struct {
	funcs map[*funcBox]struct{}
	mu    sync.Mutex
}

// NewPubsub creates a new Pubsub instance.
func NewPubsub() *Pubsub {
	return &Pubsub{
		funcs: make(map[*funcBox]struct{}),
	}
}

// Pubsubber returns itself.
func (p *Pubsub) Pubsubber() *Pubsub { return p }

// Publish publishes changes to all subscribe routines.
func (p *Pubsub) Publish() {
	glib.IdleAddPriority(glib.PriorityHighIdle, func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		for f := range p.funcs {
			f.f()
		}
	})
}

// SubscribeWidget subscribes the given widget and callback to changes. If rm is
// called, then the subscription is removed. The given callback will be called
// once in the receiving goroutine to signal a change. It is guaranteed for the
// callback to only be consistently called on that goroutine.
func (p *Pubsub) SubscribeWidget(widget gtk.Widgetter, f func()) {
	var unsub func()
	w := gtk.BaseWidget(widget)

	w.ConnectMap(func() {
		unsub = p.subscribe(f, true)
	})
	if w.Mapped() {
		unsub = p.subscribe(f, true)
	}

	w.ConnectUnmap(func() {
		if unsub != nil {
			unsub()
			unsub = nil
		}
	})
}

func (p *Pubsub) subscribe(f func(), mainThread bool) (rm func()) {
	b := &funcBox{f}

	p.mu.Lock()
	p.funcs[b] = struct{}{}
	p.mu.Unlock()

	if mainThread {
		f()
	} else {
		glib.IdleAddPriority(glib.PriorityHighIdle, f)
	}

	return func() {
		p.mu.Lock()
		delete(p.funcs, b)
		p.mu.Unlock()
	}
}

// SubscribeInit is like Subscribe, except you can't unsubscribe, the callback
// is not called, and the method is not thread-safe. It is only meant to be
// called in init() functions.
func (p *Pubsub) SubscribeInit(f func()) {
	b := &funcBox{f}
	p.funcs[b] = struct{}{}
}
