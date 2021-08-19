package autoscroll

import (
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type bottomedFunc struct {
	f func(bool)
}

// Window describes an automatically scrolled window.
type Window struct {
	*gtk.ScrolledWindow
	vadj gtk.Adjustment

	onBottomed map[*bottomedFunc]struct{}

	bottomed   bool // :floshed:
	willScroll bool
}

func NewWindow() *Window {
	sw := Window{ScrolledWindow: gtk.NewScrolledWindow()}
	sw.vadj = *sw.ScrolledWindow.VAdjustment()
	sw.SetPropagateNaturalHeight(true)
	sw.SetPlacement(gtk.CornerBottomLeft)

	sw.vadj.Connect("notify::upper", func() {
		// If the upper value changed, then update the current value
		// accordingly.
		if sw.bottomed {
			sw.vadj.SetValue(sw.vadj.Upper())
		}
	})
	sw.vadj.Connect("value-changed", func() {
		// Manually check if we're anchored on scroll.
		sw.bottomed = sw.vadj.Value() >= (sw.vadj.Upper() - sw.vadj.PageSize())

		for box := range sw.onBottomed {
			box.f(sw.bottomed)
		}
	})

	return &sw
}

// VAdjustment overrides gtk.ScrolledWindow's.
func (w *Window) VAdjustment() *gtk.Adjustment {
	return &w.vadj
}

// IsBottomed returns true if the scrolled window is currently bottomed out.
func (w *Window) IsBottomed() bool {
	return w.bottomed
}

// ScrollToBottom scrolls the window to bottom.
func (w *Window) ScrollToBottom() {
	if w.willScroll {
		return
	}

	w.willScroll = true

	// Delegate this to when the main loop is free again, just so the dimensions
	// are properly updated.
	glib.IdleAdd(func() {
		w.vadj.SetValue(w.vadj.Upper())
		w.willScroll = false
	})
}

// OnBottomed registers the given function to be called when the user bottoms
// out the scrolled window or not.
func (w *Window) OnBottomed(f func(bottomed bool)) func() {
	if w.onBottomed == nil {
		w.onBottomed = make(map[*bottomedFunc]struct{}, 1)
	}

	box := &bottomedFunc{f}
	w.onBottomed[box] = struct{}{}

	return func() { delete(w.onBottomed, box) }
}
