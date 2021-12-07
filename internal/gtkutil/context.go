package gtkutil

import (
	"context"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// IdleCtx runs the given callback inside the main loop only if the context has
// not expired.
func IdleCtx(ctx context.Context, f func()) {
	glib.IdleAdd(func() {
		select {
		case <-ctx.Done():

		default:
			f()
		}
	})
}

// ContextTaker describes a context.Context that can be taken.
type ContextTaker interface {
	context.Context
	// Take returns the current context. This is useful for dropping this
	// context into a background task.
	Take() context.Context
}

// Cancellable describes a renewable and cancelable context. It is primarily
// used to box a context inside a widget for convenience.
type Cancellable interface {
	context.Context
	ContextTaker

	// Renew cancels the previous context, if any, and restarts that context
	// using the one given into WithCanceller.
	Renew()
	// Cancel cancels the canceler. If the canceler is a zero-value, then this
	// method does nothing.
	Cancel()
}

type canceller struct {
	mu  sync.Mutex
	old context.Context

	ctx    context.Context
	cancel context.CancelFunc
}

// WithVisibility creates a new context that is canceled when the widget is
// hidden.
func WithVisibility(ctx context.Context, widget gtk.Widgetter) Cancellable {
	c := WithCanceller(ctx)
	w := gtk.BaseWidget(widget)
	w.ConnectMap(c.Renew)
	w.ConnectRealize(c.Renew)
	w.ConnectUnrealize(c.Cancel)
	return c
}

// WithCanceller wraps around a context.
func WithCanceller(ctx context.Context) Cancellable {
	old := ctx
	ctx, cancel := context.WithCancel(old)

	return &canceller{
		old:    old,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (c *canceller) Take() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.ctx
}

func (c *canceller) Cancel() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
}

func (c *canceller) Renew() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel == nil {
		c.ctx, c.cancel = context.WithCancel(c.old)
	}
}

func (c *canceller) Done() <-chan struct{} {
	return c.Take().Done()
}

func (c *canceller) Err() error {
	return c.Take().Err()
}

func (c *canceller) Deadline() (time.Time, bool) {
	return c.old.Deadline()
}

func (c *canceller) Value(k interface{}) interface{} {
	return c.old.Value(k)
}
