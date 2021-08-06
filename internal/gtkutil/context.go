package gtkutil

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/core/glib"
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

// Canceler wraps around a context and a callback.
type Canceler struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewCanceler creates a new Canceler.
func NewCanceler() Canceler {
	return WithCanceler(context.Background())
}

// WithCanceler wraps around a context.
func WithCanceler(ctx context.Context) Canceler {
	ctx, cancel := context.WithCancel(ctx)
	return Canceler{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Context returns the current canceler's context.
func (c Canceler) Context() context.Context {
	if c.ctx != nil {
		return c.ctx
	}
	return context.Background()
}

// Cancel cancels the canceler. If the canceler is a zero-value, then this
// method does nothing.
func (c Canceler) Cancel() {
	if c.cancel != nil {
		c.cancel()
	}
}

// Renew cancels the previous context, if any, and starts a new context.
func (c *Canceler) Renew() {
	c.RenewWith(context.Background())
}

// RenewWith renews Canceler with the given parent context.
func (c *Canceler) RenewWith(ctx context.Context) {
	c.Cancel()

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
}
