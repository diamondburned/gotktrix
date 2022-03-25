package gtkutil

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// BindRightClick binds the given widget to take in right-click gestures. The
// function will also check for long-hold gestures.
func BindRightClick(w gtk.Widgetter, f func()) {
	BindButton(w, gdk.BUTTON_SECONDARY, f)
}

// BindRightClickAt is a version of BindRightClick with accurate coordinates
// given to the callback.
func BindRightClickAt(w gtk.Widgetter, f func(x, y float64)) {
	BindButtonAt(w, gdk.BUTTON_SECONDARY, f)
}

// BindButton binds the given widget to take in button gestures.
func BindButton(w gtk.Widgetter, button uint, f func()) {
	BindButtonAt(w, button, func(x, y float64) { f() })
}

// BindButtonAt is a version of BindButton with accurate coordinates given to
// the callback.
func BindButtonAt(w gtk.Widgetter, button uint, f func(x, y float64)) {
	c := gtk.NewGestureClick()
	c.SetButton(button)
	c.SetExclusive(true) // handle mouse only
	c.ConnectAfter("pressed", func(nPress int, x, y float64) {
		if nPress == 1 {
			f(x, y)
		}
	})

	widget := gtk.BaseWidget(w)
	widget.AddController(c)

	if button == gdk.BUTTON_SECONDARY {
		l := gtk.NewGestureLongPress()
		l.SetTouchOnly(true)
		l.ConnectAfter("pressed", func(x, y float64) {
			f(x, y)
		})
		widget.AddController(l)
	}
}

// ForwardTyping forwards all typing events from w to dst.
func ForwardTyping(w, dst gtk.Widgetter) {
	ForwardTypingFunc(w, func() gtk.Widgetter { return dst })
}

func ForwardTypingFunc(w gtk.Widgetter, f func() gtk.Widgetter) {
	// Activator to focus on composer when typed on.
	typingHandler := gtk.NewEventControllerKey()
	// Run the handler at the last phase, after all key handlers have captured
	// the event.
	typingHandler.SetPropagationPhase(gtk.PhaseBubble)
	typingHandler.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		if gdk.KeyvalToUnicode(keyval) == 0 {
			// Don't forward these.
			return false
		}

		dstWidget := f()
		if dstWidget == nil {
			return false
		}

		dst := gtk.BaseWidget(dstWidget)
		dst.GrabFocus()
		typingHandler.Forward(dst)
		return true
	})
	gtk.BaseWidget(w).AddController(typingHandler)
}
