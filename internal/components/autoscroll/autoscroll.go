package autoscroll

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type bottomedFunc struct {
	f func(bool)
}

// Window describes an automatically scrolled window.
type Window struct {
	*gtk.ScrolledWindow
	view *gtk.Viewport
	vadj *gtk.Adjustment

	onBottomed map[*bottomedFunc]struct{}

	upperValue float64
	lockedPos  bool
	bottomed   bool // :floshed:
}

func NewWindow() *Window {
	w := Window{}
	w.view = gtk.NewViewport(nil, nil)

	w.ScrolledWindow = gtk.NewScrolledWindow()
	w.ScrolledWindow.SetChild(w.view)
	w.SetPropagateNaturalHeight(true)
	w.SetPlacement(gtk.CornerBottomLeft)

	w.vadj = w.ScrolledWindow.VAdjustment()

	w.vadj.Connect("notify::upper", func() {
		var (
			value = w.vadj.Value()
			upper = w.vadj.Upper()
		)
		if w.lockedPos {
			// Subtract the new value w/ the old value to get the new scroll
			// offset, then add that to the value.
			w.vadj.SetValue((upper - w.upperValue) + value)
		}
		w.upperValue = upper
		// If the upper value changed, then update the current value
		// accordingly.
		if w.bottomed {
			w.setBottomed(true)
		}
	})

	w.vadj.Connect("notify::value", func() {
		// Manually check if we're anchored on scroll.
		w.upperValue = w.vadj.Upper()
		w.setBottomed(w.vadj.Value() >= (w.upperValue - w.vadj.PageSize()))
		if w.bottomed {
			w.lockedPos = false
			for box := range w.onBottomed {
				box.f(w.bottomed)
			}
		}
	})

	return &w
}

// VAdjustment overrides gtk.ScrolledWindow's.
func (w *Window) VAdjustment() *gtk.Adjustment {
	return w.vadj
}

// SetScrollLocked sets whether or not the scroll is locked when new widgets are
// added. This is useful if new things will be added into the list, but the
// scroll window shouldn't move away.
func (w *Window) SetScrollLocked(locked bool) {
	w.bottomed = false
	w.lockedPos = true
}

// IsBottomed returns true if the scrolled window is currently bottomed out.
func (w *Window) IsBottomed() bool {
	return w.bottomed
}

// ScrollToBottom scrolls the window to bottom.
func (w *Window) ScrollToBottom() {
	w.setBottomed(true)
}

func (w *Window) setBottomed(bottomed bool) {
	w.bottomed = bottomed
	if bottomed {
		w.bottom()
		w.AddTickCallback(func(gtk.Widgetter, gdk.FrameClocker) bool {
			w.bottom()
			// True if we're not at the bottom, i.e. the value doesn't match up.
			return w.vadj.Value() != (w.vadj.Upper() - w.vadj.PageSize())
		})
	}
}

func (w *Window) bottom() {
	w.vadj.SetValue(w.upperValue)
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

// SetChild sets the child of the ScrolledWindow.
func (w *Window) SetChild(child gtk.Widgetter) {
	w.view.SetChild(child)
}
