package autoscroll

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// Window describes an automatically scrolled window.
type Window struct {
	*gtk.ScrolledWindow
	vadj     *gtk.Adjustment
	bottomed *bool // :floshed:
}

func NewWindow() *Window {
	sw := Window{ScrolledWindow: gtk.NewScrolledWindow()}
	sw.SetPropagateNaturalHeight(true)
	sw.SetPlacement(gtk.CornerBottomLeft)

	bottomed := new(bool)
	sw.bottomed = bottomed
	sw.vadj = sw.ScrolledWindow.VAdjustment()

	sw.vadj.Connect("notify::upper", func(vadj *gtk.Adjustment) {
		// We can't really trust Gtk to be competent.
		if *bottomed {
			vadj.SetValue(vadj.Upper())
		}
	})
	sw.vadj.Connect("value-changed", func(vadj *gtk.Adjustment) {
		// Manually check if we're anchored on scroll.
		*bottomed = (vadj.Upper() - vadj.PageSize()) <= vadj.Value()
	})

	return &sw
}

// VAdjustment overrides gtk.ScrolledWindow's.
func (w *Window) VAdjustment() *gtk.Adjustment {
	return w.vadj
}

// ScrollToBottom scrolls the window to bottom.
func (w *Window) ScrollToBottom() {
	w.vadj.SetValue(w.vadj.Upper())
}
