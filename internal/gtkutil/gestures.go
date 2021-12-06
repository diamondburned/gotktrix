package gtkutil

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

// BindRightClick binds the given widget to take in right-click gestures. The
// function will also check for long-hold gestures.
func BindRightClick(w gtk.Widgetter, f func()) {
	BindRightClickAt(w, func(x, y float64) { f() })
}

// BindRightClickAt is a version of BindRightClick with accurate coordinates
// given to the callback.
func BindRightClickAt(w gtk.Widgetter, f func(x, y float64)) {
	c := gtk.NewGestureClick()
	c.SetButton(3)       // secondary
	c.SetExclusive(true) // handle mouse only
	c.ConnectAfter("pressed", func(nPress int, x, y float64) {
		if nPress == 1 {
			f(x, y)
		}
	})

	l := gtk.NewGestureLongPress()
	l.SetTouchOnly(true)
	l.ConnectAfter("pressed", func(x, y float64) {
		f(x, y)
	})

	widget := gtk.BaseWidget(w)
	widget.AddController(c)
	widget.AddController(l)
}
