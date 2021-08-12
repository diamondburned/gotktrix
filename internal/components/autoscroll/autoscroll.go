package autoscroll

import (
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Window describes an automatically scrolled window.
type Window struct {
	*gtk.ScrolledWindow
	vadj     gtk.Adjustment
	bottomed bool // :floshed:
}

func NewWindow() *Window {
	sw := Window{ScrolledWindow: gtk.NewScrolledWindow()}
	sw.vadj = *sw.ScrolledWindow.VAdjustment()
	sw.SetPropagateNaturalHeight(true)
	sw.SetPlacement(gtk.CornerBottomLeft)

	sw.vadj.Connect("notify::upper", func() {
		// We can't really trust Gtk to be competent.
		if sw.bottomed {
			sw.vadj.SetValue(sw.vadj.Upper())
		}
	})
	sw.vadj.Connect("value-changed", func() {
		// Manually check if we're anchored on scroll.
		sw.bottomed = (sw.vadj.Upper() - sw.vadj.PageSize()) <= sw.vadj.Value()
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
	glib.IdleAdd(func() {
		w.vadj.SetValue(w.vadj.Upper())
	})
}
