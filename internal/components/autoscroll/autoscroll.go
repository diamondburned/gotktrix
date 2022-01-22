package autoscroll

import (
	"math"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type scrollState uint8

const (
	_ scrollState = iota
	bottomed
	locked
)

func (s scrollState) is(this scrollState) bool { return s == this }

// Window describes an automatically scrolled window.
type Window struct {
	*gtk.ScrolledWindow
	view *gtk.Viewport
	vadj *gtk.Adjustment

	onBottomed func()

	upperValue   float64
	targetScroll float64
	state        scrollState
}

func NewWindow() *Window {
	w := Window{
		upperValue:   math.NaN(),
		targetScroll: math.NaN(),
	}

	w.ScrolledWindow = gtk.NewScrolledWindow()
	w.ScrolledWindow.SetPropagateNaturalHeight(true)
	w.ScrolledWindow.SetPlacement(gtk.CornerBottomLeft)

	w.vadj = w.ScrolledWindow.VAdjustment()

	w.view = gtk.NewViewport(nil, w.vadj)
	w.view.SetVScrollPolicy(gtk.ScrollNatural)
	w.ScrolledWindow.SetChild(w.view)

	w.bindFrameClock()

	w.vadj.ConnectAfter("notify::upper", func() {
		lastUpper := w.upperValue
		w.upperValue = w.vadj.Upper()

		switch w.state {
		case locked:
			// Subtract the new value w/ the old value to get the new scroll
			// offset, then add that to the value.
			w.targetScroll = w.upperValue - lastUpper + w.vadj.Value()
		case bottomed:
			w.targetScroll = w.upperValue
		}
	})

	w.vadj.ConnectAfter("notify::value", func() {
		// Skip if we're locked, since we're only updating this if the state is
		// either bottomed or not.
		if w.state.is(locked) {
			return
		}

		w.upperValue = w.vadj.Upper()

		// Bottom check.
		if w.vadj.Value() >= (w.upperValue - w.vadj.PageSize()) {
			w.state = bottomed
			w.targetScroll = w.upperValue
			w.emitBottomed()
		} else {
			w.state = 0
		}
	})

	return &w
}

// Viewport returns the ScrolledWindow's Viewport.
func (w *Window) Viewport() *gtk.Viewport {
	return w.view
}

func (w *Window) bindFrameClock() {
	var clock *gdk.FrameClock
	var signal glib.SignalHandle

	w.ConnectMap(func() {
		clock = gdk.BaseFrameClock(w.FrameClock())
		// Layout gets called after the Adjustment's size-allocate, so we can
		// set the scroll here. We can't in the notify callbacks, because
		// that'll mess up the function.
		signal = clock.ConnectLayout(func() {
			// If the upper value changed, then update the current value
			// accordingly.
			if !math.IsNaN(w.targetScroll) {
				w.vadj.SetValue(w.targetScroll)
				w.targetScroll = math.NaN()
			}
		})
	})

	w.ConnectUnmap(func() {
		if clock != nil {
			clock.HandlerDisconnect(signal)
			clock = nil
		}
	})
}

// VAdjustment overrides gtk.ScrolledWindow's.
func (w *Window) VAdjustment() *gtk.Adjustment {
	return w.vadj
}

// SetScrollLocked sets whether or not the scroll is locked when new widgets are
// added. This is useful if new things will be added into the list, but the
// scroll window shouldn't move away.
func (w *Window) SetScrollLocked(scrollLocked bool) {
	w.state = locked
}

// Unbottom clears the bottomed state.
func (w *Window) Unbottom() {
	if w.state.is(bottomed) {
		w.state = 0
	}
}

// IsBottomed returns true if the scrolled window is currently bottomed out.
func (w *Window) IsBottomed() bool {
	return w.state.is(bottomed)
}

// ScrollToBottom scrolls the window to bottom.
func (w *Window) ScrollToBottom() {
	w.state = bottomed
	w.targetScroll = w.upperValue
}

// OnBottomed registers the given function to be called when the user bottoms
// out the scrolled window.
func (w *Window) OnBottomed(f func()) {
	if w.onBottomed == nil {
		w.onBottomed = f
		return
	}

	old := w.onBottomed
	w.onBottomed = func() {
		old()
		f()
	}
}

func (w *Window) emitBottomed() {
	if w.onBottomed != nil {
		w.onBottomed()
	}
}

// SetChild sets the child of the ScrolledWindow.
func (w *Window) SetChild(child gtk.Widgetter) {
	_, scrollable := child.(gtk.Scrollabler)
	if scrollable {
		w.ScrolledWindow.SetChild(child)
	} else {
		w.view.SetChild(child)
		w.ScrolledWindow.SetChild(w.view)
	}
}
