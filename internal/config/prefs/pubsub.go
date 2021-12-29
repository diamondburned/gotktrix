package prefs

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotktrix/internal/gtkutil"
)

type funcBox struct{ f func() }

// Pubsub provides a simple publish-subscribe API. This instance is safe to use
// concurrently.
type Pubsub struct {
	funcs map[*funcBox]struct{}
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
	gtkutil.InvokeMain(func() {
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
		unsub = p.Subscribe(f)
	})
	if w.Mapped() {
		unsub = p.Subscribe(f)
	}

	w.ConnectUnmap(func() {
		if unsub != nil {
			unsub()
			unsub = nil
		}
	})
}

// Subscribe adds f into the pubsub's subscription queue. f will always be
// invoked in the main thread.
func (p *Pubsub) Subscribe(f func()) (rm func()) {
	b := &funcBox{f}

	gtkutil.InvokeMain(func() {
		p.funcs[b] = struct{}{}
		f()
	})

	return func() {
		gtkutil.InvokeMain(func() {
			delete(p.funcs, b)
		})
	}
}

// SubscribeInit is like Subscribe, except you can't unsubscribe, the callback
// is not called, and the method is not thread-safe. It is only meant to be
// called in init() functions.
func (p *Pubsub) SubscribeInit(f func()) {
	b := &funcBox{f}
	p.funcs[b] = struct{}{}
}
