package gtkutil

import (
	"context"
	"sync"

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

// FuncBatch batches functions for calling.
type FuncBatch func()

// FuncBatcher creates a new FuncBatch.
func FuncBatcher(funcs ...func()) FuncBatch {
	var f FuncBatch
	f.Fs(funcs...)
	return f
}

// F batches f.
func (b *FuncBatch) F(f func()) {
	if *b == nil {
		*b = f
		return
	}

	next := *b
	*b = func() {
		next()
		f()
	}
}

// Fs batches multiple funcs.
func (b *FuncBatch) Fs(funcs ...func()) {
	for _, f := range funcs {
		b.F(f)
	}
}

// Cancellable describes a renewable and cancelable context. It is primarily
// used to box a context inside a widget for convenience.
type Cancellable interface {
	// Take returns the current context. This is useful for dropping this
	// context into a background task.
	Take() context.Context
	// OnRenew adds a function to be called once the context is renewed. If the
	// callback returns a non-nil function, then that function is called once
	// the context is cancelled.
	OnRenew(func(context.Context) (undo func())) (remove func())
}

// IsCancelled returns true if the cancellable is cancelled.
func IsCancelled(cancellable Cancellable) bool {
	return cancellable.Take().Err() != nil
}

// Canceller extends Cancellable to allow the user to control the context.
type Canceller interface {
	Cancellable
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

	ctx      context.Context
	cancel   context.CancelFunc
	renewFns renewFns
}

type renewFns struct {
	mu      sync.Mutex
	renewFn map[*funcKey]func() // -> undo
}

type funcKey struct{ f func(context.Context) func() }

func (r *renewFns) doAll(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for k := range r.renewFn {
		r.renewFn[k] = k.f(ctx)
	}
}

func (r *renewFns) cancelAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for k, cancel := range r.renewFn {
		if cancel != nil {
			cancel()
			r.renewFn[k] = nil
		}
	}
}

func (r *renewFns) add(ctx context.Context, f func(context.Context) func()) *funcKey {
	k := &funcKey{f}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.init()
	if ctx.Err() == nil {
		r.renewFn[k] = f(ctx)
	} else {
		r.renewFn[k] = nil
	}

	return k
}

func (r *renewFns) init() {
	if r.renewFn == nil {
		r.renewFn = map[*funcKey]func(){}
	}
}

func (r *renewFns) remove(k *funcKey) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.renewFn, k)
}

// WithVisibility creates a new context that is canceled when the widget is
// hidden.
func WithVisibility(ctx context.Context, widget gtk.Widgetter) Canceller {
	c := WithCanceller(ctx)
	w := gtk.BaseWidget(widget)
	if !w.Mapped() && !w.Realized() {
		c.Cancel()
	}
	w.ConnectMap(c.Renew)
	w.ConnectRealize(c.Renew)
	w.ConnectUnrealize(c.Cancel)
	return c
}

// WithCanceller wraps around a context.
func WithCanceller(ctx context.Context) Canceller {
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
	if c.cancel == nil {
		c.mu.Unlock()
		return
	}

	c.cancel()
	c.cancel = nil
	c.mu.Unlock()

	c.renewFns.cancelAll()
}

func (c *canceller) OnRenew(f func(context.Context) func()) func() {
	k := c.renewFns.add(c.Take(), f)
	return func() { c.renewFns.remove(k) }
}

func (c *canceller) Renew() {
	c.mu.Lock()
	if c.cancel != nil {
		c.mu.Unlock()
		return
	}

	c.ctx, c.cancel = context.WithCancel(c.old)
	ctx := c.ctx
	c.mu.Unlock()

	c.renewFns.doAll(ctx)
}
